---
name: hexa-analyze-deps
description: Analyze dependencies and recommend optimal tech stack for a hexagonal project. Use when the user asks to "analyze dependencies", "what libraries should I use", "recommend tech stack", "language selection", or "how should components communicate".
---

# Hex Analyze Deps — Dependency Analysis and Tech Stack Recommendation

Analyzes a problem statement to recommend optimal language/library combinations and cross-language communication patterns for hexagonal architecture projects.

## Parameters

Ask the user for:
- **problem** (required): Problem statement or feature description
- **languages** (optional, default: "typescript,go,rust"): Comma-separated target languages to consider

## Execution Steps

### 1. Decompose the Problem

Break the problem statement into architectural components:
- **UI/Presentation layer** — user-facing interfaces
- **Business logic** — core domain rules and use cases
- **Data layer** — persistence, caching, state management
- **IO layer** — external services, APIs, file system, network

For each component, identify key requirements: performance, concurrency, type safety, ecosystem maturity.

### 2. Research Package Registries

For each component domain and each candidate language, search for:
- Mature, well-maintained libraries
- Community adoption (stars, downloads, recent activity)
- Type safety and API ergonomics
- License compatibility

Sources: npm registry, pkg.go.dev, crates.io, and general web search.

### 3. Score Language x Library Combinations

Evaluate each combination against criteria:
- **Fitness**: How well does the language suit this component?
- **Ecosystem**: Quality and breadth of available libraries?
- **Performance**: Does it meet latency/throughput requirements?
- **Team familiarity**: Reasonable learning curve?
- **Interop**: Can it communicate with other components cleanly?

### 4. Recommend Communication Pattern

Based on the component decomposition, recommend how components should communicate:
- **In-process**: Direct function calls (single-language projects)
- **FFI/WASM**: For performance-critical cross-language calls
- **gRPC/Protobuf**: For service-to-service communication
- **Message queue**: For async, decoupled components
- **REST/HTTP**: For external-facing APIs

### 5. Generate Report

Write a structured recommendation to `docs/analysis/{problem-slug}-deps.md` containing:
- Component breakdown with assigned languages
- Library recommendations per component with rationale
- Dependency graph showing inter-component relationships
- Communication pattern diagram
- Risk assessment and migration path

## Output

Report the recommendation summary and the path to the full analysis document.
