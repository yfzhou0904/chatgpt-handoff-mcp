package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

// JSON-RPC types
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      any         `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RespError  `json:"error,omitempty"`
}

type RespError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP protocol types
type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	ClientInfo      struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ServerInfo      ServerInfo     `json:"serverInfo"`
	Capabilities    map[string]any `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolCallResult struct {
	Content []ContentItem `json:"content"`
}

type ContentItem struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// Business input
type RequestInput struct {
	Prompt string `json:"prompt"`
}

// Structured output
type HandoffResult struct {
	ClipboardStatus string `json:"clipboardStatus"`
	Timestamp       string `json:"timestamp"`
}

const (
	TOOLNAME_HANDOFF_TO_CHATGPT = "handoff_to_chatgpt"
	MAX_DEEPLINK_LENGTH         = 1800
)

// getToolDefinition returns the shared tool definition
func getToolDefinition() Tool {
	return Tool{
		Name:        TOOLNAME_HANDOFF_TO_CHATGPT,
		Description: "Hand off a research or debugging prompt to ChatGPT. Write detailed, specific prompts that include all necessary context. After calling this tool, you should stop and wait for the user to relay ChatGPT's response back to you.\n\nExample uses:\n1. Research: \"Research the latest developments in WebAssembly performance optimizations, focusing on 2024-2025 improvements and real-world benchmarks\"\n2. Debugging: \"Debug this Go memory leak issue: [include relevant code snippets, error messages, and context about when the issue occurs]\"",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"minLength":   1,
					"description": "The prompt to send to ChatGPT",
				},
			},
			"required":             []string{"prompt"},
			"additionalProperties": false,
		},
	}
}

var (
	httpMode = false
	httpPort = 8080
)

func main() {
	parseFlags()

	if httpMode {
		startHTTPServer()
		return
	}

	// Original stdio mode
	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			return
		}
		if err != nil {
			logErr("read: %v", err)
			return
		}
		line = bytesTrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeResp(nil, nil, &RespError{Code: -32700, Message: "Parse error"})
			continue
		}
		handleRequest(req)
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

func handleRequest(req Request) {
	switch req.Method {
	case "initialize":
		var p InitializeParams
		_ = json.Unmarshal(req.Params, &p)
		res := InitializeResult{
			ProtocolVersion: "2025-06-18",
			Capabilities:    map[string]any{"tools": map[string]any{}},
		}
		res.ServerInfo.Name = "chatgpt-handoff"
		res.ServerInfo.Version = "0.1.0"
		writeResp(req.ID, res, nil)

	case "tools/list":
		res := ToolsListResult{
			Tools: []Tool{getToolDefinition()},
		}
		writeResp(req.ID, res, nil)

	case "tools/call":
		var p ToolCallParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeResp(req.ID, nil, &RespError{Code: -32602, Message: "Invalid params"})
			return
		}
		switch p.Name {
		case TOOLNAME_HANDOFF_TO_CHATGPT:
			res, err := handleHandoff(p.Arguments)
			if err != nil {
				writeResp(req.ID, nil, &RespError{Code: 1, Message: err.Error()})
				return
			}
			writeResp(req.ID, res, nil)
		default:
			writeResp(req.ID, nil, &RespError{Code: -32601, Message: "Method not found"})
		}

	case "shutdown":
		writeResp(req.ID, map[string]any{}, nil)
		os.Exit(0)

	default:
		writeResp(req.ID, nil, &RespError{Code: -32601, Message: "Method not found"})
	}
}

func handleHandoff(raw json.RawMessage) (ToolCallResult, error) {
	var in RequestInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return ToolCallResult{}, fmt.Errorf("bad input: %w", err)
	}
	if strings.TrimSpace(in.Prompt) == "" {
		return ToolCallResult{}, errors.New("prompt is required")
	}

	// Always copy to clipboard as reliable fallback
	if err := copyToClipboard(in.Prompt); err != nil {
		return ToolCallResult{}, fmt.Errorf("failed to copy prompt to clipboard: %w", err)
	}

	// Additionally, try deeplink if prompt is short enough
	deeplink := buildChatGPTDeeplink(in.Prompt)
	if len(deeplink) <= MAX_DEEPLINK_LENGTH {
		_ = openURL(deeplink) // Best effort, ignore errors
	}

	return ToolCallResult{
		Content: []ContentItem{
			{Type: "text", Text: "Request sent. Now wait for the user to share ChatGPT's response."},
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

func writeResp(id any, result interface{}, err *RespError) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   err,
	}
	data, _ := json.Marshal(resp)
	os.Stdout.Write(data)
	os.Stdout.Write([]byte("\n"))
}

func logErr(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
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

// HTTP transport implementation
func startHTTPServer() {
	http.HandleFunc("/mcp", handleHTTPRequest)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", httpPort)
	log.Printf("Starting MCP server on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for development
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPResp(w, nil, nil, &RespError{Code: -32700, Message: "Parse error"})
		return
	}

	handleRequestHTTP(w, req)
}

func handleRequestHTTP(w http.ResponseWriter, req Request) {
	switch req.Method {
	case "initialize":
		var p InitializeParams
		_ = json.Unmarshal(req.Params, &p)
		res := InitializeResult{
			ProtocolVersion: "2025-06-18",
			Capabilities:    map[string]any{"tools": map[string]any{}},
		}
		res.ServerInfo.Name = "chatgpt-handoff"
		res.ServerInfo.Version = "0.1.0"
		writeHTTPResp(w, req.ID, res, nil)

	case "tools/list":
		res := ToolsListResult{
			Tools: []Tool{getToolDefinition()},
		}
		writeHTTPResp(w, req.ID, res, nil)

	case "tools/call":
		var p ToolCallParams
		if err := json.Unmarshal(req.Params, &p); err != nil {
			writeHTTPResp(w, req.ID, nil, &RespError{Code: -32602, Message: "Invalid params"})
			return
		}
		switch p.Name {
		case TOOLNAME_HANDOFF_TO_CHATGPT:
			res, err := handleHandoff(p.Arguments)
			if err != nil {
				writeHTTPResp(w, req.ID, nil, &RespError{Code: 1, Message: err.Error()})
				return
			}
			writeHTTPResp(w, req.ID, res, nil)
		default:
			writeHTTPResp(w, req.ID, nil, &RespError{Code: -32601, Message: "Method not found"})
		}

	case "shutdown":
		writeHTTPResp(w, req.ID, map[string]any{}, nil)
		// Note: In HTTP mode, shutdown doesn't actually stop the server
		// This would require more sophisticated lifecycle management

	default:
		writeHTTPResp(w, req.ID, nil, &RespError{Code: -32601, Message: "Method not found"})
	}
}

func writeHTTPResp(w http.ResponseWriter, id any, result interface{}, err *RespError) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
		Error:   err,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}
