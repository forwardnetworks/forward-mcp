---
name: hexa-ADR-create
description: Create a new Architecture Decision Record with auto-numbering, dependency impact analysis, and validation gates
trigger: /hexa-ADR-create
---

# Create New ADR

Creates an Architecture Decision Record with comprehensive impact analysis to prevent
downstream breakage. Every ADR that modifies, deletes, or restructures code artifacts
must include a full consumer dependency map before it can be accepted.

## Phase 1: Gather Intent

1. Get the next available ADR number and schema by running:
   ```bash
   hexa adr schema
   ```
   This returns the next id, the template, valid statuses, and required sections.

2. Ask the user for:
   - **Title** (required)
   - **Brief context description** — why this decision is needed
   - **Decision type**: one of `add | modify | delete | restructure | migrate`
   - **Drivers** (what triggered this decision)

3. If a reserved placeholder exists (`ADR-{NNN}-reserved.md`), delete it after creating the real ADR

## Phase 2: Dependency Impact Analysis (REQUIRED for modify/delete/restructure/migrate)

**This phase exists because ADR-2026-04-05-0900 proved that deleting modules without tracing
all consumers leaves compilation broken in downstream crates.**

### 2a. Identify Affected Artifacts

For each artifact being modified/deleted/restructured:

```bash
# Find ALL consumers across the entire workspace
grep -r '<artifact-name>' --include='*.rs' --include='*.ts' --include='*.yml' \
  --include='*.yaml' --include='*.json' --include='*.toml' --include='*.md' \
  --include='*.html' --include='*.js' .
```

Build a **consumer dependency map**:
```
Artifact: <name>
├── Direct consumers (import/use/reference):
│   ├── crate/file:line — how it's used
│   └── ...
├── Transitive consumers (depend on direct consumers):
│   └── ...
├── Config references (Cargo.toml, package.json, CI, Dockerfiles):
│   └── ...
├── Documentation references (ADRs, CLAUDE.md, README, workplans):
│   └── ...
└── Test references (unit, integration, e2e):
    └── ...
```

### 2b. Cross-Crate Analysis (Rust-specific)

For Rust workspace changes:
```bash
# Check feature flags that gate the artifact
grep -r 'feature.*=.*"<artifact>"' --include='*.toml' .

# Check re-exports
grep -r 'pub use.*<artifact>' --include='*.rs' .

# Check conditional compilation
grep -r 'cfg.*feature.*<artifact>' --include='*.rs' .
```

### 2c. Build Verification Gates

Define explicit gates that the workplan MUST include:

| Gate | Command | Scope |
|------|---------|-------|
| Workspace compile | `cargo check --workspace` | All Rust crates |
| TypeScript compile | `bun run check` | All TS code |
| Unit tests | `bun test` / `cargo test` | Per-crate |
| Integration tests | Defined per-ADR | Cross-crate |

**CRITICAL**: The workplan derived from this ADR MUST include a validation step that
runs these gates AFTER every phase that deletes or restructures artifacts.

### 2d. Blast Radius Classification

| Impact | Definition | Action Required |
|--------|-----------|-----------------|
| **CRITICAL** | Breaks compilation in another crate | Must fix in same phase as deletion |
| **HIGH** | Breaks runtime behavior | Must fix before next phase |
| **MEDIUM** | Stale reference (docs, comments) | Fix in cleanup phase |
| **LOW** | Cosmetic (naming, outdated counts) | Fix opportunistically |

## Phase 3: Write the ADR

Create `docs/adrs/ADR-{NNN}-{kebab-slug}.md` with all sections:

### Required Sections

- **Title**: `# ADR-{NNN}: {Title}`
- **Status**: `**Status:** Proposed`
- **Date**: today's date (YYYY-MM-DD)
- **Drivers**: from user input
- **Context**: problem, forces, constraints, alternatives
- **Impact Analysis** (for modify/delete/restructure/migrate):
  - Consumer Dependency Map (from Phase 2a)
  - Cross-Crate Dependencies (from Phase 2b)
  - Blast Radius table (from Phase 2d)
  - Build Verification Gates (from Phase 2c)
- **Decision**: clear imperative language ("We will...")
- **Consequences**: positive, negative, mitigations
- **Implementation**: phased table with validation gate per phase
- **References**: related ADRs, issues, documents

### Schema Reference

Valid statuses: `Proposed | Accepted | Deprecated | Superseded | Abandoned`

Required frontmatter: `**Status:**`, `**Date:**`, `**Drivers:**`, `**Supersedes:**` (optional)

## Phase 4: Validate the ADR

Before marking complete:

1. **Cross-reference check**: Every ADR in "Supersedes"/"References" exists
2. **Consumer completeness**: Re-run grep from Phase 2a, verify every hit is accounted for
3. **Gate completeness**: Every implementation phase has at least one validation gate
4. **Workplan alignment**: If a workplan will be created, verify it includes all gates

## Anti-Patterns (Lessons from ADR-2026-04-05-0900)

| Anti-Pattern | Problem | Fix |
|-------------|---------|-----|
| Module-scoped impact analysis | Only checked a subset of workspace crates, missed consumers in other crates | Always grep the ENTIRE workspace |
| Missing validation gates | Workplan had "delete X" but no "verify compile" between phases | Every phase must end with a workspace-wide build check |
| Documentation-only analysis | Listed docs mentioning a module but not code importing it | Code consumers are CRITICAL; docs are MEDIUM |

## Multi-Agent Safety

The `hexa adr schema` command reserves the ADR number atomically via `POST /api/adr/reserve`. This prevents two concurrent agents from creating ADRs with the same number.

## Quick Reference

| Command | What it does |
|---------|-------------|
| `/hexa-ADR-create` | Create a new ADR with full impact analysis |
