---
job: '{{JOB}}'
type: '{{TYPE}}'
source: '{{SOURCE}}'
plan: '{{PLAN}}'
status: todo
title: '{{TITLE}}'
---

# Job {{JOB}}: {{TITLE}} ({{TYPE}})

> Implementation work doc. Source spec: [{{SOURCE}}](../{{SOURCE}}.md).
> /implement-job {{JOB}} builds it via the two checklists below. When done, reflect the result into the
> SSOT (plan/policy/tech) and set status: done. Write it in English.

## Acceptance Criteria (from {{SOURCE}})

<!-- The source spec's acceptance criteria. After building, verify each is true in the running code. -->

- [ ] A1 …

## Implementation Checklist

<!-- How to build it, top to bottom. [P] = parallel (different files, no dependency). Flag (gen)/(migrate).
     Config values are never magic literals — FE via shared/config, BE via internal/platform/config. -->

- [ ] T001 …

## Grounding

- Constitution ([I1]–[I7]): [00.overview](../plan/00.overview.md) §3 (_The constitution_)
- Architecture (placement): [ARCHITECTURE.md](../ARCHITECTURE.md) — FE §3 (layers/slices/segments) ·
  BE §2 (context layout, dependency rule). Invoke `/fe-architecture` · `/be-architecture` for the
  surfaces this job touches.
- PRD / policy / tech touched: <!-- -->

## Affected files (blast radius)

<!-- Exact paths found from the source spec + a code grep — nothing outside this scope is touched.
     For each, note its target placement: FE layer/slice/segment (§3.1) or BE context/package (§2). -->

## Verification / DoD

- [ ] Every **Acceptance Criteria** item above is true in the current code
- [ ] (if type=change) no regression of the existing plan's acceptance criteria
- [ ] Codegen / migration applied (if any): `pnpm gen:proto` · migrations embedded and run at boot
- [ ] FE `pnpm build:web` · `pnpm lint` · `pnpm --filter ./frontend lint:fsd` / BE `go vet ./... && go build ./...` (Docker) pass (if any)
- [ ] Constitution sanity ([I1]–[I7]): nothing in the diff breaks an invariant
- [ ] Architecture self-audit passed (the relevant `/fe-architecture` · `/be-architecture` checklist)

## Review

- [ ] `/code-review` applied (rejections noted with reason) · for non-trivial logic, `/codex:review --background`

## After completion — reflect into the SSOT

- [ ] Update `plan/` to the new reality · update affected `spec/policy/**`·`spec/tech/**` · config values in their owner
- [ ] (if type=change) move the `changes/` source doc to `changes/archive/`
- [ ] 00.overview progress board ✅ · this doc's frontmatter `status: done`
