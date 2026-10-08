# REVIEW structure-261008
> st:ready@261008 | scope:all | at:7dd79345 | base:ARCH@20

## summary
- The code has a useful domain/FSD/actor foundation; readability is reduced mainly by evolutionary leftovers, hidden mandatory collaborators and repeated cross-cutting rules, not by an absence of architecture.
- 26 structural proposals: 10 clear existing-policy improvements adopted for sequential implementation; 16 broader scope/ownership/compatibility choices await a decision. Do not merge superficially similar code whose provenance, persistence or execution authority differs.

## findings
- F1 [o] P1 `frontend/src/shared/ui/sheet/Sheet.tsx:95`: bug: each overlay independently owns restoration of one shared body overflow value; nested or non-LIFO teardown can leave scrolling locked after every modal closes ← the shared resource needs one lifetime authority rather than per-instance snapshots; preserve current focus/animation/dismissal semantics. →T654
- F2 [o] P2 `frontend/src/entities/model-catalog/api/useCatalogDocument.ts`, `useAdminCatalog.ts`: curation hooks repeat incomplete cache dependency lists instead of one catalog-owned invalidation behavior ← refreshing and bulk/single changes must invalidate the same authoritative projections, including recommendations where they changed.
- F3 [o] P2 `backend/internal/billing/store/store.go:33`, `store/refunds.go`: the hand-written SQL handle conflates reads and writes although generated queries already distinguish them ← ordinary refund reads contend with the serialized writer and future methods must guess connection ownership; preserve transaction-scoped read-your-writes behavior.
- F4 [o] P2 `frontend/src/entities/subscription/api/useRefunds.ts`, `index.ts`: refund features consume renamed generated messages while the ordinary subscription API exposes mapped domain types ← move the wire seam into explicit refund domain models/mappers, preserving current field/status/money semantics.
- F5 [o] P2 `frontend/vite.config.ts:60`, `frontend/src/app/routes/models.ts:4`: bug: route schema import rewriting treats inline type specifiers as runtime symbols and silently keeps the UI barrel eager ← make the transformation understand its actual supported named-import shapes and pin them at the build boundary; do not introduce another routing framework.

- F6 [o] P2 `frontend/src/features/edit-template/ui/TemplateDirectEditor.tsx:56` TemplateDirectEditor: bug: successful JSON parsing is treated as object validation, so valid persisted `builderState: "null"` crashes direct editing.

  **Evidence:** `working = JSON.parse(...) as Record<string, unknown>` escapes the catch with `null`; line67 then evaluates `working.numberMemory`. `entities/ai-authoring/api/mappers.ts:154` returns private builder metadata verbatim; `backend/internal/authoring/working.go:341` validDirectMetadata checks UTF-8/size, without requiring an object. The canonical title/body may be entirely valid.

  **Actual rendered reproduction:** `/tmp/structural-template-261008.test.ts` invokes the shipped editor with a valid template and null metadata. `pnpm --filter ./frontend exec vitest run --config /tmp/structural-template-261008.config.mjs` fails with `TypeError: Cannot read properties of null (reading 'numberMemory')`.

  **Improvement:** parse to unknown; accept only non-null, non-array object metadata, otherwise `{}`; retain current authoritative title/body and child-specific `readCompositionWorkingState` validation. Put the metadata reader in the owning template model/lib rather than casting at a UI boundary. No product-policy decision is needed.

  **Cost/validation:** small. Add editor/recovery cases for null/arrays/primitives/malformed JSON, retained valid metadata and unchanged raw source. Run existing `pages/template/ui/TemplatePage.test.tsx`, `TemplateNumbers.test.tsx`, `TemplatePreviewLayout.test.tsx`, then relevant lint/format/type-build checks. EDIT-14/21 and ARCH-24 apply.


- F7 [o] P2 `frontend/src/features/edit-template/model/useTemplateDraft.ts:78`, `model/useTemplateSave.ts:20`, `features/edit-guideline/ui/GuidelineEditForm.tsx:29`, `features/configure-model-pair/ui/CandidatePairSelect.tsx:37`: retired production editors remain exported and maintained beside their current replacements.

  **Evidence:** repository-wide symbol searches find production references only in each slice's index, plus `useTemplateSave -> useTemplateDraft`; the old template hooks and CandidatePairSelect are exercised only by their own tests. GuidelineEditForm has no production caller. Current consumers use `pages/template/ui/TemplatePage.tsx:279` TemplateDirectEditor, `widgets/guideline-directory/ui/GuidelineDirectory.tsx:270,417` GuidelineDirectEditor, and model forms use ActiveModelForm/OptionalTestPair. The code retained a second save/leave/number policy that current authoring does not use.

  **Trigger/cost:** every template number, navigation, scope or selection contract update keeps obsolete hooks/forms/tests compiling and offers future developers an apparently supported alternate entry. Unifying them would create the wrong abstraction: they represent replaced execution paths.

  **Improvement:** remove the demonstrated zero-consumer symbols/files and their obsolete public exports/own tests, retaining active slice components and resources still consumed elsewhere. Do not delete the entire configure-model-pair slice: ModelPairForm, OptionalTestPair, ActiveModelForm and LabExtraCandidates remain used. Do not delete MemoryEditForm: MemoriesPage still uses it.

  **Cost/validation:** small/medium, safe under current application-only workspace. Confirm all symbol consumers after deletion; run active template page/numbers/grammar tests, guideline-directory tests and AIModelsPage/model-pair tests plus build/lint/FSD checks. No behavior or SSOT change.


- F8 [o] P3 `backend/internal/generation/service.go:172-186`, `generation/revise.go:61-102`: `StartRevision` marshals the frozen payload, immediately unmarshals it to append origin protocol/cap, then marshals it again. The complete admitted request has two builders and a variadic profile argument that represents optionality only indirectly. Generate and storyline encode the complete options in one operation; future revision metadata is easy to put in only one path.

  Fix: construct the complete revision payload/typed options once, then encode once. Preserve the small legacy encoding helper for historical test inputs if still needed, but route the production path through a complete options argument. Do not centralize all generate/storyline/revision payload formats merely because they share some fields.

  Validation: existing revision payload/language/profile/origin admission-cap and request-inspection round trips, generation service and cmd/api hold/work agreement tests. Safe mechanical change under current SSOT; a good small implementation candidate.

- F9 [o] P3 `backend/internal/authoring/service.go:42`, `template/service.go:340`, `memory/service.go:196`, `voice/service.go:590`, `job/job.go:273`, `post/service.go:1640`, `guideline/service.go:504`, `voucher/service.go:217`: eight contexts implement the same cryptographically random 16-byte lowercase-hex entity ID algorithm. This logic has no business meaning and ARCH-6 explicitly names `platform/ids` as its home, but that package does not currently exist. This is true identical-contract reuse, unlike the superficially similar token generation functions.

  Fix: a tiny `internal/platform/ids` helper, leaving context-local test seams/wrappers where necessary and preserving existing 32-hex output. Keep auth/session/reset credentials, voucher bearer tokens, file temporary names and externally versioned fingerprints separate: byte size, encoding, entropy/error lifecycle and exposure rules differ.

  Validation: format/length/uniqueness smoke plus selected creation/upload/job/voucher entity tests, whole-backend build/vet for imports. Safe mechanical candidate, low urgency; do not add a UUID package or redesign persisted IDs for this cleanup.

- F10 [o] P3 `backend/internal/provider/ports.go:59-63`; `backend/internal/plan/plan.go:31-33,52-55`; `backend/internal/llm/registry.go:111-116`: policy comments state that affordability is the sole model-access rule, tier ranking does not authorize, and levels gate nothing. Executable code directly contradicts each claim (`plan/offers.go:28-48`, `provider/service.go:585-598`, `usage/service.go:328-345`, `llm/registry.go:449-460`), and current MODEL-16/58 require grade entitlement. Reading these published seams currently teaches the previous policy and can lead a maintainer to remove a necessary gate.

  Fix: replace the obsolete policy descriptions with present behavior and precise ownership: plan ranks commercial offers; plan owns allowed model grades; source supplies curated classifications; provider resolves selector entitlement and llm enforces admitted rights/live safety. Remove historical policy narration from current API comments. Safe direct correction with no product-policy decision.

  Validation: compare updated comments to executable definitions/SSOT; gofmt/build only as appropriate. No behavioral test expansion is needed for comment-only changes.

- F11 [?] P2 `backend/internal/provider/service.go:25,32-40,63-79,585-598`, `provider/ports.go:43-46,64-70`; `backend/internal/usage/service.go:88-93,328-329,351-363`; `backend/internal/llm/registry.go:190-191,449-460`: mandatory model entitlement is an opt-in construction mode, and required tier/free-path capabilities are absent from declared ports. Forgetting `.WithModelGrades()` constructs a valid provider service that acts as master, an ordinary ledger that skips entitlement checks and a registry that does not require trusted durable admission. The provider and ledger discover `Tier`/`QualifyFree` through anonymous runtime assertions; a missing free qualifier skips a mandatory qualification instead of preventing construction. Current production correctly enables all three flags (`cmd/api/contexts.go:177,335`, `cmd/api/platform.go:88`), so this is a structural failure mode rather than a demonstrated production entitlement bypass.

  Fix: make ordinary execution/selectors construct with mandatory grade enforcement and explicit `TierReader`/`FreePathQualifier` ports. Use a named, narrow ledger for lot/coverage-only transactional consumers rather than one all-purpose service that admits a weak mode to support those unrelated consumers. Preserve admitted-period rights and live price compatibility (MODEL-16/58/68, QUOTA-61). Do not remove the three flags in one blind edit: lot-only billing/seed composition deliberately uses no models.

  Validation: provider access/default/preset/inspection tests, usage admission/free/transactional-lot tests, llm registry/free/admitted-call tests and cmd/api wiring/qualification tests. New constructor regressions should make missing admission/tier/qualification wiring fail early. Safe policy direction under current SSOT; scope/constructor split should remain a review decision.

- F12 [?] P2 `backend/internal/post/ports.go:54-68,269-292`, `post/service.go:106,374-389,896-899,957-960,1015-1018,1648-1655`, `post/list.go:99-116`: the post API service accepts only the older broad job read/base store, then discovers mandatory editor, ordinary-job and latest-failure behavior through runtime assertions. A constructor-valid store can expose save/finalize RPCs that fail only at runtime; a constructor-valid job reader can silently lose failure history or use a fallback that claims the obsolete schema permits only one active post job. Current writing tests and ordinary jobs coexist, which is why the production `PostJobs` adapter must implement the newer reads. Production currently satisfies these hidden ports; no current failed RPC is claimed.

  Fix: require `ContentStore` and the actual post read behaviors in the API service constructor. Keep sweeper/upload use cases on the already narrow ports they need; they do not require weakening the complete post-service construction contract. Keep `SetTemplateDirectory` optional because its absent mode is independently legal and tested. Remove the obsolete single-job fallback after consumers compile against explicit requirements.

  Validation: post editor/published-lock/generation-options/job-failure tests, post/store and post/rpc suites, experiment/app post-job tests and cmd/api wiring/writing-test lifecycle tests. A constructor/compile-time correction is safe under existing POST/ARCH; it changes no product policy. Related to the architecture pattern from old arch F18, but these new optional capabilities and multi-job fallback were introduced after that cleanup.

- F13 [?] P2 `backend/cmd/api/adapters_authoring_targets.go:85-121,138-175,235-286`: common authoring transport adapters contain application rules, including which kind may carry which fields, a literal description ceiling, the requirement that new guidelines carry scope, preserving template-number presence, and deciding that voice authoring always forks. These rules are tested as part of command-package integration rather than through an importable authoring application. `authoring.Targets` consumers appear simple but actual behavior requires reading the entire process composition root. This is a newer instance of ARCH-6/40 drift, separate from old generation-root F13 and prior all-261008 findings.

  Fix: move semantic kind/metadata/publication coordination to `internal/authoring/app`, over narrowly declared target ports. Leave cmd/api with construction and type adaptation. Use the existing `experiment/app/Publications` placement as precedent. Keep target-domain validators/atomic receipts owned by template, guideline, voice and clip; do not create a generic entity repository or push all kind rules into a giant shared artifact validator.

  Validation: existing cmd/api authoring-target/publication/recovery/metadata and authoring/store acceptance; owning template/guideline/voice/clip publication checks. Safe direction under ARCH, but choose the app-port shape in review before implementation. No new user behavior or unified domain field rules are recommended.

- F14 [?] P2 `backend/internal/post/origin_result.go:47-71,93-109`; `post/origin_alignment.go:285-294`; `generation/origin_revision_mapping.go:15-25`: stable semantic identity is coupled to Go domain struct layout through direct JSON marshaling. `ContentOriginIdentity` and `PlanOriginIdentity` feed persisted origin hashes, request-capture/publication fences and tests, yet adding/reordering an exported field on `post.Block`/`StorylineParagraph` silently changes existing identities even when the stored canonical payload is unchanged. Current tests compare identities computed by the same implementation (`post/origin_result_test.go:8-23`) rather than pinning the existing hash bytes. This is a new post-origin identity contract, distinct from historical generation-snapshot F14/T361.

  Fix: define explicit private canonical identity encodings whose field names/order and empty normalization reproduce today's bytes, then map domain values to them. Keep semantic identity separate from storage JSON and source-origin sidecars. Add fixed known-answer hashes and storage round-trip fixtures; future format changes become intentional versioned changes, rather than incidental struct edits. Do not alter current hashes or migrate historical rows for this refactor.

  Validation: post origin identity/manual alignment, generation origin revision/normalization and request-capture tests, post/store origin/publication/capture suites and experiment/app champion publication. Safe if byte-preserving golden tests establish compatibility; an actual hash-format change requires an explicit migration/version decision.

- F15 [?] P2 `backend/internal/voice/types.go:208-224`, `voice/analysis.go:127-154`, `voice/store/personalization.go:24-30,50-53,87`, `voice/store/{authoring,candidates,test_styles}.go`: `Tic`, `AIExample` and `AIPart` are both domain types and persisted/provider JSON shapes. The provider answer uses `[]Tic`, and analysis snapshots marshal `voice.AIPart` directly; domain tags are a direct ARCH-7 exception, so a harmless domain rename or a new field can alter every newly stored analysis and provider parser. The surrounding snapshot struct is a mapper but its nested AI/Counted fields bypass that boundary.

  Fix: give provider answers and store snapshots explicit private DTOs and map into clean domain types. Freeze existing version-1 snapshot field spellings and preserve missing/empty behavior. Keep provider schema and durable snapshot evolution separate; their overlap is accidental, because external model evidence and trusted stored analysis have different admission and compatibility rules.

  Validation: old version-1 analysis fixture reads, ordinary/synthetic/authoring/test-style round trips, voice parser/source-verbatim filtering, projection and source-revision tests. Safe under existing ARCH/VOICE if bytes and old records remain readable; no provider-prompt/schema policy change is needed.

- F16 [?] P2 `backend/internal/post/list.go:118-134`, `backend/internal/experiment/app/post_jobs.go:25-34`, `backend/cmd/api/job_subjects.go:138-140`, `backend/internal/job/store/store.go:243-284`, `backend/internal/experiment/store/store.go:138-149`: a paged post directory decorates each row with four sequential active-job SQL reads and one retained-experiment SQL read when no row has active/pending work. The exact production `ordinaryPostWriteKinds()` list has four values and `PostJobs.ActiveOrdinaryForPost` probes them individually. Thus a 20-row idle page needs 100 extra SELECTs, a 100-row page 500, before the already-batched latest-history and owner-directory reads. A pending retained comparison additionally loads candidates only to return its ID. This differs from earlier perf-cost F9 (the experiment directory) and all-261008 F42 (writing-test metadata directory).

  Fix: publish a generic job-owned bulk active-summary read by owner/subject IDs and an experiment-owned bulk pending-ID read; post consumes both through its own behavior ports. Preserve ordinary-kind precedence or replace it only after proving the ordinary job guard makes multiple matches impossible. Do not let post/store join other contexts' tables to save queries.

  Validation: add call/query-count invariants for empty/active/mixed/100-row lists; verify owner isolation, ordinary/test coexistence, latest-failure precedence, retained pending IDs and unchanged paging/search. Safe behavior-preserving change under POST-90/91 and ARCH-7; no current benchmark or production latency incident is claimed.

- F17 [?] P2 `backend/internal/clip/app/generation.go:68-69,285-295`, `backend/internal/clip/app/rerender.go:294-329,422-484`, `backend/cmd/api/clip.go:63-91`: dormant in-API preparation/final rendering remains a nil-selected execution mode after worker-only production dispatch became mandatory. `GenerationDeps.RemoteMedia` still says its nil rollout path is "removed by T387", while nil selects `media.WithWorkspace`, API-side probing/rendering/upload and a separate export-reservation/enqueue path. Production composition always constructs MediaDispatch and RenderAdmission. `spec/tasks/done/T387.cpu-vps-compatibility-checks.md` explicitly says to remove embedded preparation/final-render dispatch while preserving authored checks/preview tooling. This is distinct from perf-cost F25/T573's already removed non-Portable renderer.

  **Change cost:** worker lease, source lifetime, admission or result-publication changes must keep a second test-only orchestration path correct; omitting the collaborator selects a different architecture instead of failing construction.

  **Recommendation:** make durable dispatch/admission explicit required production dependencies and remove obsolete embedded preparation/final-render bodies. Retain API-side authored-plan checking, caption/style previews and storage authorization; these are current responsibilities, not dead code.

  **Boundary:** the desired production behavior is already decided by ARCH-40/45 and T387. Root can implement without a new product decision, but first migrate useful regression fixtures onto the dispatch seam and preserve readable/recoverable old queued payloads. Do not silently discard persisted version-1 payloads merely to remove execution code.

  **Regression:** same-host and remote synthetic admission/continuation, offline worker waiting, cancellation/completion and stale revision, export reservation release/consume, boot recovery of old records, preview assets and generated-plan-only behavior. Relevant package/consumer tests at completion; real-image/release gates only before push under ARCH-31.


- F18 [?] P2 `frontend/src/entities/clip-preview/ui/ClipDraftPreview.tsx:434-863`, `frontend/src/features/preview-clip-draft/ui/ClipDraftPreviewPanel.tsx:73-80`, `frontend/src/features/render-clip-browser/api/useBrowserRender.ts:147-191`, `frontend/src/features/render-clip-browser/api/run-render.ts:106-175,183-339`, `frontend/src/entities/clip-preview/lib/video.worker.ts:18-29,182-255`: complete legacy browser playback/export execution remains publicly selectable although current production callers always supply the local compositor. ClipDraftPreviewPanel always sets `localMode`; useBrowserRender always freezes and supplies `snapshot` before runBrowserRender. The no-snapshot path still owns server sampling/polling/raster preparation, HTML-video/full-Blob original loading, caption-sheet protocols and a separate cancellation/mux/store lifecycle.

  **Evidence:** `rg` of production `<ClipDraftPreview`, `runBrowserRender(` and `useClipPreviewRequest` callers found the local panel and the snapshot-bearing hook; remaining direct legacy preview calls are tests. Browser-media diagnostics also deliberately exercise some lower-level legacy helpers, so do not claim every old helper is unreachable or simply delete all matching names.

  **Change cost:** changes to current preview props, speech, cancellation, output lifetime and worker messages must maintain unrelated legacy capability branches and tests. The current local path still constructs requestPreview/job collaborators used exclusively by the old render mode, obscuring what its admission actually needs.

  **Recommendation:** narrow the shipped preview/export interfaces to their current local engine and remove legacy orchestration/UI after preserving useful diagnostics as clearly isolated comparison fixtures. Do not force local Worker playback and native FFmpeg delivery into one renderer. Do not retire the backend authored caption/style preview API just because the editor/export no longer consumes server rasterization.

  **Decision needed:** confirm whether the no-snapshot browser engine remains an intentionally supported diagnostic/comparison target, or should be retired completely. If kept, isolate it behind an explicit diagnostic entrypoint excluded from shipped composition; it should not remain an optional public product mode. This decision is independent of T603/T604's unmet activation evidence.

  **Regression:** actual production local preview mounted controls/seek/revision updates, local export Worker and packet backpressure, narration/source-only/silent paths, abort/stale bitmap cleanup, upload retry without encoding; keep existing template/style samples and useful native parity fixtures.


- F19 [?] P2 `backend/internal/clip/app/service.go:16-30,138-157,273-281,477-490`, `backend/internal/clip/app/generation.go:26,86-101`, `backend/internal/clip/app/quotes.go:90-115`: constructing GenerationService mutates an already constructed project Service, and the pair reaches through each other's private collaborator fields. Project reads sign output via `s.generation.objects/cfg`, project changes inspect `s.generation.jobs`, dubbing resolves `s.generation.voices`; generation reaches project storage/limits directly and calls its private duration predicate. Consequently the usable project service's behavior depends on a later constructor side effect despite its own constructor succeeding.

  **Change cost:** independently testing/wrapping either service requires constructing the other; generation constructor/wiring changes can alter ordinary project read/edit/dubbing behavior. The nil project-generation branch silently skips the active-job fence or result URL projection; current production wiring is correct, and no new production data-loss incident is asserted.

  **Recommendation:** inject the actual narrow behavior ports into Service (active job lookup, result access/signing and spoken-voice resolution) and give GenerationService its own project reader/request recorder plus owned limits. Optional guideline candidate notification may remain explicit optional wiring. Reuse small pure duration validation as an owning clip rule if both use cases need it. No service must mutate another to finish its construction.

  **Boundary:** engineering-only refactor under ARCH-6/40; safe for root to implement in a bounded task after preserving existing optional legal modes and fixture intent. No domain redesign or generic repository layer is required.

  **Regression:** isolated project service tests pin busy fence, owner-scoped signed result reads, dubbing binding resolution, read/deletion semantics and omitted optional candidates; generation quote/request record and both API composition tests continue to pass.


- F20 [?] P2 `backend/internal/clip/generation_ports.go:85-100`, `backend/internal/clip/app/quotes.go:313-315`, `backend/internal/clip/app/generation.go:133-143,331-334,457`, `backend/internal/clip/app/attempt_diagnostics.go:12-16,33-40`, and other `s.store.(...)` sites: GenerationStore is an incomplete constructor contract; required workflows discover their storage behaviors later through type assertions. The declared port omits QuoteStore, RevisionQuoteStore, StorylineStore, BrowserRenderStore, BrowserUploadStore and AttemptCheckpointStore. A decorator/fake implementing the advertised GenerationStore type compiles, then quotes fail as pricing unavailable, storyline operations fail, or checkpoint writing silently returns success without persisting anything.

  **Change cost:** interface conformance does not prove a usable generation service; every wrapper/new adapter must discover hidden dynamic capabilities across many use cases. Tests with intentionally narrow fakes can pass paths that production expects to persist diagnostics/recovery information.

  **Recommendation:** publish constructor dependencies for the behavior groups actually needed by each use case, or an explicit capability bundle with required behaviors validated at construction. Keep genuinely optional supported modes explicit and tested rather than inferred from a downcast; do not create one new mirror of all SQL methods or require browser rendering when an explicitly unsupported mode is legal.

  **Boundary:** engineering contract correction under ARCH-6/40. Required vs optional capabilities must be enumerated against current SSOT and API activation modes before editing; no silent conversion of optional checkpoint policy into fatal behavior is proposed.

  **Regression:** compile-time adapter conformance, constructor refusal for absent required quote/continuation ports, explicit supported-optional mode tests, checkpoint persistence/read, API admission/recovery and browser publication consumers.


- F21 [?] P3 `frontend/src/features/preview-clip-draft/model/useClipDraftPreview.ts:39-58`, `frontend/src/entities/clip-preview/model/composite-video.ts:43-55,66,177-193`, `frontend/src/entities/clip-preview/model/browser-composition.ts:712-728`, `frontend/src/entities/clip-preview/lib/preview.worker.ts:156-176,193-196`: immutable composition preparation is rebuilt inside frame work. Each preview hook render serializes plan/source/layout-observation content for its key, including playhead-only updates. Each composite frame serializes both complete plans, rebuilds the timeline, and its evaluator rebuilds the timeline again and canonicalizes both ink identities. Flow mode additionally evaluates the same flow separately for footage and components. A maximum 60-second 30-fps export repeats those two plan serializations 1,800 times, after compositeBrowserVideo already checked their equality once.

  **Change cost:** frame code mixes immutable validation/preparation with actual time evaluation, making it harder to keep frame work cheap and to see which checks truly need an epoch guard. This is a proven allocation/control-flow pattern, not a measured export-speed or device-memory claim.

  **Recommendation:** create a run/preview-owned prepared composition context from the validated frozen snapshot, containing timeline/source lookup/static asset ordering/identity check results; frame evaluation accepts only frame/time and current-epoch guard. Memoize the preview runtime content key on meaningful dependencies, explicitly excluding playhead time and preserving unsaved-plan/local-source changes. Preserve single-frame boundary validation for standalone callers and genuine per-frame cancellation/supersession fences.

  **Decision needed:** implementation investment and measured benefit. Do not introduce a global cache or reuse across owner/snapshot boundaries. Regressions should preserve the existing rejection of a mutable plan differing from the snapshot; choosing the immutable snapshot as sole run authority must be explicit.

  **Regression:** deterministic frame/flow outputs across cuts, transitions/rates/styles/ratios; old snapshot refusal; playback-time changes do not rebuild/static-serialize resources while caption/source/owner/revision changes do; count preparation invocations before reporting a performance improvement. Real-device qualification remains separate.


- F22 [?] P2 `frontend/src/shared/api/index.ts:245` generated Block/PostContent/Observation exports; `entities/post/model/types.ts:9,89`; `entities/post/model/content.ts:1,19`; consumers `features/edit-post-content/ui/BlockEditor.tsx:22`, `widgets/export-panel/ui/ExportPanel.tsx:5`, `pages/editor/ui/EditorFinishPanel.tsx:4`: the main post-content model still carries protobuf messages throughout domain/edit/export/presentation layers.

  **Evidence:** `entities/post/model/content.ts` imports protobuf clone/create and BlockSchema/PostContentSchema, its patches omit `$typeName`/`$unknown`, and screen/feature signatures still directly use generated PostContent/Block/Observation plus BlockType/GalleryLayout. `shared/lib/blocks/walk.ts:1` also embeds post-specific block semantics in the nominally domain-agnostic shared layer. The application consequently cannot evolve block representation at its documented adapter seam.

  **Guard gap:** `frontend/src/test/arch-proto-symbols.test.ts:17` and `frontend/eslint.config.js` test only pages/widgets/features and names ending Schema/Service or starting Proto. On pinned Node24.18.0, `pnpm --filter ./frontend exec vitest run src/test/arch-proto-symbols.test.ts` passes all4 assertions despite the model schema imports and ordinary generated aliases above.

  **Trigger/cost:** changing proto content fields, block kinds or unknown-field behavior spreads through editors, export formats, cache/queue copies and tests. This is distinct from all-261008 F32's refund DTO leak but has the same architectural principle.

  **Improvement:** sequence an entity-owned canonical content model and explicit API mappers; export content construction/traversal from entities/post (cross-entity consumers through @x), with post-specific blockPhotos/headingTag leaving shared. Strengthen the import guard to cover entity model/lib/UI and non-Proto aliases rather than relying on symbol spelling.

  **Decision needed:** choose migration order and intentional handling of unknown proto fields/enums before changing the canonical representation. This is not a small rename or reason to remove forward-compatibility behavior.

  **Cost/validation:** large, domain-wide task(s). Pin exact existing content/export results, unknown-field compatibility, gallery/photo/video ordering, edited content revision/queue behavior and writing-test publication. Run the full affected FE suite because imports alone cannot bound canonical data effects; ARCH-3/13/14/17/24 apply.


- F23 [?] P2 `frontend/src/widgets/writing-test/ui/WritingTestStudio.tsx:118` OwnedStudio, especially lines338,360,380,433,514,542: writing-test presentation also owns the coordination protocol among the operation actor, candidate-preparation actor, source defaults and publication projections.

  **Evidence:** this1591-line module reads voice/template/guideline/model/source collections, deduplicates effect-driven source/preparation/publication transitions via appliedSource/appliedPreparation/refreshedPublication refs, assigns prepared entrants, advances presentation, refreshes dependent entities and implements model/fact/publication eligibility. The boundary is substantive: line380 both mutates entrants and sends NEXT, while line514 constructs preparation admission rather than merely rendering a control. Lines171/174 re-derive busy/locked preparation state from independent phase-name arrays.

  **Trigger/cost:** adding a factor, preparation recovery case or terminal phase requires changing actor guards, this UI's phase classifications, dedupe refs and rendering together. A layout change now carries paid-admission and stale-event reasoning; extracting only JSX leaves that cost intact.

  **Improvement:** retain the existing actual actors; move pure eligibility/slot/publication projections to feature model and expose typed named coordinator events from the writing-test feature, then split material/candidate/estimate/publication views as local presentational components. Move busy/locked classifications to exhaustive actor-owned projections so new phases require explicit classification.

  **Decision needed:** decide whether preparation is an invoked child authority or a separately coordinated actor owned by one feature controller; define where the one-time completion/entrant-adoption event lives. Do not add a reducer/shadow flow or move feature-specific business coordination to shared.

  **Cost/validation:** medium/large. Existing actual actor tests plus WritingTestStudio.test.tsx/WritingTestSlots.test.tsx; add duplicate/stale preparation completion, changed factor/count, source replacement during a quote, publication receipt recovery and owner change. No provider calls, no viewport remount and no changed count/credit/publication policy. ARCH-69, MODEL-30/35/88 and EDIT-23 apply.


- F24 [?] P2 `frontend/src/features/ai-authoring/ui/AuthoringEditor.tsx:90` ScopedEditor, view bodies at462,517,528,556,599,706,837,908,926: operation-controller adaptation, nine view bodies and domain-specific editing policy share one975-line component.

  **Evidence:** ScopedEditor provides actions to studioFlowMachine (line132), adapts useAuthoring state/availability/navigation/focus, derives conflicts/direct-source/name/publication status, then inlines purpose/existing/choices/review/direct/refining/publication/working/completion views. Direct and refining views share input/preview presentation but have distinct controller semantics and preserve separate pane drafts. Each new authoring kind or field traverses this entire component.

  **Improvement:** keep one mounted ScopedEditor host and its actors/controller at that host; move named view bodies into typed local components, with explicit fields/actions and a common preview presentation component. A feature-owned pure view-model projection can replace repeated status/name/feedback ternaries. Extract stable component declarations, not component functions declared inside render.

  **Safe portion:** presentation extraction within the same feature preserves established EDIT/THEME policy and needs no new product decision. Do not unify template/guideline/voice domain editors or create a generic controller configured by dozens of boolean flags.

  **Cost/validation:** medium. AuthoringEditor.test.tsx, actual authoring/studio actor tests and ConfigurationHosts.test.tsx plus all direct-editor host regressions; verify one operation/poller, typed draft/caret continuity across method/breakpoint changes, named publications and correct voice-new-copy semantics. ARCH-14/69, EDIT-10/11/14/21 and THEME-14/51/55 apply.


- F25 [?] P2 `frontend/src/features/writing-test/model/writing-test-machine.ts:419` failureOf; `model/candidate-preparation-machine.ts:335` failure: local failure recognition duplicates and bypasses the shared parameter validation boundary.

  **Evidence:** both helpers accept any object with a reason present in appFailureSpecs, then cast to AppFailure; candidate preparation additionally accepts nested `error.failure`, whereas writing test does not. Neither local branch verifies/freezes params as `shared/api/app-failure.ts:396` normalizeAppFailure does. Hence adding a required failure parameter or wrapping a domain error changes behavior differently across adjacent actors despite their shared user-facing failure contract.

  **Improvement:** share a single unknown-to-local-failure reader in shared/api that validates a candidate reason/params/technicalDetail through normalizeAppFailure and then applies the existing Connect fallback. Keep operation-specific uncertain/conflict decisions in each actor. Avoid merging actors or silently assigning product-specific fallback reasons.

  **Cost/validation:** small/medium, safe existing contract. Pin direct/nested valid AppFailure, missing/incorrect/extra params, unknown reasons, frozen copies and real Connect details, then actual writing-test/candidate-preparation actor error/retry tests. No current server-output exploit is asserted; this is a demonstrated loss of the existing normalization guarantee. ARCH-17/24 apply.


- F26 [?] P3 `frontend/src/features/generate-clip/api/useClipQuote.ts:31`, `features/revise-clip/api/useClipRevision.ts:113`, `features/request-clip-storyline/api/useClipStorylineRequest.ts:72`: the same clip approval deadline clock and validity gating are copied across three user actions and have already diverged.

  **Evidence:** all use Date.now state, deadline parse, a bounded POLL_INTERVAL_MS timeout, expired calculation and query.binding/fetch/error admission conditions. Generation additionally intersects batch expiry. Generation and plan revision include focus/visibility clock resampling; storyline revision has the copied timer but no wake listeners. This is shared lifecycle mechanics, while each request binding and start protocol is a separate domain use case.

  **Improvement:** publish a ClipQuote-lifecycle clock/validity hook from the owning clip-project entity API, receiving the already-computed deadline/binding and query state; keep generation's batch minimum, revision's saved-plan flush/revision comparison and storyline's own binding/command separate. Do not create a catch-all paid-request hook taking operation-name switches.

  **Cost/validation:** small/medium and no policy change. Fake-timer tests for boundary, focus/visibility wake, changed deadline, expired/refetch states and teardown; existing ClipGeneration.test.tsx expiry/focus test plus plan/storyline request integration cases. Preserve current manual reapproval/no automatic provider work. ARCH-14/24 and CLIP-131/181 apply.


## notes
- The user authorized direct fixes when the existing contracts make the choice clear; broader abstraction, lifecycle, persistence or policy choices remain unadopted review proposals.
- Related previous review evidence: all-261008 F16 (scroll lock), F26/F27 (catalog dependencies), F22 (billing read pool), F32 (refund transport seam), F35 (build schema split). The earlier complete audit and unrelated STATE edits remain outside implementation commits.
- Positive backend evidence: consumer-owned ports, generation startPreconditions/observePlan, experiment/app Publications and owner-scoped private request witnesses make real boundaries explicit; retain those rather than adding an application-wide generic repository.
- Positive frontend evidence: entity adapters, framework-free models, shared controls and real XState actors are useful. The autosave scheduler is appropriately shared while post-draft and canonical-content conflict rules remain separately owned.
- Positive media evidence: one frozen snapshot/output clock, owner/revision/version fences, bounded range readers and explicit worker leases are sound. Browser and Go layout/timeline duplication is deliberately pinned by fixtures and reflects distinct runtimes.
- Keep separate: manual versus AI origin reconciliation; platform-specific output lowering; domain-specific storage/RPC mappers; browser versus native codec execution; lifecycle caches for ink and media ranges. Share a leaf mechanic only when its full contract matches, preserving named domain admission rules.
- Scope decisions: constructor/port hardening and authoring application placement need an explicit consumer migration plan; durable JSON/hash and canonical post-type changes need byte/unknown-field compatibility decisions; writing-test and authoring view decomposition must preserve one transition authority and stable mounted inputs; dormant media paths need a clear diagnostic-retention decision before removal.
- Read-only contributor evidence: the existing frontend proto-boundary guard passed four assertions despite generated content aliases/entity-model schema imports; that is a guard blind spot, not proof of conformance. A temporary rendered null-builder-state regression failed before the planned fix. Other new recommendations have source/caller evidence, not claimed measured latency/performance or production incidents.
- Cross-cutting findings from all-261008 remain separate; this review adopts only the specifically listed mechanical/structural scopes and does not silently adopt all prior security, money or retention-policy proposals.
