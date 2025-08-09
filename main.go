package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type HandoffArgs struct {
	Prompt string `json:"prompt"`
}

const (
	MAX_DEEPLINK_LENGTH = 1800
)

var (
	httpMode = false
	httpPort = 8080
)

func main() {
	parseFlags()

	srv := buildServer()
	ctx := context.Background()

	if httpMode {
		startHTTPServer(srv)
		return
	}

	// stdio mode
	transport := mcp.NewStdioTransport()
	if err := srv.Run(ctx, transport); err != nil {
		log.Fatal(err)
	}
}

func parseFlags() {
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "--http":
			httpMode = true
		case arg == "--port" && i+1 < len(os.Args):
			if p, err := strconv.Atoi(os.Args[i+1]); err == nil {
				httpPort = p
			}
			i++
		}
	}
}

func buildServer() *mcp.Server {
	impl := &mcp.Implementation{
		Name:    "chatgpt-handoff",
		Version: "0.1.0",
	}

	srv := mcp.NewServer(impl, nil)

	tool := &mcp.Tool{
		Name:        "handoff_to_chatgpt",
		Description: "Hand off a research or debugging prompt to ChatGPT, powered by the very powerful GPT-5 thinking model with advanced tools like browsing. Write detailed, specific prompts that include all necessary context. After sending your prompt, you should stop and wait for the user to relay ChatGPT's response back to you.\n\nExample uses:\n1. Research: \"Research the latest developments in WebAssembly performance optimizations, focusing on 2024-2025 improvements and real-world benchmarks\"\n2. Debugging: \"Debug this Go memory leak issue: [include relevant code snippets, error messages, and context about when the issue occurs]\"",
	}

	mcp.AddTool(srv, tool, handleHandoff)

	return srv
}

func handleHandoff(ctx context.Context, ss *mcp.ServerSession, params *mcp.CallToolParamsFor[HandoffArgs]) (*mcp.CallToolResultFor[any], error) {
	prompt := strings.TrimSpace(params.Arguments.Prompt)
	if prompt == "" {
		return &mcp.CallToolResultFor[any]{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "prompt is required"},
			},
		}, nil
	}

	// Always copy to clipboard as reliable fallback
	if err := copyToClipboard(prompt); err != nil {
		return &mcp.CallToolResultFor[any]{
			IsError: true,
			Content: []mcp.Content{
				&mcp.TextContent{Text: "failed to copy prompt to clipboard: " + err.Error()},
			},
		}, nil
	}

	// Additionally, try deeplink if prompt is short enough
	deeplink := buildChatGPTDeeplink(prompt)
	if len(deeplink) <= MAX_DEEPLINK_LENGTH {
		_ = openURL(deeplink) // Best effort, ignore errors
	}

	return &mcp.CallToolResultFor[any]{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Request sent. Now you should stop and wait for the user to share ChatGPT's response."},
		},
	}, nil
}

func copyToClipboard(s string) error {
	switch runtime.GOOS {
	case "darwin":
		cmd := exec.Command("pbcopy")
		in, _ := cmd.StdinPipe()
		if err := cmd.Start(); err != nil {
			return err
		}
		_, _ = io.WriteString(in, s)
		_ = in.Close()
		return cmd.Wait()
	case "windows":
		cmd := exec.Command("powershell", "-NoProfile", "-Command", "Set-Clipboard -Value @'\n"+s+"\n'@")
		return cmd.Run()
	default:
		// Linux: try xclip first, then xsel
		if err := exec.Command("bash", "-c", "command -v xclip >/dev/null").Run(); err == nil {
			c := exec.Command("xclip", "-selection", "clipboard")
			in, _ := c.StdinPipe()
			_ = c.Start()
			_, _ = io.WriteString(in, s)
			_ = in.Close()
			return c.Wait()
		}
		if err := exec.Command("bash", "-c", "command -v xsel >/dev/null").Run(); err == nil {
			c := exec.Command("xsel", "--clipboard", "--input")
			in, _ := c.StdinPipe()
			_ = c.Start()
			_, _ = io.WriteString(in, s)
			_ = in.Close()
			return c.Wait()
		}
		return errors.New("no clipboard utility found (install xclip or xsel)")
	}
}

func buildChatGPTDeeplink(prompt string) string {
	encoded := url.QueryEscape(prompt)
	return "https://chatgpt.com/?q=" + encoded
}

func openURL(urlStr string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", urlStr).Run()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", urlStr).Run()
	default:
		// Linux - try common browsers
		browsers := []string{"xdg-open", "sensible-browser", "x-www-browser", "firefox", "chromium", "google-chrome"}
		for _, browser := range browsers {
			if err := exec.Command("which", browser).Run(); err == nil {
				return exec.Command(browser, urlStr).Run()
			}
		}
		return errors.New("no suitable browser found")
	}
}

func startHTTPServer(srv *mcp.Server) {
	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server { return srv })

	mux := http.NewServeMux()
	mux.Handle("/mcp/", handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := ":" + strconv.Itoa(httpPort)
	log.Printf("Starting MCP server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
