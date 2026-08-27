# {{NN}}. {{TITLE}}

> {{TITLE}} — one-line summary | Scope: FE|BE|FS | Status: planning
> Filled by /create-plan through a user interview. A plan states _what must be true_ (the WHAT),
> in enough detail to re-implement on another client. Write it in English.

## Purpose

<!-- Why this feature exists. The user/product problem. -->

## Scope / Non-goals

<!-- What is in, and what is explicitly out. Non-goals prevent scope creep. -->

## Grounding

- Constitution ([I1]–[I7]): [00.overview](00.overview.md) §3 (_The constitution_)
- Architecture (placement): [ARCHITECTURE.md](../ARCHITECTURE.md) — BE §2 · FE §3
- PRD / policy / tech basis: <!-- the sections and docs this depends on -->

## Design

<!-- How it works — key decisions, contracts (proto/RPC), data model, job/state transitions, UI approach.
     Include architecture placement: which FSD layer/slice/segment (FE, ARCHITECTURE.md §3.1) or Go
     context/package (BE, §2) each new piece lands in. -->

## Acceptance Criteria

<!-- Declarative / EARS. The testable criteria that make this feature "true". Copied into the job's Acceptance Criteria. -->

1.

## Policy / Config Impact

<!-- Rules to land in spec/policy/**, plus every config value the feature introduces or changes
     (thresholds, caps, timeouts, model ids, defaults…). Config is never a magic literal buried in a
     component or handler: FE reads it through `frontend/src/shared/config`, BE through
     `backend/internal/platform/config`. Note which side owns each value and how it is supplied
     (env var, constant, DB row). /create-plan also creates/updates the relevant policy docs. -->
