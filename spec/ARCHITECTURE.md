# ARCHITECTURE

The structural SSOT: **where code goes and which way dependencies point.** The product-level SSOT is
[PRD.md](../PRD.md) — it owns *what* postpilot does and the stack decisions; this document owns *placement*.
The invariants that no plan may break live in [spec/plan/00.overview.md](plan/00.overview.md) §3.

Derived from the PRD (§6 stack) and the code as it stands. When code and this doc disagree, one of them is a bug —
say which in the job.

## 1. The shape

```
postpilot/
├─ proto/postpilot/v1/         the contract. Protobuf + Connect (unary). buf generates both sides.
├─ backend/                    Go API. Connect handlers, SQLite, worker goroutine, LLM providers.
│  ├─ cmd/api/                 composition root: config → migrations → adapters → server
│  └─ internal/
│     ├─ <context>/            one package per bounded context (§2)
│     └─ platform/             no business meaning: config, db, rpcserver, ids, health
├─ agent/                      macOS companion. Outbound publishing client, loopback setup, local Hermes/browser.
├─ frontend/src/               static SPA. Vite + React 19 + TS, Tailwind v4, TanStack Router/Query, FSD (§3)
└─ spec/                       plan · changes · jobs · code-review · policy · tech (the workflow SSOT)
```

Two rules cut across both sides:

- **The proto contract is the only seam.** Neither side reaches past it. Generated code
  (`backend/internal/gen/**`, `agent/internal/gen/**`, `frontend/src/shared/api/gen/**`) is committed, never
  hand-edited, and consumed only through the adapter layer that owns it (BE: a context's `rpc/`; agent: its
  postpilot API adapter; FE: `shared/api`).
- **The frontend stays purely static** (PRD §3.1). It is served as Cloudflare Worker static assets — no SSR, no
  server-only frontend code. The optional Mac companion is a separate local runtime, not a frontend server; a change
  that puts server-only code into the SPA still breaks the deploy model.

### 1.1 Mac companion boundary

The `agent/` module exists only for capabilities that must remain on the user's Mac: postpilot device credentials in
Keychain, Naver login/profile state, local Chromium/CDP and Hermes execution. It makes authenticated outbound calls to
the same Connect API and may serve setup UI on loopback only. It never opens a public listener or imports backend
`internal` packages/frontend source; its generated client is a separate consumer of `proto/`.

The API remains the durable source of truth for publish jobs. The agent is an executor with a lease, not another
database or an authority over post state. Local implementation follows the same inward dependency rule: composition
root → polling/publishing behavior and consumer-owned ports → pure state transitions, with Keychain, launchd,
browser and Hermes as outer adapters. See [paired-local-publishing-agent](tech/paired-local-publishing-agent.md).

## 2. Backend — domain-first Go

### 2.1 Context layout

One package per **bounded context** under `internal/`, named for the domain, not the layer. Start flat; split into
`domain/` · `app/` · `store/` · `rpc/` only when the flat package has actually become noisy.

| Kind of code | Where |
|---|---|
| wiring: build the server, read config, run migrations, inject adapters | `cmd/api` |
| domain types, value objects, pure domain functions | `internal/<context>/types.go` · `service.go` (→ `domain/` after the split) |
| use-cases, consumer-owned ports | `internal/<context>/service.go` · `ports.go` (→ `app/`) |
| persistence: SQL + row↔domain mapping | `internal/<context>/store/` |
| transport: Connect handler + proto↔domain mapping | `internal/<context>/rpc/` |
| a third-party service (LLM provider, R2) | a supporting wrapper context behind consumer-owned ports |
| no business meaning: config, db handle, rpc mux, ids, health, migrations | `internal/platform/` |

The `internal/llm` port is a hard boundary (PRD §6.4): the layers above it never learn which provider answered, and
no provider SDK type appears above it. Observation and writing choose their model *per stage* (PRD §3.3), so the port
takes the model choice as input rather than reading it from a global.

### 2.2 The dependency rule — inward only

```
rpc/ · store/ · SDK adapters  →  context behavior (app/)  →  domain model (pure)
```

The domain model knows nothing of proto, SQL rows, `database/sql`, JSON/DB tags, or transport. Two anti-corruption
mappers keep it that way: the handler maps `proto↔domain`, the store maps `row↔domain`. No foreign type crosses
inward. No ORM, no DI framework, no generic repository. Each context owns its tables; another context reads through
its published behavior, never its tables.

### 2.3 Aggregate boundaries

An entity with its own lifecycle is its own aggregate root. A shared entity is referenced **by id**, never owned.
Relationships that are computable from stored data are not promoted to domain types — materialize a projection only
if performance demands it, and only if it can be rebuilt. Domain services are pure functions.

### 2.4 SQLite and the worker (PRD §3.5, §6.3)

- **One serialized writer connection**; WAL mode. Reads may use the pool.
- **Migrations are `//go:embed`-ed and run at boot** in `cmd/api`. A failure kills the process — the deploy's
  `/health` gate then rolls back to the previous image (`DEPLOY.md §2`). There is no separate migration container.
- **Generation is a job record, not a blocking RPC.** `StartGeneration` returns a `job_id`; a worker goroutine inside
  the API process consumes the queue; the client polls `GetGeneration`. On restart, jobs left `running` are swept to
  `failed`. Edits (F-6) ride the same queue.
- **Never hold the write transaction across a provider call.** Call the model, then persist.

## 3. Frontend — Feature-Sliced Design

### 3.1 Layers, slices, segments

```
app → pages → widgets → features → entities → shared
```

Imports flow left to right only. Same-layer cross-import is forbidden (`entities`↔`entities` via `@x` is the single
exception). Every slice exposes exactly one `index.ts`; nothing reaches inside a slice. `steiger`
(`pnpm --filter ./frontend lint:fsd`) and ESLint enforce this.

**Layer** — pick by what the thing *is*:

| It is… | Layer |
|---|---|
| a domain noun (post, photo, style profile, generation job): its model, api, mappers | `entities/<noun>` |
| a user-facing action, a verb (upload-photos, edit-with-ai, copy-for-platform) | `features/<verb>` |
| a large self-contained block reused across pages | `widgets/<block>` |
| a whole route/screen; composes lower layers, holds no domain logic | `pages/<screen>` |
| domain-agnostic and reused (transport, config, ui primitives, helpers) | `shared` |

`shared/ui` is the design system: every generic control lives there and nowhere else, and every slice styles itself
with the semantic tokens in `app/styles/index.css`. The visual rules — and the four hard rules on primitives, tokens,
borders, and cards — are [tech/design-language.md](tech/design-language.md); `pnpm lint:style` enforces the token
half.

**Segment** — pick by technical role, never by kind (`components/`, `hooks/`, `types/` are wrong):
`ui` · `model` (types, store, state machines, pure logic) · `api` (Connect calls + proto↔domain mappers) ·
`lib` (slice-internal helpers) · `config` (slice-local constants).

### 3.2 `app` and `shared` are segmented, not sliced

They have no slices — but they are still divided into **segments**, not a pile of loose files. `app`'s segments:
`providers/` (query client, transport, theme, error boundary), `routes/` (router), `model/` (app-shell state),
`styles/` (global CSS). Only `App.tsx`, `main.tsx`, and `index.ts` belong at the `app/` root.

### 3.3 The transport seam

`shared/api` owns the Connect transport and the generated client. Slices call it through `shared/api`'s public
surface; nothing outside `shared/api` imports `shared/api/gen/**`. A proto rename therefore stops at one directory.

### 3.4 Image work runs in the browser (PRD §6.2)

HEIC decode (libheif WASM) → 1024px downscale → JPEG q0.85 → direct PUT to R2. Originals never reach the API. This is
what keeps the Go image CGO-free and distroless, so it is a placement rule, not an optimization: image pipeline code
belongs in the frontend (`features/upload-photos` + `shared/lib`), never in Go.

## 4. Config

No setting is a literal buried in a component or a handler.

- **Frontend** — `frontend/src/shared/config` reads Vite env (`import.meta.env`) and exposes typed values.
- **Backend** — `backend/internal/platform/config` reads env into a typed struct, validated at boot.
- A new env var is added to `.env.example` (and `.env.production.example` when it ships) in the same job.

Excluded, because they are code and not config: formulas, prompt text, and the proto/DB schema.

## 5. Naming

- Go: package name = context name, lower case, no underscores; canonical domain nouns match the product's words.
- FE: kebab-case singular slices; PascalCase component files; camelCase elsewhere; **named exports only**.
