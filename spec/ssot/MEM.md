# MEM memories
> r3 | An account owns short atomic facts about its author's world, approved by hand from what a finished post yielded, retrieved by tag and injected as the write prompt's fourth grounding source only when the draft opts in

## decisions
- MEM-1 [o] a memory (the user-facing noun 기억) is ONE atomic fact about the author's world, authored by the user approving an extracted candidate or by writing it by hand; an account owns zero or more, and an account with none produces prompts byte-identical to the ones it would produce without this domain
- MEM-2 [o] memory is the fourth authored layer beside the three that exist: VOICE decides how sentences sound, TMPL decides shape and required content, GUIDE decides what to avoid, MEM supplies facts no single post carries ← without it a post is confined to what one memo and one photo set can prove, which no voice rule, template brief or prohibition can widen
- MEM-3 [o] a memory is account-owned: not voice-scoped, not template-scoped, and carrying no per-post enable ← a fact about the author holds across every voice they write in, and `kind` plus tags already separate subject matter without partitioning the store
- MEM-4 [o] a memory carries `text`, one `kind`, zero or more tags, zero or more source-post links and timestamps; nothing else is authored and nothing is scored, ranked or weighted by a model
- MEM-5 [o] `kind` is a closed enum of five — `preference` 취향 · `persona` 설정 · `place` 장소 · `person` 인물 · `history` 이력 — with no user-defined kinds and no sub-kinds ← retrieval behaviour and prompt grouping both rest on the enum being closed; anything finer is a tag
- MEM-6 [o] retrieval treats the enum in two halves: `preference` and `persona` are candidates for every post regardless of tags, while `place`, `person` and `history` are candidates only on tag overlap ← a standing fact about the author is what lets a post leave the frame, and an unfiltered place or person fact staples an unrelated shop to the next post
- MEM-7 [o] the retrieval key is built from the post's memo, 가제, template answers and the observation `objects` and `visible_text` of its photos and videos; score is tag overlap, ties break by most recent use then creation, and at most `MEMORY_INJECT_MAX` (8) are selected
- MEM-8 [o] retrieval is deterministic and lexical, using the Go standard library alone — no vector, embedding, similarity or third-party dependency (→VOICE-51) ← at this corpus size tag overlap answers the only question asked of it, and an embedding call per generation would cost credits for no measurable gain
- MEM-9 [o] deduplication is exact after trim and nothing else: a re-approved identical text adds its source link and advances `last_seen_at` rather than creating a row, and no similarity, fuzzy or semantic matching exists anywhere (→GUIDE-10)
- MEM-10 [o] two memories may contradict each other and the product never resolves it: nothing merges, supersedes, retires or rewrites a stored fact; the more recent one is injected first and the user deletes what is no longer true ← an automatic merge discards a fact the user approved, silently and with no record
- MEM-11 [o] `MEMORY_MAX_PER_ACCOUNT` (300) is a hard cap: at the cap nothing is saved, the surface says so and names the cap, and nothing is evicted (→GUIDE-10) ← evicting the oldest would discard exactly what the user chose to keep
- MEM-12 [o] field rules: `text` trimmed, non-empty, ≤ `MEMORY_TEXT_MAX_CHARS` (120) Unicode scalar values; at most `MEMORY_TAGS_MAX` (5) trimmed non-empty tags, duplicates within one request collapsing to one; an absent or unknown `kind` is refused, never defaulted
- MEM-13 [o] extraction is explicit: `기억으로 저장` in ③ starts a durable job that passes the shared enqueue credit gate like every other LLM job (→QUOTA-13), is refused with `INSUFFICIENT_CREDITS` the same way, and is retried the same way ← charging for extraction inside a finalize would bill users who never wanted memories
- MEM-14 [o] extraction reads the post's current canonical content and its memo, and answers candidates each carrying proposed text, kind and tags; it writes no memory, changes nothing about the post, and a failure leaves the finished post exactly as it was
- MEM-15 [o] a candidate is never stored: the surface lists them with checkboxes, only checked ones are created, and unchecked ones are discarded rather than queued for later review ← the extraction is repeatable on demand, so a pending queue would preserve what the user already declined
- MEM-16 [o] what is stored is the approved facts alone — never the post body, never a summary of it, never the raw candidate list
- MEM-17 [o] a memory links every post it was approved from; deleting a post drops that link and deletes the memory only when it was its last one ← a fact re-confirmed across several posts outlives any one of them, while a fact that existed only inside a deleted post leaves no orphan
- MEM-18 [o] a post carries a `use_memory` option, default off, saved with the writing brief's run options (→POST-89) as an option save that changes no status, revision, baseline or learning eligibility (→POST-62); a post without the field decodes as off
- MEM-19 [o] the selected memories are resolved once at enqueue and frozen as text into the generation payload beside the template brief and the guideline texts (→GEN-15) and into a write comparison's snapshot (→GEN-18); handlers never re-read the rows, so editing or deleting a memory after the start changes nothing in flight, across restart-resume or retry
- MEM-20 [o] the frozen memories render as ONE `[기억]` section in the per-post half of the write prompt, beside the memo and the observations, never in the stable prefix ← the selected set differs per post, and the prefix is what the provider's cache and every prompt golden rest on
- MEM-21 [o] the section closes with its own line naming the memories as legitimate material for this post, appended only when the section exists (→GEN-16 →GUIDE-16), so a post with the option off carries no memory bytes and no mention of a source it has none of (→TMPL-46's conditional legend)
- MEM-22 [o] memories reach the write pass only; the revise pass receives none ← revise holds neither memo nor observations, and material it cannot check against would license rewriting sentences the request never touched
- MEM-23 [o] nothing is learned without the user: no model creates, approves, edits, ranks, retires or deletes a memory, no threshold promotes a candidate, and no memory reaches any prompt except through `use_memory` ← recording what the user checked is not learning about them
- MEM-24 [o] `기억` is the fifth destination of the 글 group (→THEME-38, after 지침 →GUIDE-26), listing the account's memories with kind and tags, a read-first text edit, kind and tag edits, and a delete with no undo; the page carries no standing form — one docked `새 기억` opens the shared `Sheet` (→THEME-24)
- MEM-25 [o] a memory may be written by hand on that screen with the same field rules ← a fact the author knows on day one should not require generating a post first
- MEM-26 [o] the `기억 사용` checkbox sits in the writing brief beside the other run options (→POST-51 →POST-89)
- MEM-27 [?] whether a memory's language is recorded, and whether a Korean memory may be injected into an English post or is filtered out of it (→LANG)
- MEM-28 [?] whether the management screen surfaces memories retrieval has never selected, so a fact nothing matches can be retagged rather than sitting unread
- MEM-29 [x] voice scoping, template scoping, per-memory enable/disable, manual ordering, version history, import/export, sharing between accounts, seeded memories, similarity deduplication, auto-approval, embeddings, a relationship or entity graph, memories in the revise pass, memories in clip generation — out of scope

## flow
- capture: ③ `기억으로 저장` → credit gate → extraction job → candidate list in a sheet → user checks → approved rows created(new|link added to an identical row)
- use: the brief's `기억 사용` on → StartGeneration retrieves and freezes the selected memories → write prompt carries `[기억]` → generated post
- retrieval: key from memo + 가제 + template answers + observation objects and visible text → `preference`·`persona` always candidates + `place`·`person`·`history` on tag overlap → score by overlap, tie by recency → top 8

## constraints
- retrieval, tokenization and tag matching use the Go standard library alone; no vector, embedding, similarity or model dependency (→VOICE-51)
- every bound (`MEMORY_TEXT_MAX_CHARS` `MEMORY_TAGS_MAX` `MEMORY_MAX_PER_ACCOUNT` `MEMORY_INJECT_MAX`) is code-owned in the backend and published to the client; the frontend hardcodes none (→QUOTA-8)
- the memory queries file stays ASCII-only ← sqlc mis-slices a query file containing any multi-byte character (→GUIDE-26)
- a post with `use_memory` off produces a prompt byte-identical to the one it produces without this domain

## chg
-
