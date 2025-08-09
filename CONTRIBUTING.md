# Contributing Guide

This file provides guidance when working with code in this repository.

## Project Overview

This is a minimal MCP (Model Context Protocol) server that enables seamless handoff of prompts to ChatGPT. The server provides a single tool `handoff_to_chatgpt` that copies prompts to the system clipboard and optionally opens ChatGPT via browser deeplink for short prompts.

Built using the official [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) v0.2.0.

## Build Commands

```bash
# Install dependencies
go mod tidy

# Build the binary
go build -o chatgpt-handoff main.go

# Build for specific platforms
GOOS=linux go build -o chatgpt-handoff-linux main.go
GOOS=windows go build -o chatgpt-handoff.exe main.go
```

## Testing

### Stdio Transport (Default)

Create a test file with MCP protocol messages:

```bash
# Create test file
cat > test.jsonl << 'EOF'
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"handoff_to_chatgpt","arguments":{"prompt":"Test research prompt"}}}
EOF

# Run tests
cat test.jsonl | ./chatgpt-handoff
```

### HTTP Transport

```bash
# Start HTTP server
./chatgpt-handoff --http --port 3000

# Test health endpoint
curl http://localhost:3000/health

# The /mcp/ endpoint provides Server-Sent Events (SSE) for MCP protocol communication
curl http://localhost:3000/mcp/
```

## Architecture

The codebase is a single Go file (`main.go`) implementing:

- **MCP SDK Integration**: Uses official Go SDK for protocol handling
- **Dual Transport**: Supports both stdio (default) and HTTP/SSE server modes via SDK transports
- **Cross-platform Clipboard**: Uses platform-specific commands (`pbcopy`, `Set-Clipboard`, `xclip`/`xsel`)
- **Browser Integration**: Opens ChatGPT deeplinks for prompts under 1800 characters

Key functions:
- `buildServer()`: Creates MCP server with tool registration
- `handleHandoff()`: Core business logic for prompt handoff
- `copyToClipboard()`: Cross-platform clipboard operations
- `buildChatGPTDeeplink()`: URL encoding for ChatGPT integration
- `startHTTPServer()`: HTTP/SSE transport mode using SDK

## Configuration

The server supports command-line flags:
- `--http`: Enable HTTP server mode instead of stdio
- `--port N`: Set HTTP server port (default: 8080)

For MCP client integration, add to your configuration:
```json
{
  "mcpServers": {
    "chatgpt-handoff": {
      "command": "/path/to/chatgpt-handoff"
    }
  }
}
```

## Dependencies

### Go Dependencies
- Go 1.23+ (automatically managed by Go toolchain)
- [modelcontextprotocol/go-sdk](https://github.com/modelcontextprotocol/go-sdk) v0.2.0

### Platform Dependencies
- **Linux**: Requires `xclip` or `xsel` for clipboard operations
- **macOS/Windows**: No additional dependencies required

## Tool Interface

The server provides one MCP tool:
- **Name**: `handoff_to_chatgpt`
- **Purpose**: Copy research/debugging prompts to clipboard for manual pasting into ChatGPT
- **Input**: `prompt` (string, required)
- **Behavior**: Always copies to clipboard, opens browser deeplink if prompt is short enough