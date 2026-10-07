# EDIT conversational configuration authoring
> r4 | Private bounded reusable-setting authoring with kind/mode-specific composition, one draft and explicit publication.

## decisions
- EDIT-1 [o] the authoring kinds are post templates, video templates, post guidelines, video guidelines and writing styles; each session fixes one kind and belongs to one authenticated account.
- EDIT-2 [o] an explicit recommendation request produces the requested2/4/8/16 different valid suggestions from one bounded write-model call; ordinary setting creation defaults to eight and unified tests request their exact entrant count. Freeze the prepared eligible write ref and the count/budgets at admission; never silently reduce a batch.
- EDIT-3 [o] post-template suggestions cover familiar blog structures such as a visit review, travel journal, product review, everyday diary, practical guide, curated list, comparison and information summary; suggestions may adapt these structures to the owner's plain-language request without copying a real blog or inventing owner facts.
- EDIT-4 [o] choosing a suggestion changes the session's draft only; reading, selecting, previewing, opening and closing create no model work or saved setting.
- EDIT-5 [o] a sent chat message is one explicit bounded model job using the selected current draft, the session purpose and a bounded recent conversation; its validated response updates the draft and adds a plain-language reply, while the canonical saved setting changes only on explicit Save.
  - each user message is nonblank and at most 2000 Unicode characters; retain at most twenty completed exchanges and explain when a fresh conversation is needed
  - the current draft and latest request are never silently truncated; recent history may be shortened to the input bound without changing stored history
  - conversational replies describe the change in natural language and do not expose XML, JSON or provider implementation details
- EDIT-6 [o] sessions, suggestions, selected drafts, replies, active job and save receipts are durable private account data; all lookups and updates are owner-scoped, and unknown and foreign identifiers are indistinguishable.
- EDIT-7 [o] each accepted mutation uses the expected session revision; duplicate operation keys return the same admitted work, and stale responses cannot overwrite another account, kind, selection or later draft.
- EDIT-8 [o] one authoring model job may be active per account. Failure, cancellation and provider uncertainty retain the last valid draft, suggestions and request text; interrupted uncertain work is never repeated automatically.
- EDIT-9 [o] saving validates the selected revision with its target domain and publishes at most one setting idempotently; interrupted publication can be retried without creating another setting or undoing later explicit changes.
  - existing template and guideline targets require the captured target version to match; a conflict or deletion preserves the draft and explains why it cannot be applied
  - scope links and template generation numbers stay under their existing domain rules and cannot be changed by model output
- EDIT-10 [o] each domain keeps its own semantics: a template holds structure, a guideline holds writing direction, and a writing style holds sentence character; AI drafts do not cross these layers silently.
- EDIT-11 [o] AI writing-style output is synthetic and its example is fictional; it never enters personal materials, readiness or personal excerpts.
  - refining a personal voice creates a new synthetic style on Save, preserving its personal analysis and samples
  - refining an existing synthetic voice also publishes a new synthetic style, leaving its stored analysis and previous snapshot intact; the save action clearly names creation of a new style
  - choosing the default voice is explicit and applies only on first publication of that revision
- EDIT-12 [o] every generation and chat request displays its estimate and uses shared admission, metering, cancellation and settlement; no hidden correction, fallback, automatic acceptance or preview call spends credits.
- EDIT-13 [o] AI-generated content is bounded and domain-valid before it becomes a visible completed draft; invalid output fails with a friendly stable reason and retains the previous valid state.
- EDIT-14 [o] an existing saved setting opens as that named usable item and offers peer AI editing and direct editing methods, both starting from its same captured saved baseline.
  - manual builders/source views remain submodes of direct editing, not a competing source of truth
  - unfinished work is continued through a separately named state action; loading saved content is not presented as an editing method
  - personal voice fingerprints stay read-only; editing a voice description follows EDIT-11 synthetic-new-copy semantics, while personal source editing follows VOICE-64
- EDIT-15 [o] setup/settings hosts share the same authoring behavior and recover owner/kind/target-scoped unpublished work. Confirmed publication identifies the setting kind/name and whether it created a new item or updated the named target, then shows the confirmed saved item.
- EDIT-16 [o] authoring stores only its requested setting, necessary scope/version metadata, conversation and any explicitly owner-supplied bounded form reference. It automatically reads no account prose, photos, other settings or unrelated materials into the model request; account deletion removes all private authoring data.
- EDIT-17 [o] new setting creation separates purpose, candidate choice, selected review, optional refinement, explicit named publication and completion; an existing-setting edit starts from saved-item review and an explicit method, without an irrelevant recommendation/purpose prerequisite.
  - selected valid drafts may save without chat; refinement keeps the related preview available
  - voice publication states synthetic new-copy/default semantics; template/guideline publication names the retained scope
  - server session/job/receipts determine valid mutations and uncertain-save recovery; presentation movement starts neither AI nor publication

- EDIT-18 [o] saved availability and private editing state are separate facts: usable saved item, unpublished changes, active AI request, publication confirmation needed, or target conflict.
  - a saved item remains usable while a private draft is edited; an unsaved new item is not shown as an already usable setting
  - failures/unknown reads are not treated as absence; summaries identify the current kind/target and confirmed publication
- EDIT-19 [o] known-target copy identifies kind, display name and outcome for entry, draft continuation, save and completion.
  - use concrete AI editing/direct editing/continue editing/save changes wording, not an unexplained load revision draft or saved settings phrase
  - if a title is absent, identify the kind plus a readable text summary; synthetic voice saving explicitly creates a new AI-generated style
- EDIT-20 [o] starting a fresh chat retains the current working draft and clearly resets conversation context; resetting to the named saved baseline is a separate explicit discard action with a warning when it loses unpublished changes. Neither action publishes a setting or performs AI work.
- EDIT-21 [o] AI and direct editing of the same session share one durable working draft and captured target version; switching methods retains changes, while explicit Save alone changes the canonical target.
  - direct changes use the same revision/idempotency/owner fencing as chat; a stale response cannot undo a newer manual edit
  - bounded incomplete/invalid manual source is retained with its parse state and last valid preview; publication and AI-completed output require domain validity
  - domain-owned scope, generation numbers and provenance are retained unless the owner explicitly edits permitted fields; AI output/method switches never change them silently
- EDIT-22 [o] owner-scoped directory summaries expose saved availability, unpublished-work state, active job and last confirmed publication without per-row conversation/content fetches; new unsaved creations are separately recoverable and are never silently mixed into the saved directory.
- EDIT-23 [o] unified tests may consume a frozen unpublished candidate revision after domain validation without publishing it; explicit winner adoption publishes exactly the tested setting snapshot under MODEL-90, never regenerating or labeling synthetic writing as personal evidence.

- EDIT-24 [o] setting authoring composes only the selected kind/mode's material, domain grammar and consumed response fields (→EDIT-1/10/16 →MODEL-93).
  - literal/fact/instruction boundaries and template grammar meaning match ordinary composition (→TMPL-70); irrelevant kind examples and discarded output instructions do not consume context
  - selected draft/latest request are authoritative current material; bounded recent history cannot silently undo newer direct edits
  - optional technical inspection uses MODEL-94/95, separately from setting publication provenance and ordinary natural-language replies; it creates no call or saved setting

## flow
- create: choose setting kind → explicit candidate suggestions with estimate → durable generation → select one → readable draft + chat → optional explicit refinement → Save → confirmed setting
- refine: named saved item → AI editing | direct editing → one retained private draft → explicit named Save → version-matched template/guideline update | new synthetic voice copy
- recover: reopen → owner/kind/target session read → pending job progress | prior valid suggestions/draft → continue without automatic model work

## constraints
- counts: recommendation count2/4/8/16; user message ≤2000 Unicode characters; twenty completed exchanges; recent model context is bounded independently of stored conversation
- the target domain owns content limits, parsing, account caps, scope, uniqueness, defaulting and lifecycle guards
- publication must tolerate an interruption between a target-domain commit and its authoring receipt without duplicate settings or later-state reversal

## chg
-
