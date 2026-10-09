---
name: hexa-inference
description: Configure and check the inference hexa uses: providers, keys, model tiers, the frontier path, and the local benchmark. Use when the user asks to "set up a model", "configure inference", "which model", "add a provider", "benchmark models", or "hexa inference".
---

# hexa inference: providers, tiers and the benchmark

hexa does not do the writing. A model does, and hexa decides whether the
result counts. This skill sets up the model and measures it.

## Set up

```bash
hexa bootstrap --dry-run       # show what would happen
hexa bootstrap                 # prerequisites, local server, models, config
hexa config                    # providers and model tiers in .hexa/project.json
hexa doctor                    # installation check
```

The models hexa pulls come from `.hexa/project.json`. There is no built-in
list; a project that configures no models gets none, and `hexa bootstrap`
says so. API keys are environment variables only. Never write a key into a
file.

## The frontier path

`hexa scaffold`, `hexa build` and `hexa harden` delegate to a logged-in
`claude` CLI, so `claude --version` is the only check. `hexa do` uses the
local server first and the frontier path as a fallback.

## Tiers

`inference.tier_models` in `.hexa/project.json` maps tiers to models. A
caller names a tier, never a model; that is what lets a model be swapped by
editing configuration. The rule `no-model-name-outside-inference` in
`.hexa/ADR-rules.toml` enforces it in code.

## Measure before you trust

```bash
hexa bench agentic             # run the corpus through the loop, per model
```

A model's leaderboard rank does not predict how it does in the agent loop.
Rust and Go are strict, and weaker models fail both where they pass
TypeScript. Measure the model you intend to use, on the language you intend
to use it for, and keep the number.

The details are in `docs/INFERENCE.md`.
