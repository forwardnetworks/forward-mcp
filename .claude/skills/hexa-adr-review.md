---
name: hexa-ADR-review
description: Review code changes against existing Architecture Decision Records
trigger: /hexa-ADR-review
---

# Review Code Against ADRs

## Steps

1. Get the current git diff: `git diff --cached` or `git diff HEAD`

2. Read all ADR files from `docs/adrs/` to understand architectural decisions

3. For each changed file, check:
   - Does it violate any boundary rules from ADRs?
   - Does it contradict any accepted decisions?
   - Should a new ADR be written for this change?

4. Report findings:
   - **Compliant**: changes align with existing ADRs
   - **Warning**: changes touch areas covered by ADRs but may not violate them
   - **Violation**: changes directly contradict an accepted ADR
   - **New ADR needed**: significant architectural change without an ADR

## Key ADRs to check
- The project's own ledger: `hexa adr list`
- The rules every scaffold carries: `.hexa/ADR-rules.toml`
