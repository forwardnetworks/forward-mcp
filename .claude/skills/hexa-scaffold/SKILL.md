---
name: hexa-scaffold
description: Scaffold a hexagonal project. Use when the user asks to "create a hexa project", "scaffold", "new ports and adapters project", or "hexa init".
---

# hexa-scaffold

The scaffold is a verb, not a prompt. This skill decides which form to run.
It does not generate files itself.

## The floor only

Deterministic, no model call, passes its own tests immediately.

```bash
hexa init <dir> --scaffold --lang <rust|go|ts>
```

Use this when the user wants a starting point and will write the code.

## The floor plus the project

The frontier path builds the description onto the skeleton. Two gates: the
language's test command, and the architecture grade.

```bash
hexa scaffold "<one-sentence description>" --target <dir> --lang <rust|go|ts> --grade A
```

Use this when the user describes what the project should do. Ask for the
language if it is not stated. Do not lower `--grade` below A without being
told to.

## After either

```bash
hexa analyze <dir>
```

Report the grade and any violations. Do not edit the scaffold by hand to fix
a violation. Run `hexa do` with the failing check as the evidence command.
