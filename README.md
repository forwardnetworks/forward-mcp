# Forward MCP

**Version 2.0.0**

Forward MCP is an open-source server that provides a set of tools and APIs for interacting with Forward Networks' platform. It enables automation, analysis, and integration with network data using the MCP protocol.

## Features
- Exposes Forward Networks tools via the MCP protocol
- Supports prompt workflows and contextual resources
- Designed for easy integration and automation

## High-Level Architecture
- **cmd/server/main.go**: Entry point for the server. Initializes configuration, logging, and registers tools, prompts, and resources.
- **internal/service**: Implements the core Forward MCP service logic.
- **internal/config**: Handles configuration loading (API URL, credentials, etc).
- **internal/logger**: Provides logging utilities.

## Prerequisites
- Go 1.20 or later
- Access to Forward Networks API (API URL and API Key)

## Build Instructions
```sh
git clone https://github.com/forward-mcp/forward-mcp.git
cd forward-mcp
go build -o forward-mcp ./cmd/server
```

## Run Instructions
Set the following environment variables before running:
- `FORWARD_API_BASE_URL` – Base URL for the Forward Networks API
- `FORWARD_API_KEY` – Your Forward Networks API key
- `FORWARD_API_SECRET` - Your Forward Networs API Secret
- `FORWARD_DEFAULT_NETWORK_ID` – (Optional) Default network ID
- `FORWARD_INSECURE_SKIP_VERIFY` – (Optional, default: false) Set to true to skip TLS verification

Run the server:
```sh
./forward-mcp
```

The server will start and listen for MCP protocol messages via stdio (compatible with Claude Desktop and other MCP clients).

## Documentation
- See the `docs/` folder for troubleshooting, architecture, and advanced guides.

## Contributing
Contributions are welcome! Please open issues or pull requests for bug fixes, features, or documentation improvements. 


## AI Attribution

Portions of this project were generated or assisted by AI tools, including OpenAI GPT-4, Cursor, and Claude. All AI-generated content was reviewed and, where necessary, modified by human contributors.
