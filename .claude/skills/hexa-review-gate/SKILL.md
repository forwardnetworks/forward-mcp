---
name: hexa-review-gate
description: Run review in both directions and enforce governance as the agent acts — REVIEW.md passes on every PR, hooks as approval gates, and non-interactive agent steps in CI that stop at the production gate. Use when the user says "review this PR", "set up code review", "add an approval gate", "block that action", "governance", "who approves", or asks to ship a change.
---

# Hex Review Gate — Stage 5, Deploy

**What changes:** review runs in both directions — the agent reviews incoming
PRs and addresses comments on its own — and governance is enforced as the agent
acts rather than discovered in a review cycle. The agent does everything up to
the production gate and nothing past it.

Traditional: review capacity was planned around human output. A PR waits for a
reviewer to read all of it, quality varies with the reviewer's load, and the
backlog grows. Reviewing each line by hand made sense when a person wrote each
line; it cannot keep up once agents write most of the diff.

## Prerequisites

`CLAUDE.md`, the skills that encode the policies the passes enforce, and a repo
with branch protection requiring a code owner's approval.

## Part 1 — review passes

1. **Write the review policy as `REVIEW.md` at the repo root**, divided into the
   passes the organization actually cares about, with an explicit definition of
   what counts as Important versus a nit, and what to skip.
2. **Set the human threshold.** Findings do not approve or block on their own;
   branch protection still requires a code owner. Gate merges on severity counts
   only if the tally is machine-readable and the team has tuned the passes first.
3. **Let the agent address comments on its own PRs.** The thread records both the
   request and the change. For PRs the agent opened, sweep unresolved comments
   and failing checks until the PR is green and waiting only on approval.
4. **Feed findings back into `CLAUDE.md`.** When review flags the same mistake
   twice, the correction goes into `CLAUDE.md` as part of that review — and
   because review reads `CLAUDE.md`, the mistake is caught from the next PR on.
   Review should also flag when a change has made `CLAUDE.md` outdated.
5. **Tune monthly.** Rate findings, cap nit volume, exclude generated paths and
   anything CI already enforces.

```markdown
# Review instructions

## Passes
Run three passes and tag each finding with its pass:
- Bugs: logic errors, broken edge cases, subtle regressions
- Security: injection risks, authentication gaps, PII in logs
- Compliance: the change matches the spec, the plan and our design principles

## What Important means here
Reserve Important for findings that would break behavior, leak data
or breach a policy. Style and naming are nits.

## Cap the nits
Report at most five nits per review; summarize the rest as a count.

## Do not report
Generated files under src/gen/ and anything CI already enforces.
```

The compliance pass is what makes Stage 2 and Stage 3 pay off: it checks the diff
against the committed spec and plan, which is a check no reviewer could make when
the plan lived in someone's head. In a hexagonal codebase add a fourth pass —
boundary violations — backed by `hexa analyze .` so the deterministic check runs
alongside the judgment call.

## Part 2 — hooks as approval gates

A skill is an advisory control: it makes the agent likely to apply a policy while
the code is written, and nothing forces a session to comply. A hook is the
deterministic layer behind it. The skill makes violations rare; the hook makes
them close to impossible.

Hooks can **allow**, **ask**, or **block**:

- *Allow / block, no human involved* — build-phase guardrails: protected paths,
  frozen packages, credentials in the diff, formatter after each edit. Keep these
  fast and scoped to the file that changed; heavier checks belong at commit or PR.
- *Ask* — the release gate: pause the action until a named person approves.

An approval prompt during the build puts a person back on the critical path of
every session running in parallel, so keep ask-gates at the boundaries that
genuinely need them.

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [{
          "type": "command",
          "command": "${CLAUDE_PROJECT_DIR}/.claude/hooks/production-gate.sh"
        }]
      }
    ]
  }
}
```

```bash
#!/bin/bash
# Production deploys require a named release authorization
cmd=$(jq -r '.tool_input.command' < /dev/stdin)
if [[ "$cmd" == *"deploy"* && "$cmd" == *"production"* ]]; then
  if [ -z "$RELEASE_APPROVAL" ]; then
    echo "Production deploys need a release authorization." >&2
    exit 2   # exit 2 blocks the action; the message goes to the agent
  fi
fi
exit 0
```

Rules that hold:

- **A block must explain itself.** When a hook stops an action, the reason and the
  route to approval appear in the agent's output. A silent block just gets worked
  around.
- **Team hooks live in `.claude/settings.json` in git.** Non-negotiable hooks live
  in managed settings owned by the platform admin, where an engineer cannot switch
  them off.
- **List the gates before writing them.** Engineering leadership, change
  management and compliance name the human approvals that must survive; each one
  becomes a hook.

In a hexa project the handlers are already wired: `hexa hook` implements the
session, pre-edit, pre-bash and pre-agent hooks installed by `hexa init`, and
`hexa analyze . --exit-code` is the blocking architecture check. Add project gates alongside
them rather than replacing them.

## Part 3 — agent steps in the pipeline

Run the agent non-interactively for the judgment steps a script cannot do:
triaging a failed build, summarizing a flaky test, drafting a changelog. Start
read-only, then add write steps behind the existing gates.

```yaml
- name: Triage failed build
  if: failure()
  run: >
    claude -p "Read the build log at out/build.log. Identify the most
    likely cause, say whether the failure looks flaky or real, and write a
    three-line summary for the PR thread." >> triage.md
```

The governing principle: **the agent may act up to the production gate and cannot
pass it.**

- Branch protection turns anything the agent writes into a PR — no path to main.
- Agent jobs run sandboxed, with short-lived scoped tokens and no standing
  production credentials.
- Deployment is exposed as tools (deploy, status, rollback) scoped per
  environment, so the agent's powers are an allowlist rather than a shell script
  holding credentials.
- Autonomy is tiered by environment: free in development, gated in production,
  somewhere between in staging.
- **Rollback is the most rehearsed path in the pipeline** — one command, exercised
  regularly in staging. Stage 6 calls it, so it must be proven in advance.

## Governance

Separation of duties survives: the agent that wrote the code has no way to
approve it. `REVIEW.md` applies to every PR; findings, fixes, ratings and
approvals live in the PR history, so the PR is the audit record. Each
non-interactive run acts under its own identity, so the log separates what the
agent did from what the engineer who triggered it did.

## Measure it

- **Leading:** time to first review (should fall to minutes); share of review
  comments resolved without a human touching the branch; time spent waiting at
  each approval gate; share of pipeline failures triaged without paging a human.
- **Lagging:** defects and vulnerabilities caught before merge versus those
  escaping to production; gate violations reaching production before and after
  the hooks; DORA measures.

## Next

A merged change is watched by `hexa-close-loop`.
