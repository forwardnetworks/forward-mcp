# Forward MCP

**Version 2.2.0**

Forward MCP is an open-source server that provides a set of tools and APIs for interacting with Forward Networks' platform. It enables automation, analysis, and integration with network data using the MCP protocol.

## Features
- Exposes Forward Networks tools via the MCP protocol (55+ tools)
- Supports prompt workflows and contextual resources
- **NEW**: Instance lock protection prevents multiple server instances
- **NEW**: Additional API functions for snapshot, location, and device management
- **NEW**: Bloomsearch integration for efficient handling of large NQE results
- **NEW**: Automatic bloom filter generation for fast data filtering
- **NEW**: Persistent bloom indexes for network analysis
- Designed for easy integration and automation

## High-Level Architecture
- **cmd/server/main.go**: Entry point for the server. Initializes configuration, logging, and registers tools, prompts, and resources.
- **internal/service**: Implements the core Forward MCP service logic.
- **internal/service/bloom_search.go**: Bloomsearch integration for large result handling.
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

### Set up .env


Copy env.example to a new file called .env.  In this new file, set the following environment variables before running the MCP:
- `FORWARD_API_BASE_URL` – Base URL for the Forward Networks API
- `FORWARD_API_KEY` – Your Forward Networks API key
- `FORWARD_API_SECRET` - Your Forward Networks API Secret
- `FORWARD_DEFAULT_NETWORK_ID` – (Optional) Default network ID
- `FORWARD_INSECURE_SKIP_VERIFY` – (Optional, default: false) Set to true to skip TLS verification

### Bloomsearch Configuration (Optional)
- `FORWARD_BLOOM_ENABLED` – (Optional, default: true) Enable bloomsearch for large results
- `FORWARD_BLOOM_THRESHOLD` – (Optional, default: 100) Minimum result size to trigger bloom filter creation
- `FORWARD_BLOOM_INDEX_PATH` – (Optional, default: data/bloom_indexes) Path for bloom index storage

### Instance Lock Configuration (Optional)
- `FORWARD_LOCK_DIR` – (Optional, default: /tmp) Directory for server instance lock file

Run the server:
```cd bin
./forward-mcp-server
```

The server will start and listen for MCP protocol messages via stdio (compatible with Claude Desktop and other MCP clients).

## New Bloomsearch Capabilities

### Automatic Bloom Filter Generation
- Automatically creates bloom filters for NQE results with >100 items
- Enables fast prefiltering to reduce memory usage and improve search performance
- Persistent storage of bloom indexes for reuse across sessions

### Enhanced Search Performance
- Bloom filters provide O(1) lookup time for membership queries
- Reduces memory footprint by only loading relevant data blocks
- Supports complex search patterns with multiple terms

### Integration with Existing Systems
- Works seamlessly with the semantic cache and memory system
- Automatically uses bloom filters when available for search operations
- Maintains backward compatibility with existing workflows

## Documentation
- See the `docs/` folder for troubleshooting, architecture, and advanced guides.
- **NEW**: Instance lock protection guide - `docs/INSTANCE_LOCK_GUIDE.md`
- **NEW**: New API functions guide - `docs/NEW_API_FUNCTIONS.md`
- **NEW**: Bloomsearch integration guide and performance optimization tips.

## Contributing
Contributions are welcome! Please open issues or pull requests for bug fixes, features, or documentation improvements. 

## AI Attribution

Portions of this project were generated or assisted by AI tools, including OpenAI GPT-4, Cursor, and Claude. All AI-generated content was reviewed and, where necessary, modified by human contributors.
