# EDIT conversational configuration authoring
> r2 | Private AI-assisted drafting of reusable writing and video settings through eight suggestions, one selected draft, conversational refinement and explicit publication.

## decisions
- EDIT-1 [o] the authoring kinds are post templates, video templates, post guidelines, video guidelines and writing styles; each session fixes one kind and belongs to one authenticated account.
- EDIT-2 [o] an explicit recommendation request produces exactly eight different, valid suggestions together from one bounded write-model call; the eligible prepared active write selection is frozen at admission without a model-selection prerequisite.
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
- EDIT-14 [o] the default authoring surface is suggestions, readable preview and chat; manual forms, template builders and raw source remain explicit advanced alternatives. Saved-setting editors offer AI refinement from the owned current setting.
- EDIT-15 [o] first-use setup and settings destinations share the same authoring behavior. Closing keeps durable work available on return, and a confirmed save updates the caller's actual setting and destination.
- EDIT-16 [o] authoring stores only the requested setting, its necessary scope/version metadata and conversation; it reads no unrelated materials, account writing, photo or other settings into the model request. Account deletion removes the private authoring records.

- EDIT-17 [o] the authoring funnel separates purpose, eight-candidate comparison, one selected-result review, optional chat, explicit save and completion; changing views never starts a provider request or publishes a setting.
  - the selected preview remains available in refinement, which is optional; saving without chat is allowed when the domain validates the selected draft
  - saving a writing style shows its synthetic new-copy semantics and optional default choice at publication; templates/guidelines state the preserved target scope when relevant
  - the server session and job, not a presentation step, determine valid work, owner isolation, interrupted-save recovery and mutation eligibility

## flow
- create: choose setting kind → explicit eight suggestions with estimate → durable generation → select one → readable draft + chat → optional explicit refinement → Save → confirmed setting
- refine: owned setting → AI refinement session → chat + preview → explicit Apply → matching target version updated | personal voice becomes a new synthetic style
- recover: reopen → owner/kind/target session read → pending job progress | prior valid suggestions/draft → continue without automatic model work

## constraints
- counts: eight recommendations; user message ≤2000 Unicode characters; twenty completed exchanges; recent model context is bounded independently of stored conversation
- the target domain owns content limits, parsing, account caps, scope, uniqueness, defaulting and lifecycle guards
- publication must tolerate an interruption between a target-domain commit and its authoring receipt without duplicate settings or later-state reversal

## chg
-
