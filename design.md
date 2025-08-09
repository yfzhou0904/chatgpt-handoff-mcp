# MCP Server: `chatgpt-handoff-mcp`

A simple MCP server that hands off prompts to ChatGPT by copying them to the clipboard and optionally opening ChatGPT via deeplink.

## Overview

This server provides a single MCP tool `handoff_to_chatgpt` that:
1. Copies the prompt to the system clipboard
2. Opens ChatGPT in browser with the prompt (if short enough for URL)
3. Returns a success message to Claude

## Tool Interface

**Tool name:** `handoff_to_chatgpt`

**Input schema:**
```json
{
  "type": "object",
  "properties": {
    "prompt": {
      "type": "string",
      "minLength": 1,
      "description": "The prompt to send to ChatGPT"
    }
  },
  "required": ["prompt"],
  "additionalProperties": false
}
```

**Example usage:**
```json
{
  "name": "handoff_to_chatgpt",
  "arguments": {
    "prompt": "Research the latest developments in WebAssembly performance optimizations, focusing on 2024-2025 improvements and real-world benchmarks"
  }
}
```

## Configuration

**Claude Code registration:**
```json
{
  "mcpServers": {
    "chatgpt-handoff": {
      "command": "/usr/local/bin/chatgpt-handoff"
    }
  }
}
```

**Optional flags:**
- `--http`: Run as HTTP server instead of stdio
- `--port N`: Set HTTP port (default 8080)

## Implementation

- **Protocol**: Minimal MCP JSON-RPC over stdio or HTTP
- **Clipboard**: Uses `pbcopy` (macOS), `powershell Set-Clipboard` (Windows), or `xclip`/`xsel` (Linux)
- **Browser**: Opens `https://chatgpt.com/?q=<encoded-prompt>` for prompts under 1800 chars
- **Dependencies**: None - uses system clipboard utilities

## Building

```bash
go build -o chatgpt-handoff main.go
```

Binary can be placed anywhere in PATH or referenced directly in Claude config.