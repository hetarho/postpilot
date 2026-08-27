---
change: '{{NN}}'
plan: '{{PLAN}}'
status: planning
title: '{{TITLE}}'
---

# {{NN}}. Change: {{TITLE}}

> A **change proposal** (a WHAT delta) that modifies the already-shipped behavior of [{{PLAN}}](../{{PLAN}}.md).
> Filled by /create-change through a user interview — a reviewable doc, not a one-line prompt.
> The STEPS live in the job that /create-change-job creates. Write it in English.

## As-is

<!-- How the part of {{PLAN}} this change touches works today. The starting point. -->

## To-be

<!-- The behavior/contract that must be true after the change. Be concrete. -->

## Scope / Non-goals (regression boundary)

<!-- What changes, and what must NOT change. -->

## Acceptance Criteria

<!-- The testable criteria that mark this change done. Copied into the job's Acceptance Criteria section. -->

1.

## Grounding

- Target plan: [{{PLAN}}](../{{PLAN}}.md)
- Constitution ([I1]–[I7]): [00.overview](../plan/00.overview.md) §3 (_The constitution_) — a change never breaks an invariant
- Affected policy / tech / config: <!-- -->
