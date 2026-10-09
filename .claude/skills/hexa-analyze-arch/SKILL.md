---
name: hexa-analyze-arch
description: Grade the architecture of a project and fix what the grade names. Use when the user asks to "check architecture", "find dead code", "validate boundaries", "architecture health", "detect circular dependencies", or "hexa analyze".
---

# hexa analyze-arch: grade the architecture, then fix what it names

`hexa analyze .` walks the import graph and grades the boundaries. The grade
is the second gate. This skill runs it, reads it, and fixes what it names
one file at a time, with the analyzer itself as the evidence command.

## 1. Grade

```bash
hexa analyze .                 # grade, boundary violations, rule violations
hexa analyze . --json          # score_components, dead_exports, unused_ports
hexa analyze . --exit-code     # nonzero on any error, for a gate
```

Read `score_components` first. A score is a sum, and a sum with no components
gets a story attached. The four components are `violations`, `circular_deps`,
`dead_exports` and `unused_ports`. Each has a list under the same name.

## 2. Fix, one file at a time

For each finding, the fix is one edit and one evidence command:

```bash
hexa do run "<what to change>" --file <path> --evidence "hexa analyze . --exit-code"
```

`hexa do` edits the file, runs the evidence command, and commits only if it
exits 0. A boundary violation in an adapter is fixed by importing through the
port; if the port does not re-export the type, add the re-export to the port
first. A dead export is removed, or made private when its own file uses it.
Before removing anything, run `hexa graph consumers <path>`.

## 3. Grade again

Run `hexa analyze .` after the last fix. The grade must not fall, and every
component the fixes targeted must be zero. Report the before and after
scores and the components, not a summary.

## What the display lines mean

`cohesion`, `duplication` and `god types` read Rust only and print `n/a` on a
tree with no Rust. `dead layers` and `orphans` read Rust, Go and TypeScript.
None of the five moves the grade; they are read next to it, so a nonzero
number there is worth a look and is not a failure.
