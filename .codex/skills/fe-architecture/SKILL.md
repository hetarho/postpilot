---
name: fe-architecture
description: >-
  The frontend placement gate — invoke BEFORE creating, moving, or naming ANY frontend file (anything under
  frontend/src), and when reviewing whether FE code sits in the right place. Use when the user says "add a
  page/feature/component", "where does this file go", "wire up a provider", "build the X screen", or during
  /implement-job for any FE surface. It carries the Feature-Sliced Design decision procedure (layer → slice →
  segment), the app-layer segment rule, and a self-audit checklist. The authoritative rules live in
  spec/ARCHITECTURE.md §3 — this skill is the actionable procedure + audit, not a second copy of the rules.
  Read §3 first when a case is ambiguous.
---

# Frontend architecture gate (FSD)

**SSOT = [spec/ARCHITECTURE.md](../../../spec/ARCHITECTURE.md) §3** (layers/segments/placement). This skill does
**not** restate the rules to own them — it gives the *procedure* to apply them and the *audit* to catch drift. If a
placement is ambiguous or you're unsure a rule still holds, **open §3 and read it** — don't guess from memory. That is
the whole point: this is the step whose skipping makes an `app/` layer drift flat.

## When this fires
Before you create/move/rename **any** file under `frontend/src`, and whenever you review FE structure.
`/implement-job` invokes this for every FE job.

## The decision procedure — where does this file go? (§3.1)

Ask in order:

1. **A domain noun** (a post, a photo, a style profile, a generation job) — its model, its API calls, its mappers?
   → `entities/<domain-noun>`.
2. **A user-facing action (a verb)?** → `features/<verb>` (e.g. `features/upload-photos`,
   `features/edit-with-ai`, `features/copy-for-platform`).
3. **A big self-contained block reused across pages?** → `widgets/<block>`.
4. **A whole route/screen?** → `pages/<screen>` (composes lower layers; holds no domain logic).
5. **Domain-agnostic & reused?** → `shared` (transport, config, ui primitives, lib helpers).

Then pick the **segment** by technical role — never `components/`/`hooks/`/`types/`:
`ui` (React components) · `model` (types, store, state machines, pure logic) · `api` (Connect calls + proto↔domain
mappers) · `lib` (slice-internal helpers) · `config` (slice-local constants; tuning values come from
`shared/config`, not inline literals).

**Imports go one way only:** `app → pages → widgets → features → entities → shared`. Same-layer cross-import is
forbidden (only `entities`↔`entities` via `@x`). Each slice exposes one `index.ts`. Enforced by `steiger`
(`pnpm --filter ./frontend lint:fsd`) and ESLint.

## The app layer is segmented, not flat

`app` and `shared` are **not sliced** — but they are **still divided into segments**, never a pile of loose files.
The `app` layer's segments are its technical roles: **`providers/`** (query client, transport, theme, error boundary,
i18n + their config), **`routes/`** (router), **`model/`** (app-shell state, app-level pure logic), **`styles/`**
(global CSS). Only the true entrypoint (`App.tsx`, `main.tsx`, `index.ts`) may sit at the `app/` root. A provider or
client file dropped directly in `app/` is a **placement bug** — put it in `app/providers/`.

> Current state: `frontend/src/app` still holds `query-client.ts` and `router.tsx` at its root (scaffold shape). The
> first FE job that touches them should move them into `app/providers/` and `app/routes/` rather than adding more
> loose files beside them.

## Generated code
`frontend/src/shared/api/gen/**` is buf output — never hand-edit it, never import it outside `shared/api`. Slices
consume the transport + mappers `shared/api` exposes, so a proto rename doesn't ripple into `features/`.

## Naming
kebab-case singular slices; PascalCase component files; camelCase/kebab elsewhere; **named exports only**.

## Self-audit (run before you call the FE work done)
- [ ] Every new file lands in a `layer/slice/segment` per the procedure above (no generic `components/`/`hooks/`).
- [ ] No loose files under `frontend/src/app` except `App.tsx`/`main.tsx`/`index.ts` — providers/config/router sit in
      `app/providers|routes|model|styles`.
- [ ] Imports are one-way; no same-layer cross-import; slices expose a single `index.ts`.
- [ ] No import of `shared/api/gen/**` outside `shared/api`; no hand-edits to generated files.
- [ ] No config literal inline in a component — it comes from `shared/config`.
- [ ] Gates green: `pnpm lint` · `pnpm --filter ./frontend lint:fsd` (steiger) · `pnpm build:web`.
