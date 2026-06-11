# Forward MCP

**Version 3.0.0**

Forward MCP is an open-source server that provides a set of tools and APIs for interacting with Forward Networks' platform. It enables automation, analysis, and integration with network data using the Model Context Protocol (MCP).

## Features
- Exposes 54 Forward Networks tools via the MCP protocol, plus 6 workflow prompts and a network-context resource
- Built on the official [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) (protocol revision 2025-06-18, negotiated with older clients)
- Tool behavior annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`) so clients can distinguish queries from destructive operations
- Schema-enforced input validation with LLM-friendly error messages
- Supports prompt workflows and contextual resources
- Instance lock protection prevents multiple server instances
- Bloomsearch integration for efficient handling of large NQE results, with automatic bloom filter generation and persistent indexes
- Semantic cache and knowledge-graph memory system for query results
- Designed for easy integration and automation

## What's New in 3.0.0

### Migrated to the official MCP Go SDK
The server now uses `github.com/modelcontextprotocol/go-sdk` (v1.6.1), replacing the unmaintained `metoro-io/mcp-golang` library:

- **Protocol currency**: initialize handshake reports protocol revision 2025-06-18 and negotiates with clients on older revisions (previously pinned to 2024-11-05).
- **Populated `serverInfo` and `instructions`**: clients now see the server name, version, and usage guidance during initialization.
- **Tool annotations**: every tool declares its behavior — read-only (e.g. `list_networks`), additive (e.g. `create_entity`), or destructive (e.g. `delete_snapshot`) — enabling better client confirmation UX.
- **Enforced input validation**: tool arguments are validated against their JSON schemas before handlers run; invalid input returns a tool execution error the model can self-correct from. Tools that support the `set_default_network` fallback accept an omitted `network_id`.
- **Cursor pagination** for tool/prompt/resource lists, per the MCP spec.

### Forward API client validated against the official OpenAPI spec
The client in `internal/forward` was audited against `https://docs.fwd.app/latest/api/spec/complete.json` and brought into conformance:

- `Network`/`Snapshot` timestamps decode as RFC3339 strings (`createdAt`, `processedAt`); network descriptions map to the API's `note` field
- Device pagination uses the correct `skip` parameter (previously `offset`, which the API silently ignored)
- NQE query options use the spec's single `sortBy` object and `itemFormat` field; diff results read `totalNumRows`
- Single path search decodes the spec's response shape (previously returned empty results)
- Added spec fields: `Device.displayName`/`sourceName`/`collectionError`/`processingError`/`tags`, `NqeRunResult.totalNumItems`, path search `srcIpLocationType`/`unrecognizedValues`

## High-Level Architecture
- **cmd/server/main.go**: Entry point for the server. Initializes configuration, logging, and registers tools, prompts, and resources.
- **internal/service**: Implements the core Forward MCP service logic.
- **internal/service/mcp_adapter.go**: Adapts service handlers to the official MCP Go SDK and defines per-tool behavior annotations.
- **internal/service/bloom_search.go**: Bloomsearch integration for large result handling.
- **internal/forward**: Forward Networks API client (validated against the official OpenAPI spec).
- **internal/config**: Handles configuration loading (API URL, credentials, etc).
- **internal/logger**: Provides logging utilities (stderr/file only — stdout is reserved for MCP protocol messages).

## Prerequisites
- Go 1.25 or later
- CGO enabled (required for SQLite)
- Access to Forward Networks API (API URL and API Key)

## Build Instructions
```sh
git clone https://github.com/gaberger/forward-mcp.git
cd forward-mcp
CGO_ENABLED=1 go build -o forward-mcp ./cmd/server
# or: make build
```

## Run Instructions
Set the following environment variables before running:
- `FORWARD_API_BASE_URL` – Base URL for the Forward Networks API
- `FORWARD_API_KEY` – Your Forward Networks API key
- `FORWARD_API_SECRET` - Your Forward Networks API Secret
- `FORWARD_DEFAULT_NETWORK_ID` – (Optional) Default network ID

Note: TLS 1.3 is enforced for all API connections; certificate verification cannot be disabled.

### Bloomsearch Configuration (Optional)
- `FORWARD_BLOOM_ENABLED` – (Optional, default: true) Enable bloomsearch for large results
- `FORWARD_BLOOM_THRESHOLD` – (Optional, default: 100) Minimum result size to trigger bloom filter creation
- `FORWARD_BLOOM_INDEX_PATH` – (Optional, default: data/bloom_indexes) Path for bloom index storage

### Instance Lock Configuration (Optional)
- `FORWARD_LOCK_DIR` – (Optional, default: /tmp) Directory for server instance lock file

Run the server:
```sh
./forward-mcp
```

The server will start and listen for MCP protocol messages via stdio (compatible with Claude Desktop, Claude Code, and other MCP clients).

## Bloomsearch Capabilities

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

## Testing
```sh
make test-quick        # unit tests
go test -race ./internal/...   # race detector (required before commits)
make test-integration  # live API tests (requires .env with credentials)
```

## Documentation
- See the `docs/` folder for troubleshooting, architecture, and advanced guides.
- Instance lock protection guide - `docs/INSTANCE_LOCK_GUIDE.md`
- New API functions guide - `docs/NEW_API_FUNCTIONS.md`
- Bloomsearch integration guide and performance optimization tips.

## Contributing
Contributions are welcome! Please open issues or pull requests for bug fixes, features, or documentation improvements. 

## AI Attribution

Portions of this project were generated or assisted by AI tools, including OpenAI GPT-4, Cursor, and Claude. All AI-generated content was reviewed and, where necessary, modified by human contributors.
