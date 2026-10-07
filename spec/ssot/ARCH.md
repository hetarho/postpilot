# ARCH postpilot architecture
> r20 | Code placement and gates, including task-impact verification, pre-push CI/CD parity and a browser-owned media pipeline.

## decisions
- ARCH-1 [o] product: a paid product anyone may sign up for (→AUTH-1, →BILL) — photos + notes → a blog draft in the user's own voice → per-platform copy export for manual posting; ko/en UI. Behavior lives in the domain SSOTs; root PRD.md is a reference brief and ssot/ wins on conflict
- ARCH-2 [o] shape:
  | directory | holds |
  |---|---|
  | `proto/postpilot/v1` | Protobuf + Connect unary, buf v2 workspace at repo root |
  | `backend/` | Go 1.26 API + media worker |
  | `frontend/` | Vite 8 + React 19 + TypeScript 6 + Tailwind v4 + TanStack Router/Query + i18next SPA |
  | `spec/` | this system |
  - pnpm workspace, Node pinned by `.node-version`
- ARCH-3 [o] the proto contract is the only seam between sides; generated code (`backend/internal/gen`, `frontend/src/shared/api/gen`) is committed, never hand-edited, and consumed only through the adapter that owns it (BE `<context>/rpc`, FE `shared/api`); every hand-kept mirror of a proto enum (BE string enums and FE mirrors) is pinned by a test that walks the generated enum, and a default/fallback branch in such a mapping is a test failure, not a value ← builds must not depend on buf/sqlc being installed, and a silently degraded enum copy is how a typo becomes behavior
- ARCH-4 [o] the frontend is purely static: Cloudflare Workers static assets via `wrangler.jsonc` with SPA not-found handling; no SSR, no server-only FE code; `VITE_*` values are baked at build time so they carry public values only, secrets stay in backend env
- ARCH-5 [o] backend = one package per bounded context under `backend/internal/<context>`, named for the domain (auth authoring billing clip experiment fxrate generation googleauth guideline job llm mail memory modelcatalog plan platform post provider quality storage template tosspay usage voice voucher — every directory there but the generated `gen` (→ARCH-3) and the dev-only `devseed` (→ARCH-44))
  - start flat, split into domain/ · app/ · store/ · rpc/ only when the flat package is actually noisy ← no empty packages to satisfy a diagram
- ARCH-6 [o] BE placement:
  | what | where |
  |---|---|
  | composition wiring | only `cmd/api` and `cmd/media-worker`, with migrations owned by the API |
  | domain types and pure functions | `<context>/types.go service.go` |
  | use-cases and consumer-owned ports | `service.go ports.go` |
  | SQL + row↔domain mapping | `<context>/store` (sqlc sources `store/queries/*.sql`, output `store/sqlc`) |
  | Connect handler + proto↔domain mapping | `<context>/rpc` |
  | third-party services (LLM providers, R2) | wrapper contexts behind consumer-owned ports |
  | code with no business meaning (config, db, rpcserver, ids, health, migrations) | `internal/platform` |
  - a use-case that must commit across two contexts' tables in one transaction lives in `<owning-context>/app` over transaction-scoped ports the other contexts publish (`WithStore`/`NewTx` shaped, consumer-declared), never in either command ← ARCH-7 forbids touching another context's tables, and without a named home the composition root becomes the application layer
  - a port is named for the behaviour one use-case group needs and has one implementer per behaviour, never a mirror of a table surface
  - a generic queue or ledger (`job`, `usage`) exposes product-agnostic primitives and the product context composes them, so `job`/`usage` never name a product
- ARCH-7 [o] dependency rule, inward only: rpc / store / SDK adapters → context behavior → pure domain. The domain imports no proto, SQL rows, `database/sql`, JSON/DB tags, or transport; two anti-corruption mappers keep it so (handler proto↔domain, store row↔domain); no ORM, no DI framework, no generic repository; each context owns its tables and other contexts read through its published behavior, never its tables
- ARCH-8 [o] aggregates: an entity with its own lifecycle is its own root; a shared entity is referenced by id, never owned; relationships computable from stored data are not promoted to domain types (a projection only if performance demands it and it can be rebuilt); domain services are pure functions
- ARCH-9 [o] `internal/llm` is a hard boundary: nothing above it learns which provider answered, no provider SDK type appears above it, and the model choice is an input per stage rather than a global
- ARCH-10 [o] persistence: SQLite via `modernc.org/sqlite` (pure Go), WAL, one serialized writer connection plus a read pool; on every API process boot goose reads the stored schema version and applies only pending `//go:embed`-ed migrations under `backend/internal/platform/db/migrations/NNNN_<slug>.sql` before any context or listener; a failed migration kills the process so the deploy health gate rolls back; there is no separate migration command, and a write transaction never spans a provider call ← keeps the image CGO-free and distroless, and the schema can never disagree with the binary reading it
- ARCH-11 [o] long server work has a durable job record and polling; model work stays in the API under GEN, and native CLIP validation/rendering uses leased media workers under ARCH-45 and ARCH-50.
  - browser preparation/rendering is page-owned cancellable work with server-authorized identities and revision fences, not an in-process server execution job
  - API restart fails interrupted in-process model execution and reconciles durable media handoffs without replaying uncertain paid calls
- ARCH-13 [o] frontend = Feature-Sliced Design: layers `app → pages → widgets → features → entities → shared`, imports flow left to right only, same-layer cross-import forbidden except `entities`↔`entities` through `@x`; exactly one `index.ts` per slice and nothing reaches inside a slice; enforced by steiger and ESLint boundaries
- ARCH-14 [o] FE layer by what the thing is:
  | the thing | layer |
  |---|---|
  | a domain noun (model, api, mappers) | `entities/<noun>` |
  | a user action, a verb | `features/<verb>` |
  | a large self-contained block reused across pages | `widgets/<block>` |
  | a whole route/screen composing lower layers with no domain logic | `pages/<screen>` |
  | domain-agnostic and reused | `shared` |
  - the verb line: an entity `api` holds the noun's reads and its own CRUD mutations with their query keys, a feature is a user-facing action that renders UI, confirms, starts a job, or touches another noun — a bare hook nobody renders is entity `api`, not a feature
  - a page owns no transport, service call or state machine — it composes hooks that features/entities export
  - cross-noun cache dependencies (a post embeds its voice and template) are declared once in the depending entity through `@x` and exposed as one invalidation entry that verb features call, never by importing another noun's query keys
- ARCH-15 [o] FE segment by technical role only: `ui` · `model` (types, store, state machines, pure logic) · `api` (Connect calls + mappers) · `lib` · `config`; never `components/` `hooks/` `types/`
- ARCH-16 [o] `app` and `shared` are segmented, not sliced:
  - `app/providers` (query client, transport, theme, i18n runtime + namespace registration, error boundary) · `app/routes` (route groups, one file per group; a page exports its own search schema) · `app/model` · `app/styles`
  - only `App.tsx` `main.tsx` `index.ts` sit at the `app/` root
  - an i18n namespace is slice-shaped and its `{ko,en}` resources live in the owning slice's `config` segment, `app/providers/i18n` only assembles them (key parity stays tested)
  - `frontend/src/test` holds shared test harnesses and fixtures only
- ARCH-17 [o] `shared/api` owns the Connect transport, failure normalization, language mapping and the generated client; nothing outside it imports `shared/api/gen/**`, and proto service descriptors, message schemas and `@connectrpc/connect-query` are named only in `shared/api` and `entities/*/api` — pages, widgets and features consume hooks and domain types an entity exports (ESLint boundaries) ← a proto rename stops at one directory and a message rename at one entity
- ARCH-18 [o] pure FE layers (`shared/api` `shared/config` `shared/lib` `entities/*/model`) import no react / react-dom (ESLint boundaries)
- ARCH-19 [o] image work runs in the browser before upload: libheif WASM decode → 1024px downscale → JPEG q0.85 → direct PUT to R2; originals never reach the API; image pipeline code lives in the FE (`features/upload-photos` + `shared/lib/image`), never in Go ← the only practical HEIC decoder for Go is cgo
- ARCH-20 [o] design system: every generic control is a `shared/ui` primitive and slices never hand-roll one; classes resolve only to tokens registered in `@theme` in `app/styles/index.css` (no stock Tailwind colours, no arbitrary values); borders are the last resort; cards are rare and never nested; the base breakpoint is the phone and `sm:`/`md:` only add upward; `pnpm lint:style` enforces the token half; the full design language lives in THEME
- ARCH-21 [o] config: no setting is a literal in a component or handler — the FE reads Vite env through `frontend/src/shared/config`, the BE reads env into a typed struct validated at boot in `internal/platform/config`
  - both hold env-derived values and cross-slice/cross-context constants only, and neither imports a domain package ← platform importing a domain inverts ARCH-7
  - a product limit or default (durations, counts, byte caps, token budgets) is a named constant in the owning entity/context (`entities/<noun>/config`, `<context>/limits.go`) that the platform ctor merges env overrides into ← one file that every slice edits is a merge hotspot
  - slice-local UI tuning lives in `<slice>/config`
  - a new env var lands in `.env.example` (and `.env.production.example` once it ships) in the same task
  - formulas, prompt text, and the proto/DB schema are code, not config
- ARCH-22 [o] naming: Go package = context name, lower case, no underscores, the product's own nouns; FE kebab-case singular slices, PascalCase component files, camelCase elsewhere, named exports only
- ARCH-23 [o] auth mechanics: self-signup with the email as the login id and verification before the first session, Google sign-in, and the operator CLI beside it (`cmd/adduser`, or `api adduser` in the container); argon2id, HttpOnly cookie session; the session token never appears in a body, log, or URL; authenticated handlers read the actor from the interceptor-set context, never from the payload; every user-facing RPC except `/health` is 401 without a session (behavior in AUTH); internal media RPCs use separately authenticated worker identities under ARCH-47
- ARCH-24 [o] tests are mandatory for every implementation task: FE vitest (jsdom + Testing Library) beside the code, BE `go test` beside the code; a task's acceptance names the tests that pin it
  - task completion runs the tests added or modified by that task and existing tests for plausible side effects; unrelated full suites are reserved for the pre-push gate (→ARCH-31)
  - assess the complete task delta from its starting revision, including intermediate commits, deletions and uncommitted changes; FE consumers/shared harnesses and BE reverse dependencies/wiring belong in the impact assessment
  - import-based selection is an aid, not proof of complete coverage; shared contracts, migrations, authentication, media behavior or test configuration require the matching consumer/integration regressions, and an impact that cannot be bounded expands to the affected full suite
  - acceptance plus relevant lint, formatter and build/type checks must pass; record the selected commands and the impact rationale in the task result, and never count an empty selection as successful behavioral verification
- ARCH-25 [o] pre-push full FE verification:
  - `pnpm --filter ./frontend test`
  - `pnpm lint`
  - `pnpm lint:fsd`
  - `pnpm lint:style:probe && pnpm lint:style`
  - `pnpm build:web`
- ARCH-26 [o] pre-push full BE verification: `cd backend && test -z "$(gofmt -l .)" && go vet ./... && go build ./... && go test -timeout 30m ./...` ← two packages run real SQLite and the whole boot sequence and pass Go's ten-minute per-package default on their own; the flag bounds a hang without failing work that is merely long
- ARCH-27 [o] verify publishing automation stays absent (ARCH-34 I1): `pnpm lint:retirement`
- ARCH-28 [o] verify generated code (buf and sqlc run through Docker): `pnpm gen:proto && git diff --exit-code -- backend/internal/gen frontend/src/shared/api/gen` · `pnpm gen:sql && git diff --exit-code -- backend`
- ARCH-29 [o] format: `pnpm --filter ./frontend format` (Prettier; `dist/` and `shared/api/gen` are ignored) · `cd backend && gofmt -w .`
- ARCH-30 [o] verify skills: `pnpm exec haeram-spec-creator check` — the skills haeram-spec-creator installs into `.claude/skills` and `.codex/skills`, as `.haeram-spec-creator-lock.json` lists them, are package-managed and never hand-edited; `recomend-models`, in both directories, is the project's own skill and is edited in place
- ARCH-31 [o] verification has separate task-completion and pre-push stages
  - task completion and commits on main use ARCH-24's impact-selected checks; reproducing every CI/CD step locally is a pre-push obligation, not a per-task obligation
  - before push, run the full local equivalent of `CI`: `pnpm test:dev`, ARCH-25 + ARCH-27 + ARCH-28 + ARCH-30 on the pinned Node, and ARCH-26 plus the deploy Python unittests with `deploy/requirements-test.txt` installed
  - when the push can trigger `Deploy backend` or `Verify media`, also validate the production Compose layouts, build both deployed CPU images and execute all three media gates: production image smokes, CPU worker execution and separate-process CPU release in colocated and remote layouts; commands and local environment handling live in `docs/verification.md`
  - verify the final push candidate with the workflows' image targets, test flags and CPU/memory budgets; source/configuration/dependency changes invalidate affected evidence, while unchanged successful checks need not be repeated within that pre-push verification
  - a local pass does not certify GitHub runners, GHCR, SSH, production health or external CORS; after push inspect `CI` and every triggered deployment/media workflow for that revision, record the failing workflow/job/step and first meaningful error, and resolve the release failure without hiding it or repeatedly rerunning all task tests
- ARCH-32 [o] deploy: FE → Cloudflare Workers static (dashboard build `pnpm --filter ./frontend build`, deploy `npx wrangler deploy`); API → `ghcr.io/hetarho/postpilot-api` from `backend/Dockerfile` (golang:1.26-alpine builder, `CGO_ENABLED=0`, `gcr.io/distroless/static-debian12:nonroot`) behind the shared edge Caddy in `deploy/edge`; a separately versioned media-worker image runs beside it or on another Linux Docker host with a CPU or NVIDIA runtime; `main` → prod, `develop` → staging; each service rollout health-gates and rolls back on failure, and only API startup migrates the API-local SQLite volume ← root DEPLOY.md is the ops runbook
- ARCH-33 [o] dependencies: before adding, upgrading, or configuring any library or service, read the current official docs (context7 MCP first, otherwise the official site), install through the package manager's latest resolver (`pnpm add`, `go get`), confirm the resolved version in the lockfile, copy env-var and config names verbatim from those docs, and use official scaffolds only when they target this exact stack (Vite SPA + TanStack Router, connect-go) ← stale-memory setups fail only at runtime
- ARCH-34 [o] invariants no task may break silently (stop and resolve with the owner first):
  | id | invariant |
  |---|---|
  | I1 | no destination-platform publishing automation or credentials |
  | I2 | the canonical post is a block array and every platform output is derived from it (POST) |
  | I3 | generation separates observe from write with every model choice explicit — ordinary generation observes once and calls one writer; an explicit single-factor writing test prepares exactly2/4/8/16 complete posts once, shares observation unless the observer is the varied factor, and uses human binary decisions with no further provider calls (GEN, MODEL) |
  | I4 | voices are mutually isolated per account and a post selects at most one (VOICE) |
  | I5 | long server work is a durable job; browser work has page ownership and durable admission/publication fences (ARCH-11) |
  | I6 | image work happens in the browser (ARCH-19) |
  | I7 | migrations are embedded and run at boot (ARCH-10) |
- ARCH-35 [o] doc truth order: behavior → `ssot/<DOMAIN>.md`, placement and gates → this file, progress → STATE.md; root PRD.md and DEPLOY.md are reference docs and ssot/ wins on conflict
- ARCH-36 [o] real-binary media and clip-input smokes run in the image that executes each supported media profile, including its fonts, filters and runtime libraries; CPU images keep the nonroot runtime and a GPU profile is additionally exercised on matching NVIDIA hardware; env gates keep these smokes outside ARCH-26
- ARCH-37 [o] production media changes use impact-selected behavioral tests at task completion (→ARCH-24) and the matching real-image smoke before push (→ARCH-31); introducing or enabling a production GPU profile additionally requires a run on supported GPU hardware, with unavailable hardware recorded as an unmet activation gate; documentation, image packaging and isolated diagnostic tooling may complete with build/configuration/CPU checks while recording GPU execution as unverified, never as a passing GPU substitute; the smokes never join ARCH-26
- ARCH-38 [o] until closed beta opens, the smokes run beside the deploy instead of upstream of it: the pushed image does not depend on them, the rollout does not wait, and a failure is reported rather than withheld from production; the blocking gate returns when closed beta opens ← the smokes are ten of the deploy's twelve minutes while the owner is the only user, and ARCH-32's health gate and rollback still stand
- ARCH-39 [o] each media image is checked at build time against every filter, decoder, encoder and muxer its declared execution profiles name; a hardware profile also passes the runtime capability check in ARCH-53 ← compiled-in encoder names alone do not prove that a device and driver can execute them
- ARCH-40 [o] construction: a collaborator a service needs to do its job is a constructor argument; `Set*`/`With*` setters exist only for optional behaviour whose absence is a legal, tested mode; the API composition root is `loadPlatform → buildContexts → registerJobs → serve`; the media-worker root wires configuration, transport and execution adapters only, with no application rule bodies in either command and no database migrations in the worker ← a forgotten setter is a nil at runtime and only the slowest test package can catch it
- ARCH-41 [o] proto layout: `buf.yaml` breaking uses `PACKAGE`, so a message or rpc may move between files inside `postpilot.v1`; one proto file and one service per rpc family (`clip_template` `clip_source` `clip_generation` `clip_render` …), split when a service passes ~15 rpcs or two task streams keep colliding on it; moving a message between files is not a wire change; moving an rpc to a new service changes its path, so the BE and FE halves of a service split ship in one rollout ← `FILE` breaking makes the split itself a breaking-check failure
- ARCH-42 [o] local `pnpm dev` is a Node launcher that runs the host Vite client beside the Air-supervised Compose API and a separate CPU media worker; each API child reaches readiness only after the production boot sequence, every documented dev-only environment override must pass the same bounded owning-context validation as its production default, and a pre-listener failure remains an outage with its named cause in the API log; a launcher flag that writes to the database (`--seed`, →ARCH-44) runs with the api stopped ← SQLite serves one writer and a write racing a live boot half-applies

- ARCH-43 [o] removal is forward-only: a migration file once applied is never edited, and a removed shared enum value keeps its name and number reserved

- ARCH-44 [o] dev-only tooling is not a bounded context and is excluded from ARCH-5's list:
  - the local test fixture is `internal/devseed`, which owns the installation it produces as data and reaches every context through consumer-owned ports the composition root adapts from that context's own store, never its tables ← a fixture whose shape lives in the composition root is one nobody can review
  - a command that can destroy account data is its own `cmd/` the production image does not build (`cmd/seed`), while operator commands that must exist on the box stay dispatched from `cmd/api` ← a delete-every-account path reachable from the deployed ENTRYPOINT is one mistyped argument from an outage
  - the fixture wipes account-owned rows only and leaves installation-wide curation standing ← a seed that erased the registered models would leave a fresh install unable to generate

- ARCH-45 [o] server media execution is separate from the API even on one host: bounded analysis-copy verification, explicit qualified native preparation and native final rendering/output checks use the same leased worker contract locally or remotely.
  - supported browser preparation/rendering owns full-original decoding/transcoding, layout, caption frames and background sampling; it requests no server rasterization or rendering slot
  - the API owns artifact authorization, planning, credits, export entitlement and durable state; verification never accepts unbounded client claims (→CLIP-203)
- ARCH-46 [o] a versioned media job freezes its operation, owning attempt and project revision, source fingerprints, required renderer/asset versions and output contract; requests carry bounded domain data and artifact references, never shared filesystem paths or arbitrary subprocess commands
- ARCH-47 [o] workers pull authorized jobs and report progress/results through authenticated internal API operations scoped to their deployment and current lease; worker identities confer no user-session authority, direct SQLite access or provider/credit authority
- ARCH-48 [o] workers download inputs and upload stage outputs through short-lived access to private S3-compatible storage; durable records carry object references rather than signed URLs, heavy render intermediates stay in bounded worker-local workspaces, and no shared volume or warm cache is required for correctness
- ARCH-49 [o] media claims are atomic finite leases renewed by heartbeats, with a distinct token per execution attempt; progress and completion require the current unexpired lease and active owning job, so an expired, superseded or duplicated report cannot publish or settle twice
- ARCH-50 [o] API restart reconciles persisted media-stage requests and accepted results without failing a valid remote lease or losing a completed handoff; only a durable continuation may advance the owning job, and recovery never replays an uncertain paid provider call
- ARCH-51 [o] accepted cancellation, deletion and supersession prevent further media claims and result publication; a worker stops when told to cancel or when it cannot renew before lease expiry, and abandoned local files and unaccepted uploaded outputs have recoverable cleanup
- ARCH-52 [o] worker concurrency, encode/decode threads and CPU, memory, temporary-disk and GPU resource budgets are explicit bounded deployment settings, initially one media job per worker; same-host deployment reserves capacity for the API and never derives safe concurrency from host core count alone
- ARCH-53 [o] acceleration modes are `cpu`, `auto` and `nvenc`: CPU always uses a validated CPU profile, auto selects a compatible validated GPU profile only after real codec/filter execution checks pass and otherwise selects CPU, and nvenc refuses work without its required GPU capability; GPU failure never suppresses input or output validation
- ARCH-54 [o] host placement, worker API address, identity, CPU/GPU image and GPU device exposure are explicit deployment configuration; application startup neither discovers a hosting provider nor installs drivers or provisions machines
- ARCH-55 [o] each media execution attempt records its selected profile, renderer/tooling and asset versions plus bounded resource settings; a retry records its own selection, and artifact reuse checks source, plan and profile compatibility before accepting cached output
- ARCH-56 [o] release verification covers same-host CPU and remote CPU execution, lease loss/reclaim, API restart, cancellation/completion races and object cleanup; enabling a GPU profile additionally requires representative end-to-end timings and output-quality checks under CLIP-163 before auto may select it
- ARCH-57 [o] delivery order: separate CPU execution with both deployment layouts and three-environment deployment guides → later measure and validate one NVIDIA profile → enable validated automatic selection → tune bounded concurrency and threads; live host provisioning and topology migration are separate operator actions, and the API and worker keep an explicit contract-version compatibility check through rollout and rollback
- ARCH-58 [x] AMD/Intel/Apple GPU backends, automatic machine provisioning/scaling and a cloud-provider-specific scheduler are deferred
- ARCH-59 [o] the default deployment remains API plus CPU media worker on the existing VPS; README links to DEPLOY.md procedures for one server without GPU, one server with NVIDIA GPU, and one API server plus a separate NVIDIA worker server, covering per-host services/env, image/device selection, prerequisites, private connectivity, verification and rollback; repository delivery includes configuration and locally verified procedures, while installing on another host, moving live workers and running real-GPU validation happen later under operator control

- ARCH-60 [o] browser media stack: existing React editor plus Mediabunny/WebCodecs for media input/output, with a PixiJS 8 WebGL compositor qualified against an optimized Canvas 2D control.
  - WebGPU is optional only after the same qualification; API or hardware-hint presence alone never proves acceleration or faster export
  - resvg-wasm supplies compatible cached bundled-font/SVG ink where needed; existing SoundTouchJS core and local loudness processing retain pitch-preserving audio semantics
  - PixiJS Filters 6 may supply needed effects for PixiJS 8; import only qualified effects, and use authored shaders/geometry for styles it cannot reproduce
  - Remotion web-renderer is an isolated comparison candidate, not a second production framework unless the measured comparison supports an explicit architecture change
- ARCH-61 [o] a frozen browser render snapshot is versioned domain data derived from ClipEditPlan, not executable AI HTML, CSS, shader or JavaScript.
  - bind project/plan revision, authorized source identities/fingerprints and time transforms, component/font/asset versions, exact text/placement/intervals and immutable speech hashes/volume
  - one pure output-frame/time evaluator drives preview and export; seeking, cancellation and supersession invalidate older decode callbacks and publication
  - runtime signed URLs and decoded media never become durable plan fields; cross-version reuse requires explicit compatibility checks
- ARCH-62 [o] browser media placement follows FSD: generic demux/decode/encode/mux and resource adapters stay in shared/lib/media; clip frame evaluation, placement, styles and component drawing stay behind entities/clip-preview with clip-design's published rules.
  - features own preparation/render admission, upload/result promotion and page lifecycle; the existing workspace composes controls and preview without owning transport or codec internals
  - React edits plan state and draws editor controls; frame generation does not require a React render or DOM seek per output frame
- ARCH-63 [o] browser export pipelines selected source-range decoding, local composition and asynchronous encoding inside a bounded Worker/OffscreenCanvas path.
  - use sequential/sample-window decoding and a small active transition set; bound in-flight VideoFrames, audio samples, encoded packets and cached textures, closing/releasing each after use
  - per-frame queue pressure waits for capacity; final drains and error boundaries use flush without repeatedly draining a healthy pipeline
  - export advances explicit frame timestamps without real-time playback waits or frame drops; preview may reduce resolution/cadence while preserving the composition clock
  - source audio reads selected ranges plus codec/time-stretch guard samples, rather than decoding a long original wholesale; speech remains natural-speed and exact under DUB
- ARCH-64 [o] local component drawing reuses text/word masks, plates, outlines and static shadows; transforms, gradient/mask state, filters and animated geometry derive from frozen output time.
  - cache keys include every property affecting ink/layout; movement-only changes do not rerasterize text, and filters use bounded areas including declared bleed
  - typography/glyph fallback, sRGB/alpha handling and sampled-background rules remain shared contracts; whole-frame CPU readback is not the ordinary effect path
  - render-specific background measurements read transformed original frames locally and are bound to that plan/version, never borrowed from analysis proxies or another render
- ARCH-65 [o] original/proxy uploads and finished output use short-lived owner-bound direct private-storage transfers; browser output is muxed incrementally with explicit packet backpressure.
  - prefer a supported seekable file writer; otherwise use bounded private origin temporary MP4 spooling, then a bounded Blob fallback; MP4 position-aware writes are not blindly concatenated or PUT as sequential chunks
  - spooling stores no original, decoded audio, caption frame or credential; completion/cancellation/logout and abandoned-run recovery reclaim files, and another account cannot reopen them
  - local output becomes durable only through the existing revision/verdict/upload-promotion fences; upload retry never re-encodes valid bytes or repeats AI work
- ARCH-66 [o] browser media dependencies are locked to qualified versions and carry a version-specific license/source inventory before distribution.
  - MIT components preserve notices; MPL-2.0 components provide covered source access including modifications; bundled fonts retain their OFL notices
  - the shipped minified JS/WASM exposes the applicable notices and source-access path; audit bundled dependencies as well as the top-level package name
  - hardware codec availability is a runtime capability, not a promise supplied by a library license; optional comparison/commercial SDKs do not enter the production bundle implicitly
- ARCH-67 [o] browser qualification uses reproducible identified-device runs with cold/warm phase timings, peak resources and output checks, including Canvas 2D/PixiJS comparison and blur-in/neon/glitch/ember stress cases.
  - browser integration tests exercise real Worker, codec, fonts, canvas, audio and storage behavior; unit mocks cannot establish performance or hardware use
  - analysis evaluation isolates high-quality references, native proxies and browser proxies at matched inputs/model/endpoint/prompt versions, with human-grounded labels and a separately approved live-call budget
  - record missing hardware, provider credentials or review as unmet gates; compile success and synthetic structure tests do not certify semantic accuracy or production voice readiness
- ARCH-68 [o] release qualifies and activates browser final rendering separately from browser analysis preparation.
  - final-render activation requires complete component/output parity and real-device performance; analysis activation additionally requires server artifact-verification and semantic-quality evidence
  - HTTPS, private-storage CORS, module Worker URLs, versioned font/WASM assets, notices/source access and temporary-output cleanup are checked in the static deployment
  - same-host CPU workers retain finite job/queue/resource budgets; an unsupported browser never triggers an implicit native job, GPU rental or desktop installation

- ARCH-69 [o] guided setup, personal learning, EDIT authoring and unified writing-test presentation use XState v5 statecharts and official React actors as their transition authority.
  - typed owner/session/revision/operation-fenced events and synchronous guards admit each action once; no reducer wrapper or second mutable shadow flow
  - actors survive viewport changes and do not turn stop/close into paid work or server cancellation
  - recovery reads durable records/jobs and never restores a snapshot that reissues provider calls or canonical writes
  - test actual actors through duplicate/stale events, owner changes, interruption, mode changes, binary decisions and publication recovery; pure projections remain framework-free
- ARCH-70 [o] development uses the main checkout and completes one dependency-ready task at a time.
  - read the current SSOT and task, record its start in STATE, implement its acceptance and run ARCH-24 checks
  - record the result, archive the completed task, update STATE and commit on main before starting the next task
  - keep correctness review within implementation and verification, with additional review only for concrete unresolved risks
  - preserve unrelated work and reconcile pending SSOT changes before implementation; ARCH-31 remains the pre-push gate

## constraints
- Node is pinned by `.node-version` (24.18.0); run FE verify on that version (fnm/nvm read the file) — newer local Node versions break the jsdom-based tests
- Docker is required for `pnpm gen:*` (buf, sqlc), `pnpm dev:api`, and the media smokes (ARCH-37); Go 1.26 for BE
- BE self-audit before done: the domain imports no proto/sql/transport · dependencies point inward · both mappers sit at the edges · ports are consumer-declared, behaviour-shaped and actually consumed · no shape-only packages · writes go through the single writer, no transaction across a provider call, a new migration is embedded · no inline config literal and no product constant in `platform/config` · required collaborators enter through the constructor · `cmd/api` and `cmd/media-worker` gained wiring only · `job`/`usage` name no product
- FE self-audit before done: every file sits in layer/slice/segment (ARCH-14, ARCH-15) · no loose files under `app/` (ARCH-16) · one-way imports and a single `index.ts` per slice · no `shared/api/gen` import outside `shared/api` and no proto symbol or `connect-query` import outside `shared/api` + `entities/*/api` (ARCH-17) · a page owns no transport or state machine (ARCH-14) · no inline config literal and no product constant in `shared/config` (ARCH-21) · a new i18n key lands in the owning slice's `config` (ARCH-16) · every control from `shared/ui`, tokens only (ARCH-20)
- dev ports: web 2564, api 7678 (compose maps 7678 → 8080; containers use 8080)

## chg
- r19 261007 ARCH-24✎ ARCH-25✎ ARCH-26✎ ARCH-31✎ ARCH-37✎ every-task full local CI/media verification→impact-selected task completion and full CI plus applicable deploy/media gates before push; exact workflow/revision failure evidence required
- r20 261007 ARCH-70+ ARCH-31✎ separate task submission/integration lifecycle→one dependency-ready task implemented, verified and committed directly on main
