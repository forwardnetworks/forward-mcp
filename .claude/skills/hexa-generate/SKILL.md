---
name: hexa-generate
description: Generate code within a hexagonal adapter boundary. Use when the user asks to "generate adapter", "implement port", "create adapter", "code adapter", or "implement interface".
---

# Hex Generate — Implement an Adapter for a Port Interface

## Parameters

Ask the user for:
- **adapter** (required): Target adapter path relative to `src/adapters/` (e.g., "secondary/git-adapter", "primary/cli-adapter")
- **port** (optional): Specific port interface to implement (e.g., "IGitPort"). If omitted, infer from adapter name.
- **language** (optional, default: typescript): One of `typescript`, `go`, or `rust`
- **test_style** (optional, default: london): `london` (mock-first) or `chicago` (state-based)
- **max_iterations** (optional, default: 5): Max feedback loop iterations

## Step 1: Resolve Port

If port is not specified, infer from adapter path:
- `secondary/git-adapter` -> IGitPort + IWorktreePort
- `secondary/llm-adapter` -> ILLMPort
- `secondary/build-adapter` -> IBuildPort
- `secondary/fs-adapter` -> IFileSystemPort
- `secondary/ast-adapter` -> IASTPort
- `primary/cli-adapter` -> ICodeGenerationPort + IWorkplanPort
- `primary/http-adapter` -> ICodeGenerationPort + ISummaryPort
- `primary/agent-adapter` -> ICodeGenerationPort + IWorkplanPort + ISummaryPort

## Step 2: Load Context

Load AST summaries at appropriate levels:
- **L1** of all `src/core/ports/**` — skeleton of all port interfaces for contract awareness
- **L2** of the specific port file — full method signatures being implemented
- **L2** of existing adapter code in `src/adapters/{adapter}/**` — current state
- **L1** of `src/core/domain/**` — domain entity skeletons for type references
- **L3** of the specific file being edited (on-demand)

Stay within token budget: ~100K total, reserve 20K for response.

## Step 3: Write Failing Tests First (TDD Red)

Create unit tests in `tests/unit/{adapter_name}.test.{ext}`:
- Test every method defined in the port interface
- Mock all dependencies (other ports) using London-school style
- Include happy path, error cases, and edge cases (empty input, null, timeout)
- Assert return types match the port contract

Run tests to confirm they FAIL before implementation exists.

## Step 4: Implement the Adapter (TDD Green)

Generate the adapter in `src/adapters/{adapter}/index.{ext}`:
- Implement every method from the resolved port interface
- Use dependency injection for all external dependencies
- Include proper error handling with domain-specific error types
- Add JSDoc/godoc/rustdoc for all public methods
- Keep file under 500 lines; split if needed

## Step 5: Feedback Loop (Compile -> Lint -> Test -> Refine)

Run quality gates in order, up to max_iterations:

1. **Compile**: `npx tsc --noEmit` / `go build ./...` / `cargo check`
2. **Lint**: `npx eslint src/adapters/{adapter}/ --ext .ts` / equivalent
3. **Unit Test**: `npx vitest run tests/unit/{adapter_name}.test.ts` / equivalent
4. **AST Diff** (optional): `npx hexa summarize --file src/adapters/{adapter}/index.ts --level L2`

On failure:
1. Parse structured error output from the failing gate
2. Load L3 context of the failing file
3. Apply targeted fix using Edit tool (not full rewrite)
4. Re-run only the failing gate before full cycle

If all gates fail after max_iterations, report the errors and escalate for human review.

## Step 6: Refactor (TDD Refactor)

After all gates pass, clean up:
- Eliminate code duplication (DRY)
- Ensure single responsibility per function
- Verify consistent naming with project conventions
- Check error messages are descriptive
- Confirm all public API has documentation

Re-run quality gates after refactoring to confirm nothing broke.

## Step 7: Run Architecture Analysis

Run `hexa analyze .` to verify the new adapter does not introduce boundary violations or circular dependencies.

## Output

Report: adapter name, port implemented, quality gate results (compile/lint/test), iteration count, files created/modified.
