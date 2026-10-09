---
name: hexa-summarize
description: Generate token-efficient AST summaries of source files. Use when the user asks to "summarize project", "ast summary", "token context", "generate context", "project overview", or "what does this project do".
---

# Hex Summarize — Tree-Sitter AST Summaries for LLM Context

Generates tree-sitter AST summaries at configurable detail levels, compressing source files to ~10% token count while preserving structural information needed for code generation and navigation.

## Parameters

Ask the user for:
- **target** (optional, default: "."): File or directory path to summarize
- **level** (optional, default: L1): Summary detail level (L0, L1, L2, or L3)
- **format** (optional, default: text): Output format — `text`, `json`, or `markdown`
- **filter** (optional): Glob pattern to filter files (e.g., `**/*.ts`, `src/adapters/**`)
- **max_tokens** (optional, default: 50000): Maximum total tokens for output
- **include_tests** (optional, default: false): Whether to include test files

## Summary Levels Explained

| Level | Name | Tokens/File | What It Captures |
|-------|------|-------------|------------------|
| L0 | Index | ~5 | Filename, language, line count |
| L1 | Skeleton | ~50 | Exports, imports, dependencies |
| L2 | Signatures | ~200 | Full type signatures, params, return types |
| L3 | Full | ~2000 | Complete source code (use sparingly) |

## Execution

### 1. Discover Files

Find all source files (`.ts`, `.go`, `.rs`) in the target path, excluding `node_modules`, `dist`, `target`, and optionally test files.

### 2. Run hexa summarize

For a single file:
```bash
npx hexa summarize <file> --level <L0|L1|L2|L3>
```

For a directory:
```bash
npx hexa summarize <directory> --level <L0|L1|L2|L3>
```

If a filter is specified, apply it to limit which files are summarized.

### 3. Token Budget Management

Estimate tokens using `word_count * 1.3` heuristic. If total exceeds max_tokens:
1. Drop L2 files to L1
2. Drop L1 files to L0
3. Prioritize: ports > domain > adapters > infrastructure

### 4. Format Output

- **text**: Plain text with `---` separators between files
- **json**: Structured JSON with filePath, language, level, exports, imports, dependencies, lineCount, tokenEstimate per file
- **markdown**: Table format with file, language, lines, tokens, exports columns

### 5. Write Output

Display the summary to the user. If json or markdown format, also write to `docs/context/{level}-summary.{ext}`.

## Output

Report: level used, number of files summarized, estimated token count, and compression ratio (original tokens vs summary tokens).
