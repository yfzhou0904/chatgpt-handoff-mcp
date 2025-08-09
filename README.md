# ChatGPT Handoff MCP Server

A minimal MCP (Model Context Protocol) server that enables seamless handoff of prompts from Claude Code to ChatGPT. This tool simply copies your prompt to the clipboard so you can paste it directly into ChatGPT.

## Features

- **Single MCP tool**: `request_chatgpt_research` with flexible prompt input
- **Dual transports**: stdio (default) and HTTP server modes
- **2025-06-18 MCP spec**: Latest protocol version with structured outputs
- **Cross-platform clipboard**: Works on macOS, Windows, and Linux
- **Zero dependencies**: Single Go binary with no external requirements
- **Minimal and fast**: No file I/O, no notifications, just clipboard copying

## Installation

### From Source

```bash
git clone https://github.com/yourorg/chatgpt-handoff
cd chatgpt-handoff
go build -o chatgpt-handoff
```

### Install Binary

Place the `chatgpt-handoff` binary somewhere in your `$PATH`, such as `/usr/local/bin/`.

### Platform Dependencies

**Linux users**: Install a clipboard utility:
```bash
# Ubuntu/Debian
sudo apt install xclip

# Or alternatively
sudo apt install xsel
```

**Windows/macOS**: No additional dependencies required.

## Configuration

### Claude Code Integration

Add to your Claude Code MCP configuration (typically `~/.config/claude-code/settings.json`):

```json
{
  "mcpServers": {
    "chatgpt-handoff": {
      "command": "/usr/local/bin/chatgpt-handoff"
    }
  }
}
```

### Command Line Options

- `--http`: Enable HTTP server mode instead of stdio
- `--port <number>`: HTTP server port (default: 8080, only with --http)

### Example configurations:

**Stdio mode (default)**:
```json
{
  "mcpServers": {
    "chatgpt-handoff": {
      "command": "/usr/local/bin/chatgpt-handoff"
    }
  }
}
```

**HTTP server mode**:
```bash
# Start server
./chatgpt-handoff --http --port 8080

# Configure Claude Code to connect to HTTP server
{
  "mcpServers": {
    "chatgpt-handoff": {
      "transport": {
        "type": "http",
        "baseUrl": "http://localhost:8080/mcp"
      }
    }
  }
}
```

## Usage

Once configured, ask Claude Code to research topics:

**Example prompts:**
- "Do deep web research on quantum computing trends in 2024"
- "Research competitive analysis of AI coding assistants"
- "Investigate the latest developments in renewable energy storage"

Claude will automatically call the `request_chatgpt_research` tool, which simply copies your prompt to the clipboard.

## Tool Schema

The MCP tool accepts a simple prompt parameter:

```json
{
  "prompt": "string (required) - The research prompt to send to ChatGPT"
}
```

### Structured Output

The tool returns structured data:

```json
{
  "clipboardStatus": "copied|failed",
  "timestamp": "2025-08-09T01:40:00Z"
}
```

## How It Works

1. You provide a prompt to Claude Code
2. Claude Code calls the `request_chatgpt_research` tool
3. Your prompt is copied directly to the clipboard
4. You paste it into ChatGPT manually

That's it! Simple and reliable.


## Manual Testing

You can test the MCP server directly with JSON-RPC:

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"test","version":"1.0"}}}' | ./chatgpt-handoff
echo '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | ./chatgpt-handoff
echo '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"request_chatgpt_research","arguments":{"prompt":"Research the latest AI trends in 2025, focusing on practical applications and market impact."}}}' | ./chatgpt-handoff
```

## Troubleshooting

### Linux Clipboard Issues
If clipboard copying fails, ensure you have `xclip` or `xsel` installed:
```bash
which xclip || which xsel
```

## License

MIT License - see LICENSE file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Future Enhancements

- Multiple clipboard formats (plain text, markdown, etc.)
- Optional prompt preprocessing/formatting
- Support for additional clipboard utilities