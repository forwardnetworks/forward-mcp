---
name: hexa-project-output
description: Every hexa project created by an agent must include README.md and a startup script. Defines the required output structure for scaffold-validator. Use when creating new projects, examples, or applications.
always_load: true
---

# Project Output Requirements

When creating a new project, application, or multi-file codebase, you MUST include these files alongside the code:

## Required Files

### 1. README.md

Every project directory must contain a `README.md` with:

```markdown
# {Project Name}

{One-line description}

## What This Does

{2-3 sentences explaining the project purpose and key features}

## Prerequisites

- {runtime} (e.g., Python 3.10+, Node.js 18+, Rust 1.75+)
- {any dependencies or system requirements}

## Quick Start

```bash
./start.sh
```

## Usage

{Show the main commands/API with examples}

## File Structure

```
project/
  file1.py    — {purpose}
  file2.py    — {purpose}
  start.sh    — startup script
  README.md   — this file
```

## How It Works

{Brief architecture explanation — what talks to what}
```

### 2. start.sh (or run.sh)

An executable startup script that:
- Installs dependencies if needed (pip install, npm install, etc.)
- Sets up any required directories or config
- Runs the application
- Is idempotent (safe to run multiple times)

```bash
#!/usr/bin/env bash
set -euo pipefail

# Install deps (if needed)
pip install -r requirements.txt 2>/dev/null || true

# Run
python3 main.py "$@"
```

Make it executable: `chmod +x start.sh`

## File Creation Order

When building a multi-file project, create files in this order:

1. **Directory structure** — `mkdir -p` the project directory
2. **Domain/model files** — data structures, types
3. **Library/backend files** — business logic, storage
4. **Main/CLI files** — entry point, argument parsing
5. **start.sh** — startup script
6. **README.md** — documentation (write LAST so it accurately reflects what was built)

## Verification

After creating all files, ALWAYS:
1. Run `start.sh` or the main entry point to verify it works
2. Show the output to confirm correctness
3. If it fails, fix the issue and re-run

## Examples

### Python CLI App
```
todo-app/
  models.py     — TodoItem dataclass
  storage.py    — JSON file storage backend
  todo.py       — CLI entry point (argparse)
  start.sh      — #!/bin/bash — python3 todo.py "$@"
  README.md     — usage docs
```

### Node.js Web App
```
web-app/
  src/
    server.js   — Express server
    routes.js   — API routes
  package.json  — dependencies
  start.sh      — npm install && node src/server.js
  README.md     — API docs
```

### Rust Binary
```
my-tool/
  src/
    main.rs     — entry point
    lib.rs      — core logic
  Cargo.toml    — dependencies
  start.sh      — cargo run --release -- "$@"
  README.md     — usage docs
```
