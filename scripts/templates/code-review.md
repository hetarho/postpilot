---
review: '{{NN}}'
status: report
title: '{{TITLE}}'
created: '{{DATE}}'
scope: code-quality
---

# Code Review {{NN}}: {{TITLE}}

> Read-only code-quality and architecture review. This report records refactor
> opportunities only; implementation belongs in `spec/jobs/`. Write it in English.

## Scope

<!-- What was reviewed: full codebase, frontend, backend, specific modules, or current diff. -->

## Grounding

- Branch/worktree:
- Commands:
- Docs consulted:
- Constraints:

## Architecture Baseline

<!-- Summarize the repo rules used for judgment: FSD, backend package boundaries, SSOT, state-machine policy, etc. -->

## Findings

<!-- Use R001, R002, ... and keep each finding evidence-backed. -->

### R001 Example finding title

- Priority: P1 | P2 | P3
- Area:
- Evidence:
- Why it matters:
- Recommendation:
- Suggested job split:

## Cross-Cutting Themes

<!-- Patterns that appear across multiple findings. -->

## Questions / Tradeoffs

<!-- Anything not proven enough to call a finding, but worth deciding before implementation. -->

## Verification Notes

<!-- Commands that passed, failed, or were blocked. Include blockers honestly. -->

## Candidate Jobs

<!-- Each item should be a coherent implementation job candidate for /create-refactor-job. -->

1.
