# MODEL providers, model catalog, experiments
> r35 | Explicit eligible models, private single-factor writing tests and safe versioned request composition/inspection.

## decisions
- MODEL-1 [o] `backend/internal/llm` is the only way a model is called: no adapter package or provider SDK is imported anywhere except under `internal/llm/…` and in `cmd/api`, enforced by `internal/llm/boundary_test.go` over `go list -deps`; every completion, voice-design, voice-confirmation and speech operation carries an explicit admitted model/profile reference through a provider-neutral port with no default (→ARCH-9 →MODEL-77)
  - observe, write and analyze retain their explicit stage ModelRefs; speech operations never masquerade as text completions
- MODEL-2 [o] errors normalize to `ErrModelUnavailable` `ErrProviderDisabled` `ErrRateLimited` `ErrUnsupported` `ErrBadOutput` `ErrOutputTruncated`, plus a `ProviderError` that keeps provider prose as diagnostic detail while supporting `errors.Is`; `llm.Failure` is the one stable mapper to `MODEL_UNAVAILABLE` `MODEL_RATE_LIMITED` `MODEL_UNSUPPORTED` `MODEL_OUTPUT_INVALID` `MODEL_OUTPUT_TRUNCATED` `UNKNOWN_FAILURE`; provider text is never primary UI copy or an interpolated param
- MODEL-3 [o] output ending `finish_reason: length` with no usable content, including partial JSON a caller parser rejects, maps to the truncated reason; when the provider reported a reasoning token count, `TruncatedError`'s technical detail names the reasoning/visible split ← the remedies are opposite: a body that filled its budget wants a larger one, a body the model never wrote because it reasoned through the budget wants a lower effort for that purpose or another model; the user-facing string is the same for both
- MODEL-4 [o] `ErrRateLimited` means the provider refused for rate reasons — the caller's quota, the account's, or the gateway's upstream pool — attributes nothing to a tier, and may arrive as an HTTP 429 or as an upstream `code: 429` inside an HTTP 200
- MODEL-5 [o] check capabilities before network work: unsupported vision, video input or structured output returns ErrUnsupported; a plain-text path checks the flag and omits unsupported schema.
  - request structured output when declared, retaining the existing json_schema request without strict and caller parser fallback; schema ownership stays with callers
  - schema conformance is not semantic-origin accuracy; missing origin information follows GEN-84 without changing canonical-output failure handling

- MODEL-6 [o] every call runs under `LLM_STAGE_TIMEOUT` (5 min); the `openai_compatible` adapter requests a stream and joins it server-side so a long draft does not idle out an intermediary, and the stream never leaves the process; a stream ending without `[DONE]` or a finish reason is a truncated answer failing as `ErrBadOutput`; a 404 means "model gone" only when the body says so; `Usage` and `FinishReason` are preserved beside an error so a failed candidate keeps billable evidence
- MODEL-7 [o] `ReasoningEffort` accepts `none minimal low medium high xhigh max`; empty means no decision; `unset` is an internal/yaml sentinel that omits the whole wire key; resolution is the operator's `reasoning_effort` override for the purpose the call is made for → the request/stage value → nothing sent ← the override is a property of a registration, not of a model: one model may observe at one strength and write at another in a single run, whereas one blanket value silently changes photo observation whenever writing is tuned
- MODEL-8 [o] the nested `reasoning: {effort}` wire object is an OpenRouter dialect enabled only when the provider declares `reasoning_format: openrouter`; other OpenAI-compatible endpoints omit it even when an effort was supplied; an unknown format stops boot; `none` is sent explicitly; `reasoning.exclude` is forbidden ← excluding the returned trace stops neither generation nor billing of reasoning tokens; reasoning and visible output share the completion budget
- MODEL-9 [o] stage reasoning policy: observe `low` · write/revise `low` · a template request `low` (→TMPL-58) · analyze has no field and sends nothing (the model's own adaptive behaviour, the most permissive setting); per-user effort selection is rejected ← the right effort is a measurement of a model against a task
- MODEL-10 [o] connections are files; models, speech profiles and recommendation sets are curated rows
  - `backend/config/providers.yaml` declares exactly one completion/catalog provider (`id` `adapter` `base_url` `api_key_env` `reasoning_format`) and may declare one separate speech connection (`id` `adapter` `base_url` `api_key_env`)
  - completion catalog ids remain scoped to the single completion provider; the speech connection never serves those ids or changes saved text-model selections
  - the file ships at `/config/providers.yaml` (`PROVIDERS_CONFIG`), accepts a mounted replacement, and contains no models, profiles, prices or recommendations
  - unknown fields/adapters, malformed connections, completion-provider count other than one or speech-provider count above one stop boot; omitted/unconfigured speech leaves speech work unavailable without stopping ordinary generation
- MODEL-11 [o] `api_key_env` names an environment variable read at boot and never written to the file; an unset key is not a boot failure — every model is listed `disabled` with reason `API key not configured` and cannot be selected — and the entry is still validated so a bad `base_url` cannot hide behind a missing key; `api_key_env` is optional, and a keyless endpoint (a local Ollama, vLLM, LM Studio) is enabled as is with no Authorization header
- MODEL-12 [o] the registry reads its models through an injected `llm.ModelSource` on every request, so curating a model takes effect for the next call rather than the next deploy; an empty catalog is a valid state (a fresh install curated nothing — the answer is an empty picker and a trip to `/admin/models`, not a refused boot); boot never contacts the provider's catalog
- MODEL-13 [o] `catalog_models` is the curated-model list and `catalog_model_purposes` holds its registrations across five purposes, curated on the five tabs of 모델 관리:
  - `photo-analysis` · `style-analysis` · `writing` · `image-generation` · `video-generation`
  - both are global (what an installation offers is an operator decision) and only master may read or change them (`ModelCatalogService`, in the interceptor's master set →AUTH-18)
- MODEL-14 [o] registered purposes map to user stages: photo-analysis → observe, style-analysis → analyze, writing → write, whose active selection also answers a template request (→TMPL-58); generation purposes remain admin-only until a stage consumes them. Ordinary selectors show classified registrations, including locked paid grades with their required plan (→QUOTA-19 →QUOTA-20). Unregistered or unclassified refs are unavailable to ordinary new selections; operator curation retains the rows.
- MODEL-15 [o] each purpose enforces a capability gate at registration, server-side: `photo-analysis` requires `vision`, `image-generation` / `video-generation` the matching `image_output` / `video_output`, the text purposes take any model (`MODEL_PURPOSE_INELIGIBLE`); the admin tab force-filters its candidates to the same gate; capability drift after a refresh stops the stage at once (`stagesOf` re-checks the gate) while the registration row is kept and stays visible on its tab for the operator to uncheck — never auto-retired
- MODEL-16 [o] every new selection, pair/preset application, comparison-result model adoption and AI admission checks stage membership, provider availability, capability and account model entitlement. An already-admitted job retains its admission-period rights under QUOTA-61 while every actual call still enforces live compatibility and price safety. No client-supplied ref bypasses these gates.
- MODEL-17 [o] completion catalog candidates come live from `GET {base_url}/models?output_modalities=text,image,video` — unauthenticated plain `net/http` under `OPENROUTER_CATALOG_FETCH_TIMEOUT` (15 s), one unpaginated document cached in memory for `OPENROUTER_CATALOG_TTL` (5 min), bypassed and replaced by the admin's 새로고침 (`ListCatalog(refresh: true)`)
  - the modality query is mandatory; this catalog excludes speech, transcription, embeddings and rerank; speech profiles follow MODEL-77
  - only the operator path triggers the read; failure writes nothing and missing credentials never block completion-catalog browsing
- MODEL-18 [o] field mapping:
  | source | row |
  |---|---|
  | `id` | `model_id` verbatim (variant suffixes kept; `ModelRef.provider_id` stays `openrouter` and the slug before `/` is a grouping key) |
  | `name` | `label` |
  | `created` | `source_created_at` |
  | `context_length` | `context_tokens` |
  | `"image" ∈ input_modalities` | `vision` |
  | `"video" ∈ input_modalities` | `video_input` |
  | `"image"` / `"video" ∈ output_modalities` | `image_output` / `video_output` |
  | `"structured_outputs" ∈ supported_parameters` | `structured_output` |
  | presence of the `reasoning` object | `reasons` |
  | `reasoning.supported_efforts` | `reasoning_efforts` (a JSON array in the source's descending order — the order a selector offers) |
  | `reasoning.default_effort` | `reasoning_default_effort` |
  | `reasoning.mandatory` | `reasoning_mandatory` |
  | `reasoning.supports_max_tokens` | `reasoning_max_tokens` |
  | `"reasoning_effort" ∈ supported_parameters` | `reasoning_native_effort` |
  - `description` is display only and `default_enabled` is not persisted
  - unknown fields are ignored (this is the source's schema, not ours) and an entry missing `id` or `name` is skipped with a warning
- MODEL-19 [o] prices are decimal strings in USD per token, multiplied by 10⁶ with decimal string arithmetic (no float round-trip); the stored pair depends on the output modality — text (including image+text) keeps `prompt` / `completion` where a zero is a real free price; image-only uses `prompt` else `image_token` and `completion` else `image_output` (per output image token despite the spec's wording); video publishes no token price, so both columns stay empty and the screen says 토큰 단가 미공개; `pricing_checked_at` is the fetch time; prices inform estimates, hold pricing and free-path qualification (→MODEL-68) and are displayed to master only (→QUOTA-66), while reported provider cost remains authoritative for the usage ledger
- MODEL-20 [o] availability bookkeeping — `listed`, `last_seen_at`, the label/context/pricing snapshot and the reasoning capability — is written only by a successful live read
  - a failed read writes nothing, reports `fetch_error` to the operator and degrades the browse list to DB rows ← treating an outage as evidence would retire the whole catalog on the first hiccup
  - a model the source stopped offering is flagged `listed = 0` AND deregistered from every purpose in that same successful read — the row is kept, the registration-bound efforts and levels go with their registrations (→MODEL-7 →MODEL-57, as on an operator's uncheck), it disappears from every 모델 관리 tab (no badge, no banner) and returns as an unregistered candidate when the source offers it again; users' saved selections fall through the vanished-selection machinery (→MODEL-24) ← a registration nobody can be served is only noise the operator has to hunt through five tabs for, and the outage guard is already the write-only-on-success rule
- MODEL-21 [o] the operator's effort override is bounded by what the model publishes: `SetModelReasoning` refuses a value outside a published list (`ErrInvalidReasoning`), refuses `none` when reasoning is mandatory or the published list omits it, and refuses any effort on a model the source confirms does not reason; a falsy capability value is UNKNOWN, never "supports nothing" — a model whose accepted values are unpublished keeps all eight; `unset` is labelled with the model's own default effort ← it omits the wire key and therefore means the model's default, not off; the screen's option filtering is an affordance and the server rule is the contract
- MODEL-22 [o] drift is a flag, never an action: an override the source would refuse today is kept and still sent, with a warning on the admin row derived at read time as "would be refused if written now"
  - nothing about an override auto-corrects, clears or migrates, and the only automatic deregistration is delisting (→MODEL-20)
  - the browse response carries an unstored seventh field `reasoning_known` ← `reasons = 0` with no list is both "publishes no reasoning object" and "nothing has asked yet", which is what a row no successful live read has refreshed says and what every row says while the fetch is failing
  - a live candidate is known by construction, a stored row only if it says something
  - an unknown capability offers the full vocabulary and accepts any effort, a known non-reasoning model offers no control and accepts none
- MODEL-23 [o] selection memory holds account/stage active refs and explicit comparison slots; pair and manual recommendation writes remain atomic. Protected first-use initialization fills only absent active observe/analyze/write slots under MODEL-87, while every generation start still freezes and passes its chosen eligible refs explicitly.
- MODEL-24 [o] GetSelections distinguishes absent registration, disabled provider and temporary plan/balance restrictions.
  - an unavailable active ref remains stored and visibly unavailable until the owner explicitly changes it; automatic initialization never replaces it
  - removed comparison refs retain their existing once-visible missing/conditional-clear behavior
  - loading or failed catalog reads never imply removal; downgrade/insufficient credit preserve selections and history and never silently substitute a model
- MODEL-25 [o] manual selection, comparison and full recommendation application accept only compatible, registered, enabled and account-entitled refs; insufficient balance alone does not invalidate a saved choice.
  - saved pair refs stay distinct and larger test draft candidates pass MODEL-75; explicit full recommendation application validates all seven slots before an atomic write
  - MODEL-87 automatic active defaults are separate from full recommendation application and never create or alter comparison choices
- MODEL-26 [o] recommendation refs are validated against current registration, compatibility and plan entitlement at apply time; the operator's save checks registration and classification only (→MODEL-70). No tier may apply a set containing a ref outside its rights. Removed models remain readable in snapshots, and newly adopted active refs must pass the same gate as manual selection.
  - a fresh installation starts with one seeded set `balanced-2026-08` (Gemini/Qwen observe, GPT analyze, Claude/Grok write) that the operator edits or deletes like any other, without promising that every tier can apply it
- MODEL-27 [o] browser model projections include ids, labels, capabilities, the purpose-specific classification, plan eligibility/required plan, affordability/unavailability reasons and QUOTA-64's per-post credit figure, and carry no price, cost or pricing date (→QUOTA-66). Operator views additionally expose public descriptions/prices; keys, SDK payloads and base URLs never cross the wire.
- MODEL-28 [o] `/admin` is four routed tabs — 계정 관리 (the user-plan table, QUOTA), 모델 관리, 편수 기준 조합 (estimator-combo assignment, →QUOTA-25 →QUOTA-39) and 이용권 (the voucher list, GIFT)
  - 모델 관리 holds five purpose tabs, 추천 조합 (→MODEL-69) and 목소리 (→MODEL-83)
  - 모델 관리 shows the live catalog merged with DB state on the five purpose tabs, each capability-force-filtered, featured providers first in the `FEATURED_MODEL_PROVIDERS` order then the rest alphabetically, newest first within a provider, with client-side search over id and name, provider / capability / registered filters and a 정렬 control, over one response, on a virtualized list (`@tanstack/react-virtual`, `CATALOG_ROW_ESTIMATE_PX` 132, `CATALOG_ROW_OVERSCAN` 6) that virtualizes the window and keeps the page's single scroller while search and filters reach every model including unmounted ones
  - the 정렬 control: 기본 (the provider/newest order above) · 등급순 (→MODEL-57 order, unlevelled last, 기본 inside a level) · 가격 낮은순 · 가격 높은순, the price pair keyed on output price per million with input price as the tie-break and unpriced models last in both directions ← output tokens dominate the app's spend, so one key is enough
  - `ListCatalog` is read per purpose and each entry reports that purpose's effort and a spend signal — the recent reasoning-vs-completion split per model for that purpose's stage, read from the ledger through a consumer-declared port (the context never reads `usage_events`; the adapter translates the registry ref to the provider-local id; a model with no call carries nothing rather than zero)
  - the row's effort and 등급 Listboxes appear only on a tab the model serves, the 등급 one marked while unset (→MODEL-58)
  - `SetModelPurpose` writes the registration and its gate in one transaction
  - `UpdateModel` names its purpose and is refused server-side for one the model is not registered to
- MODEL-30 [o] one actual-writing test varies exactly one factor: a model ref at observe or write stage, a writing-style profile, a post-template structure, or one designated post-guideline slot.
  - freeze common material, attachment identities, target/length/tags, nonvaried profiles/template/rules, memory opt-in and selected memories, quality options, prompt/schema versions and entrant-specific values before generation
  - changing templates does not recalculate guideline scope, target length or tag count; changing one guideline keeps all other enabled rules and their order fixed
  - identical named template-answer material is available to every contestant; resolve every candidate's required inputs before start rather than inventing answers
  - seed-free tests accept explicit material/scenarios without requiring a saved post; fictional scenarios are labelled and never claimed as owner experience
- MODEL-31 [o] ordinary generation calls one prepared active writer; an explicit common test produces one complete validated PostContent per contestant without changing its source.
  - writer/style/template/guideline tests prepare common observations once; observer-model tests observe the same real attachments independently for each contestant then write with one fixed writer
  - observer tests require attachments and compatible models; every contestant produces a complete post, not isolated observation output or a short verification sample
  - a creation editor may enter the common two-contestant model format with retained material/context; larger formats use the same common test workflow
- MODEL-32 [o] candidate identities are blind on the wire until champion confirmation or explicit abandonment; persist opaque contestant IDs and one cryptographically shuffled initial bracket, stable through reload/viewport changes.
  - predecision responses include complete outputs and bracket positions, omitting model/setting identity, accounting, identity-bearing errors and identity-revealing prompt/source details; inspect only identity-safe shared material until the existing reveal boundary
  - later rounds reuse the same stored outputs and seeded progression; completed history reveals frozen identities/timing/tokens while supplier cost remains master-only
- MODEL-33 [o] tests accept up to sixteen entrants but run at most five candidate pipelines concurrently; queue/progress are bounded and durable, provider work runs outside database transactions, and a round decision never issues a provider call.
- MODEL-34 [o] a test progresses from queued/running generation to ready matches, then completed champion or explicit abandonment; partial/failed generation is recoverable under MODEL-35.
  - persist the account-owned test and job before provider work; all requested outputs must succeed before any bracket decision
  - matches contain two contestants and one human winner, with N-1 decisions for N entrants; no tie, bye or failed-candidate automatic advancement
  - test jobs do not block ordinary source-post writing, and test review is distinct from canonical publication
- MODEL-35 [o] partial/failed generation offers explicit failed-only retry or abandonment, retaining successful outputs and original frozen inputs. Retry is a new admitted job and never regenerates a successful entrant. Interrupted uncertain calls never repeat automatically; no failure silently changes format or awards a champion.
- MODEL-36 [o] champion confirmation changes no saved setting, default, active model or canonical post; subsequent actions are explicit and separately named.
  - a model champion may be adopted as its matching active stage only after live eligibility checks
  - a setting champion may be saved/used under MODEL-90; readable test outputs support manual copy/export
  - source-post output application is available only for a compatible model-factor test with an owned draft/review source, expected input/content revisions and unchanged frozen writing assignments; other setting-factor results are exported or used through a newly chosen setting
  - a publication retry resumes its existing receipt, never rerunning votes/generation or overwriting later explicit changes
- MODEL-37 [o] explicit compatible source-post application atomically replaces validated content, matching storyline and their semantic-origin review information, establishes the machine baseline and moves to review.
  - preserve frozen content-language provenance without changing a newer target; never finalize implicitly
  - retain usable output with unconfirmed origin information under GEN-84/POST-114
  - stale, deleted, finalized or published sources preserve the champion and offer export rather than overwrite

- MODEL-39 [o] every actual candidate/provider call records usage and latency and retains billable failure evidence; supplier cost is authoritative when reported and otherwise explicitly estimated/unavailable under QUOTA.
  - customer test/history reads omit supplier price/cost for every tier; master-only admin cost reads may aggregate by stage and24h/7d/30d windows
  - logs include IDs/stage/accounting and normalized failure, never material, prompts, examples or output
- MODEL-40 [o] frozen model/setting names and versions remain understandable after catalog/source changes; new start/retry/model adoption rechecks entitlement, availability and capability. Source changes do not mutate stored contestant output or silently replace a contender.
- MODEL-41 [o] every test/candidate/match/receipt/history read/action derives the owner from authentication, with foreign/unknown owned references indistinguishable. No public/global model quality ranking is offered; private material never becomes a shared ranking dataset.
- MODEL-42 [o] finished/abandoned tests retain private inputs, outputs, result-related origin evidence and captured product requests for thirty days under the existing content sweep; durable factor/format/bracket/champion/usage metadata remains readable without private payload.
  - an explicitly adopted setting is independently owned and survives test payload expiry
  - source-post deletion purges associated private test payload before detaching history metadata; account deletion cascades tests and their private data
  - deletion/expiry marks a durable payload-purge fence; queued/in-flight/late callbacks cannot restore purged private data or publish a result from it
  - payload expiry is not a new generation or a reason to recreate an adopted setting
- MODEL-44 [o] unified writing tests are a first-class destination at /tests with history at /tests/history and owner detail at /tests/$testId; model selections remain named settings at /ai-models.
  - voice/template/guideline/model entry points all seed the same factor/format/material flow; changing routes or choosing candidates starts no work
  - model/setting selectors show eligibility/classification and readable unavailability reasons; incomplete formats refuse start before admission
  - desktop review shows two complete readable posts beside each other; phone review preserves candidate reading positions and shows the same pair with explicit winner controls
- MODEL-45 [o] automated judges/winner selection, round-by-round regeneration, three/five-way ranking, Elo winner selection, multi-factor changes, video-render tournaments, automatic traffic splitting/model fallback, statistical-superiority claims, BYOK and a second completion provider are outside unified writing tests.
- MODEL-46 [o] `LLMCompletionBudget` applies a code-owned headroom multiplier to the write and revision budgets only when the resolved model's `reasoning_native_effort` is true, bounded by `Ceiling` ← for a native-effort model the effort string is passed through and is a hint, not a cap, so reasoning can spend the whole completion budget and truncate, while on a model whose effort OpenRouter converts to a percentage extra room would only buy a longer think
  - observation keeps its batch-derived budget
  - `LLM_MAX_TOKENS_DEFAULT` and `WriteFloor` are never lowered by it
  - the native-effort signal reaches the budget decision without a catalog type crossing the `internal/llm` boundary
  - QUOTA-32 makes the raised budget held rather than absorbed at settlement
- MODEL-47 [o] when the resolved effort is `none` and the model's recorded `reasoning_efforts` does not contain `none`, `openaicompat.buildRequest` sends `reasoning: {enabled: false}` instead of `reasoning: {effort: "none"}` ← that is the documented mechanism for a disable-able model that lists no `none`; nothing else in the body differs, `reasoning.exclude` is never sent, and an unresolved effort sends no `reasoning` key
- MODEL-48 [o] a reasoning-caused truncation stays a failure — no automatic retry, no in-app model fallback, `MODEL_OUTPUT_TRUNCATED` with its ordinary user-facing message, `technical_detail` carrying the split — and the 모델 관리 surface shows, per model and purpose, how many calls ended in a reasoning-caused truncation beside the reasoning spend (→MODEL-28), sourced from the ledger the same way
- MODEL-49 [o] `video_input` is a recorded capability, not a purpose or a registration gate: it is shown as a 영상 badge on the observe selector and the `photo-analysis` admin tab and checked per run only when the post carries videos (→VIDEO-11) ← a sixth purpose would make every account curate two observe selections for one stage; the badge is derived from the stored flag and, like every capability, a falsy value is unknown rather than "cannot" (→MODEL-21)
- MODEL-50 [o] the source's `:batch` variants (the asynchronous batch endpoint's half-price twins) are dropped at mapping — never a candidate, never curatable — while `:free` variants stay ← every stage call is synchronous, so a batch variant could serve nothing; a `:batch` row already curated is treated as delisted (→MODEL-20)
- MODEL-51 [o] the paste protocol is one plain-text document: the first non-blank line is `# postpilot models v1` and any other version is refused, then `[<purpose>]` section headers each followed by one model id per line, ids verbatim as the source writes them (`:free` and other variant suffixes included); further `#` comment lines, blank lines and surrounding whitespace are ignored, and the five purposes of MODEL-13 and one `[recommendations]` section (→MODEL-72) are the only accepted headers ← the operator hand-edits and pastes a curator's list, so a format a stray space survives beats one a quote mark breaks
- MODEL-52 [o] a section is that purpose's complete membership: applying registers every id it lists and deregisters every current registration it omits, while a purpose the document gives no section is left untouched; each write is the same one the tab's checkbox makes, so a deregistration drops that registration's effort override and level with it (→MODEL-20), and the document carries registrations and their level (→MODEL-59) — no label, effort or any other column is read from it or written by it
- MODEL-53 [o] bulk curation validates the whole document before any write; apply all sections atomically or none.
  - reject unknown versions/purposes, duplicate sections or ids, malformed lines, classification tokens outside MODEL-57, unavailable catalog ids and purpose-incompatible refs
  - a free classification also requires a verified zero-cost usable path (→MODEL-68); an unknown or positive applicable price cannot be labelled free
  - report every rejected line grouped by cause; classification never bypasses stage capabilities
- MODEL-54 [o] applying takes two calls: a preview parses and validates the document, returns per purpose what would be registered, what deregistered, whose level would change and what already holds, plus every rejected line, and writes nothing; the apply carries the same document text and validates it again from scratch — a preview is never a token the apply trusts ← the catalog moves between the two calls
- MODEL-55 [o] the reverse direction renders the current registrations of all five purposes, each id followed by its level when one is set, and the recommendation sets (→MODEL-72) as the same document; a purpose with no registration is emitted as an empty section, so an exported document pasted straight back previews as no change
- MODEL-56 [o] 모델 관리 carries one 일괄 편집 entry shared by its six tabs rather than a control per tab, since one document names any purpose and the recommendation sets; both directions are `ModelCatalogService` RPCs behind the master check (→MODEL-13) and neither touches `model_selections` — a deregistration strands a saved selection exactly as unchecking does (→MODEL-24)
- MODEL-57 [o] each purpose registration has an operator-selected classification: free (무료), or one of the four paid grades value/balanced/premium/top (가성비/밸런스/고급/최고).
  - free is managed as a separate group, not a paid grade or an automatically selected zero-price row; MODEL-68 qualifies its price safety
  - classification is per purpose, may be unset for operator curation, and disappears with deregistration; paid grades are not inferred from price
  - admin offers a separate free-group view/filter and explicit classification controls; user selectors order free before the four paid grades
- MODEL-58 [o] classification gates ordinary selections, preset application, comparisons and execution through QUOTA-19, as well as paid estimator-combo assignment. Unclassified registrations remain visible only for operator curation and cannot bypass entitlement. Locked higher grades stay visible to users with the required plan; being price-zero alone does not classify a registration as free.
- MODEL-59 [o] a models-v1 paste line is an id optionally followed by `free`, `value`, `balanced`, `premium` or `top`. An omitted classification explicitly unsets it; export/preview/apply round-trip the complete membership and classification. Unknown tokens reject the whole document; ordinary access follows MODEL-58.
- MODEL-60 [o] tests retain the originating creation/settings/history location under THEME-58, including filters and safe return through reload or new-tab deep links. A direct test link defaults to test history; a missing source does not imply an unconditional post-list return.
- MODEL-66 [o] test admission, generation and match decisions never mutate source content, observations, voice, template, guideline or active model. Only MODEL-36/90 explicit result actions publish; the test origin controls contextual return, not hidden source mutation.
- MODEL-65 [o] saved eligible model A/B pairs may prefill the two-contestant model test; choosing a complete distinct pair saves it atomically, while count4/8/16 candidates belong to the test draft. Existing seven-slot operator recommendation documents remain compatible and do not silently populate/replace larger test entrants.
- MODEL-68 [o] a curated free model is usable by every tier at zero credit cost and zero credit balance, subject to the same purpose/capability and bounded-execution checks.
  - verify zero cost for the applicable request path; unknown prices or drift to paid pricing make free execution unavailable, never silently bill or select a paid replacement
  - disclose that provider daily/rate/capacity limits can restrict free-model availability; no product per-user daily count or guaranteed daily job total is offered
  - provider refusal offers an explicit retry; operator product exemptions cannot bypass provider limits
  - if no compatible free model exists for a feature, show why it cannot run on the free plan rather than inventing a compatible model

- MODEL-69 [o] recommendation sets are operator-curated rows managed on 모델 관리's 추천 조합 tab and in its 일괄 편집 document (→MODEL-72) by master-only procedures (→AUTH-18); the operator adds, edits, deletes and reorders them, at most 10 sets ← each set is one full section of the phone page
  - a set is a server-assigned immutable id, a label of 1–60 characters after trimming that no other set uses, and seven filled slots: observe active · A · B, analyze active, write active · A · B (→MODEL-23)
  - each slot picker offers the models registered to that stage's purpose with their classification shown; the server rule (→MODEL-70) is the contract

- MODEL-70 [o] saving validates the whole set before any write and refuses it whole, reporting every offending slot by cause:
  - a missing slot, or an observe or write pair naming one model twice
  - a label another set already uses (→MODEL-72)
  - a ref not currently registered to its stage's purpose (→MODEL-14) or registered without a classification (→MODEL-57) ← a set naming either could be applied by no tier
  - no plan or balance check: which tiers can apply a set is settled per account at apply time (→MODEL-26)
  - a later deregistration, delisting or declassification never edits a saved set; the admin list flags the affected slots, derived at read time (→MODEL-22)

- MODEL-71 [o] a set is advice: saving, reordering or deleting it rewrites no account's selections, pairs, experiments or history, and applying copies the set as it is at that moment (→MODEL-25)
  - `/ai-models` lists every set in the operator's order, each with its label, its blocking reasons and its own apply control (→MODEL-26); the id is never shown; with no set the section says there is no recommendation

- MODEL-72 [o] the document's `[recommendations]` section is the complete ordered list of recommendation sets: applying makes the sets exactly the section's, in its order; a document without the section leaves the sets untouched, and an empty section removes every set
  - a set is a `set <label>` line followed by exactly one line per stage, ids verbatim as MODEL-51 with the registry's single provider implied (→MODEL-10):
    | line | ids |
    |---|---|
    | `observe <active> <a> <b>` | three |
    | `analyze <active>` | one |
    | `write <active> <a> <b>` | three |
  - sets are matched to current ones by label: a matched set keeps its identity, a current set the section does not name is deleted, and a new label creates a set ← an account's open page applies by identity, so a set the document only reorders or retunes stays applicable
  - export emits the section after the five purpose sections with every set in order, and an empty section when there is none

- MODEL-73 [o] the document validates its sets with the rest of the document before any write (→MODEL-53) and applies them in the same transaction:
  - reject a label that is empty, over 60 characters or repeated within the section, an 11th set (→MODEL-69), a stage line before any `set` line, a set missing or repeating a stage, and a stage line with the wrong number of ids
  - an added or changed set passes MODEL-70 against the registrations and classifications as the same document leaves them: a set may use a model the document registers and may not use one it deregisters
  - a set identical to a current set in label and slots is kept without re-validation ← a later deregistration never edits a saved set (→MODEL-70), so an exported document pasted back previews as no change
  - preview reports the sets added, removed, changed and reordered beside the purpose diff (→MODEL-54); applying rewrites no account's selections (→MODEL-71)

- MODEL-75 [o] the only valid entrant counts are2,4,8,16, mapped to A/B, four/eight/sixteen-entry knockout formats. Validate the exact count, unique semantic contestants, domain validity, all source ownership/revisions and every model's execution rights before admission; never skip an invalid slot or silently shrink the format.
- MODEL-76 [o] paid comparison/verification history is retained read-only without inventing a binary bracket or champion from a stored multiway outcome.
  - deprecated starts/ranking/check mutations refuse with a localized common-test destination; existing admitted jobs finish/settle safely without automatic replay
  - existing requested publication receipts may finish idempotently, but no new ranking or legacy follow-up is created
  - retained unresolved records do not block ordinary writing; data/privacy/retention remain owner-scoped and no migration silently discards paid outputs
- MODEL-77 [o] speech profiles are curated separately from the five completion/generation purposes and their models-v1 bulk document; each profile identifies one description-based voice-design model and one compatible reusable-voice speech model under the same configured speech connection
  - profile identity and revision are stable; the confirmed voice freezes both explicit refs and the settings needed to preserve its sound
  - the creation picker shows the design model and the associated speech model; the clip picker selects the confirmed voice instead of selecting models again (→DUB-11)
  - saving or changing a profile never rebinds a confirmed voice; incompatible or withdrawn bindings refuse new speech without fallback (→DUB-12)
- MODEL-78 [o] only master registers speech combinations, changes their offered state/classification and adjusts their optional synthesis settings in 모델 관리's 목소리 tab (→MODEL-83/84); classification follows MODEL-57/58, and design and later speech each enforce MODEL-16/68 and QUOTA-19 at admission
  - registration may precede classification or price/quality verification; an unclassified or unready combination remains admin-visible and unavailable for ordinary creation
  - the creation picker exposes grade, required plan, availability and input limits with no implicit initial selection; a free classification requires a verified zero-cost path for every applicable operation
  - completion recommendation sets, comparisons and the five-purpose bulk document neither select nor mutate speech profiles
- MODEL-79 [o] an eligible speech profile verifies Korean description-generated candidates, exact candidate confirmation, supplier-account-scoped voice reuse, supported speech input/output formats and finite request limits
  - advertised speech output alone establishes none of voice-design, custom-voice reuse or timing support
  - missing, changed or unverified capabilities disable the affected operation before a provider call while leaving stored voices and samples readable
- MODEL-80 [o] a speech profile carries versioned applicable price units and provenance resolved from the common supplier tariff (→MODEL-85), rather than manually entered per-combination tariffs or text-token prices; QUOTA-69/70 govern ceilings and settlement
  - supplier credits, input characters, requests, generated seconds and confirmation fees are distinct units; only applicable qualified units are priced and overlapping components are not double-counted
  - absent price or unit evidence makes the paid operation unavailable; an operator cannot classify an unknown price as free
- MODEL-81 [o] speech model readiness distinguishes missing credentials, unavailable profile, unsupported Korean/custom-voice path, missing bounded pricing and incompatible saved voice; customer responses contain product reasons and credits only under QUOTA-65/66
- MODEL-82 [o] production speech availability requires recorded voice-creation qualification under DUB-27 and, for narrated clips, preview and both-export qualification under DUB-25; an offline fixture or provider capability flag cannot establish live pronunciation or continuity
- MODEL-83 [o] 목소리 uses a catalog list under the single configured ElevenLabs speech connection: the operator checks 사용 on a product-supported design/synthesis combination and chooses its classification, with no separate add-combination form
  - the product supplies compatible candidate pairs from the supplier's available models and documented design paths; each row identifies both models and derives its initial display name from their labels
  - presentation labels the two roles as 목소리 만들기 (description-generated audition candidates) and 대본 읽기 (later synthesis with the confirmed voice); friendly names are primary and exact model ids are optional technical detail
  - each pair has one current registration; enabling, disabling or adjusting it never creates duplicate registrations for the same pair
  - registered rows expose classification and an optional collapsed 더빙 합성 설정 control; capabilities, resolved prices, effective limits and readiness are read-only information
  - supplier selection, manual model-id entry, custom pairing and required naming are absent from registration; no catalog entry registers itself or selects a model for a customer
  - 새로고침 rechecks supplier metadata; a failed read preserves saved registrations, settings and history and shows the setup/fetch failure under MODEL-81
  - withdrawing a combination prevents new generation while preserving owned voices, stored samples and generated audio under MODEL-77 and DUB-10/12
- MODEL-84 [o] synthesis defaults and request limits belong to the product; the only optional per-combination sound adjustments are stability, similarity and style strength
  - the settings control states that these values affect later dubbing synthesis, not voice-design audition generation
  - supported controls start with product defaults; an unsupported style control is hidden and its effective setting is neutral
  - speaker boost, natural synthesis speed and supported audio output format are product-managed and respect the selected model's capabilities
  - description, audition and speech limits are derived from product limits and documented supplier/model bounds; the server applies them without per-combination input fields
  - each effective binding is versioned under MODEL-77; new product defaults or registration edits never rewrite a confirmed voice's frozen sound settings
- MODEL-85 [o] the installation has one common account tariff configuration for its single ElevenLabs speech connection, managed by master outside individual combination rows
  - the editor explains that these values price supplier work for every registered combination; empty draft monetary fields start with sourced public reference defaults, including an explicitly provisional confirmation estimate
  - existing monetary strings, including an explicit zero, are preserved; references neither save themselves nor acknowledge complete account evidence, set verification dates or authorize generation
  - monetary editing uses an understandable 1,000-character/reference-unit basis and retains exact decimal precision in the stored unit prices
  - the product obtains verifiable supplier/account pricing evidence and applies documented operation rules and model factors; only unresolved account-specific monetary terms require common setup
  - billing units, non-overlapping charges, applicable multipliers, bounded unit conversions and request ceilings are product-resolved from verified evidence and effective request limits, never required per-model inputs
  - price and bound sources and their verification times are collected with the evidence; a fetch time alone never establishes verified pricing or a free confirmation fee
  - unresolved account terms or operation evidence show 요금 설정 필요 on affected combinations; registration remains possible, but paid work and free classification retain MODEL-80's gates
  - common tariff changes preserve historical snapshots; new quotes resolve current applicable evidence, and a stale price snapshot cannot authorize new work (→QUOTA-69/70)
  - the catalog offers no supplier picker or multiple supplier-account tariff setups
- MODEL-86 [o] every admin speech row shows approximate candidate-creation, confirmation and later script-reading costs before registration, with the applicable quantity and when each cost occurs
  - known, complete account tariffs take precedence where their unit conversion is verifiable; otherwise public references are identified as estimates with source and review date, never as the account's confirmed bill
  - candidate estimates state their standard-rate assumption when only preview-character billing is documented; unknown synthesis models show an unavailable reference rather than a guessed price
  - temporary public discounts carry their expiry and stop applying afterward; persistent common defaults use the undiscounted reference rate
  - these admin references never replace MODEL-80's bounded pricing or MODEL-82's live qualification

- MODEL-87 [o] an authenticated idempotent default-initialization operation prepares each absent active observe/analyze/write slot without model calls, jobs, credit holds or debits.
  - for each stage prefer the first currently eligible active ref in the operator's ordered recommendation sets; comparison refs do not affect this choice
  - if no curated active ref is eligible, choose an eligible registered/classified catalog ref in free/value/balanced/premium/top order and stable catalog order; free access passes the full zero-price/endpoint gate under MODEL-68
  - preserve every existing active ref, including locked, missing and unavailable choices, and every comparison slot; concurrent initialization/manual saves use insert-if-absent semantics and return the actual saved selections
  - a stage with no eligible ref stays unavailable with readable retry/settings help, without an onboarding model-choice requirement or a fabricated fallback
  - repeated login/navigation/catalog changes do not replace an established choice; the owner may change it in settings

- MODEL-88 [o] contestants reference an eligible registered ModelRef, an owned setting revision or an owned prepared authoring candidate revision, never arbitrary client-supplied profile/template/rule payload. Generated contenders remain unsaved, labelled synthetic drafts; snapshot material is server-validated and supports no unrelated private-data lookup.
- MODEL-89 [o] each match admits exactly one human-selected winner from its two stored contestants with expected test revision and an idempotent operation key; duplicate decisions return the existing result and stale/conflicting decisions preserve the current bracket. Completion needs exactly N-1 confirmed decisions and never permits undo that regenerates/recharges work.
- MODEL-90 [o] explicit winner saving publishes the tested frozen setting exactly once with an owner/test/winner/action receipt.
  - generated writing styles keep their tested synthetic analysis/fictional example and never become personal evidence; model adoption requires current stage eligibility
  - template saving retains valid structure/numeric domain rules; guideline saving requires explicit application scope and normal caps/uniqueness
  - unchanged existing owned winners may be used directly; changed/deleted synthetic styles, templates or guidelines offer an explicitly named new copy rather than silent overwrite/resurrection
  - personal-voice winners use only their still-active owned accepted profile; changed/deleted personal sources remain historical results until explicit restore/reanalysis/retest, never transferring personal materials/analyses to a new voice
  - default/assignment choices are explicit; lost-response retries return the same confirmed setting and never repeat defaulting or undo a later edit
- MODEL-91 [o] explicit cancellation/abandonment names the test and its confirmed usage consequence; stop remaining planned work where possible, settle issued usage once and fence late results. Closing/navigating only preserves recoverable work and never cancels or spends.
- MODEL-92 [o] seed-free preparation uses EDIT/VOICE bounded2/4/8/16 private candidates and is a separate estimated explicit action from generating actual test posts. The winner can become an owned reusable seed; nonwinning candidates are not automatically saved.

- MODEL-93 [o] every model-request stage and mode has one discoverable prompt inventory covering observation, planning, writing/revision, voice/style work, setting authoring, memory extraction and existing video work.
  - identify owning source files/composing functions, stable fragments, roles, activation conditions, input boundaries, precedence, output schema/parser/consumer and prompt/schema version
  - each instruction has one authoritative owner; shared composition preserves its stage meaning and language rather than copying independently maintained rules
  - request System/User roles and code/account authorship are separate inventory fields; a wire role alone does not identify the content owner or semantic source
  - inspect the final application request from the same composition used for execution; useful long instructions remain, while duplicate or irrelevant context is removed under GEN-85
  - developer/operator inspection of code/configuration and synthetic material grants no new access to another account's private requests

- MODEL-94 [o] optional owner-scoped request inspection shows a safe product-level projection of ordered prompts/material roles, selected rules, output contract and effective model/budget/effort conditions.
  - distinguish current configuration, prepared request preview and captured issued request; capture uses the effective application request at execution and records its prompt/schema versions
  - prepared previews make no execution claim; missing historical capture is unavailable, never reconstructed as the past request
  - label character counts, UTF-8 bytes, reference-token estimates and actual provider usage separately; unavailable runtime measures remain unknown
  - inspection uses no provider call, hold, fallback or canonical mutation, and preserves existing owner/source retention policies

- MODEL-95 [o] private request/origin inspection inherits authenticated ownership, cache partitioning, deletion and payload-purge fences.
  - never expose credentials, SDK/network payloads, base URLs, signed media links or supplier costs through customer inspection (→MODEL-27 →QUOTA-66)
  - prompts, material and output do not enter ordinary logs (→MODEL-39); capture is private result-related payload, not permanent accounting metadata
  - blind comparisons retain MODEL-32 identity boundaries across prompt text, setting metadata and source details; purge/expiry never triggers regeneration or restoration of private payload

## flow
- call: caller(stage, ref, request) → Registry.Complete(admitted entitlement + stage membership + capability/price checks → effort resolution(override → stage → none) → budget → adapter stream → normalized usage / error)
- curate: 모델 관리 tab → ListCatalog(live read ∪ DB rows | DB rows + fetch_error) → SetModelPurpose | SetModelReasoning | SetModelLevel → the next Complete sees it
- curate speech: 목소리 → supported combination list + saved registrations → check 사용 → choose classification | adjust optional synthesis settings → versioned effective binding and resolved common tariff → readiness/entitlement gates → explicit customer selection
- speech pricing: common ElevenLabs account setup → verified supplier/account terms and operation rules → resolve combination tariffs/limits → freeze evidence for a bounded quote → explicit paid-work approval under QUOTA-69
- recommend: 모델 관리 추천 조합 tab → save(whole-set validation → one write) | reorder | delete → /ai-models lists sets in order → owner apply(MODEL-26 gate over seven refs → one transaction)
- bulk curate: 일괄 편집 → export(current five sections + recommendations) | paste → preview(per-purpose diff + set diff | rejected lines, no write) → apply(re-validate against the document's own registrations → one transaction) → the next Complete sees it
- test: choose factor/format → owned or prepared entrants + common material → freeze/validate/estimate → explicit generation → all outputs ready → N-1 human pair decisions → champion/reveal → explicit save/adopt/export → thirty-day private-payload sweep

## constraints
- speech administration uses one ElevenLabs connection; catalog registration performs no voice generation, supplier charge or readiness promotion, and preserves MODEL-77/79/80/82's compatibility, pricing, history and qualification contracts
- config: `PROVIDERS_CONFIG` · the yaml's `api_key_env` (`OPENROUTER_API_KEY`) · `LLM_MAX_TOKENS_DEFAULT` 8192 (env, refused at boot when invalid) · `LLM_STAGE_TIMEOUT` 5m · `OPENROUTER_CATALOG_TTL` 5m · `OPENROUTER_CATALOG_FETCH_TIMEOUT` 15s · candidate concurrency ≤ 5 per experiment · `EXPERIMENT_CONTENT_RETENTION` 720h and `EXPERIMENT_SWEEP_INTERVAL` 24h (env) · admin cost windows24h/7d/30d · recommendation sets ≤ 10 · set label ≤ 60 characters; test format/count and admin cost windows are code-owned constants; FE `entities/model-catalog/config`: `MODEL_CATALOG_STALE_MS` 300000 · `MODEL_PURPOSES`; FE `features/manage-model-catalog/config`: `FEATURED_MODEL_PROVIDERS` · `CATALOG_ROW_ESTIMATE_PX` 132 · `CATALOG_ROW_OVERSCAN` 6; the per-stage budgets and the MODEL-46 headroom multiplier are code-owned in `backend/internal/platform/config`, the stage reasoning policy in `backend/internal/generation` (`DefaultReasoningPolicy`); the eight-value effort list in `entities/model-catalog/model/types.ts` is the fallback for a model with no published list
- schema: `catalog_models` (id, provider slug, label, `vision` `video_input` `structured_output` `image_output` `video_output`, context, pricing snapshot + `pricing_checked_at`, `listed`, `last_seen_at`, `source_created_at`, `reasons` `reasoning_efforts` `reasoning_default_effort` `reasoning_mandatory` `reasoning_native_effort` `reasoning_max_tokens`) · `catalog_model_purposes(model_id, purpose, reasoning_effort, level)` · `model_selections` · owner-scoped writing tests, candidates, matches and publications beside retained paid experiment metadata; source-post associations detach only after private payload purge
- placement BE: `backend/internal/llm` (port, registry, errors, cost resolver, `openaicompat`, boundary test) · `backend/internal/modelcatalog` (+ `openrouter` client, mapping, availability) · `backend/internal/provider` (selections, pairs, recommendation sets, `ListModels` with affordability) · `backend/internal/experiment` (aggregate, runner, single-factor snapshots, binary test matches, publication receipts, retained paid history and retention sweeper)
- placement FE: `entities/model-catalog` · `entities/model-experiment` · `features/select-model` `configure-model-pair` `apply-model-recommendation` `manage-model-catalog` `start-model-experiment` `review-model-experiment` · `widgets/candidate-comparison` · `pages/ai-models` `pages/model-experiment` `pages/admin`
- dependencies: `github.com/goccy/go-yaml` (BE, `gopkg.in/yaml.v3` is archived) · `@tanstack/react-virtual` (FE)
- known gap: `MODEL_PURPOSE_NOT_REGISTERED` and `MODEL_PURPOSE_INELIGIBLE` have no entry in the frontend's normalized reason catalog and render as the generic failure (LANG owns that catalog)

## chg
- r35 261007 MODEL-5✎ MODEL-32✎ MODEL-37✎ MODEL-42✎ MODEL-93+ MODEL-94+ MODEL-95+ capability-only schema boundary→syntax and semantic accuracy distinguished; unrestricted blind details→identity-safe inspection; content-only application→matching origin review; input/output expiry→origin/request expiry
