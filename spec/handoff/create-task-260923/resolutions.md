# Binding engineering resolutions for the published-status delta

Every task written from this delta MUST follow these. They settle the dev forks the nine area maps
raised (scratchpad/maps.json) and the 15 places where two maps disagreed. Planning truth is the SSOT
at HEAD 19b232e4: QUAL r2 (all), POST r11 (r10+r11), GEN r10 (r9+r10), GUIDE r5 (r4+r5), TMPL r10
(r9+r10). Where a map recommendation conflicts with this file, this file wins. Where this file is
silent, follow the map's recommendation for that area.

## Identifiers and wire types
- R1 분야 is a proto enum `BlogField` in `proto/postpilot/v1/post.proto` (it is a post attribute first):
  `BLOG_FIELD_UNSPECIFIED = 0` (없음), `BLOG_FIELD_RESTAURANT = 1`, `_CAFE = 2`, `_DOMESTIC_TRAVEL = 3`,
  `_FASHION_BEAUTY = 4`, `_PRODUCT_REVIEW = 5`, `_PARENTING_MARRIAGE = 6`, `_PETS = 7`,
  `_INTERIOR_DIY = 8`, `_DAILY_LIFE = 9`. SQL stores the ASCII ids `restaurant cafe domestic_travel
  fashion_beauty product_review parenting_marriage pets interior_diy daily_life` as TEXT with no CHECK;
  a domain parser refuses unknown ids. Generated-enum walk tests pin the mapping on both sides (ARCH-3).
- R2 The canonical 분야 catalogue (id, Korean display name, batch query string = name with `·` replaced
  by a space) is Go code owned by `internal/quality` (QUAL-23). `internal/post` and `internal/guideline`
  never import it: each declares a consumer-owned port (`FieldDirectory{ Known(id string) bool }`)
  wired in `cmd/api` (ARCH-6). The FE mirror is `entities/blog-field` (enum ↔ id ↔ label; labels in
  that entity's own i18n fragment, ko and en) — never a bare `field` slice, which already means form
  field in shared/ui.
- R3 Metrics: proto enum `QualityMetric { QUALITY_METRIC_UNSPECIFIED = 0; _TITLE_SATURATION = 1 (M1);
  _CROSS_POST_PHRASES = 2 (M2); _IN_POST_REPETITION = 3 (M3); _COMPOSITION = 4 (M4) }`, ASCII ids
  `title_saturation cross_post_phrases in_post_repetition composition`. Verdict: `QualityVerdict {
  _UNSPECIFIED = 0; _OVER_BAND = 1; _WITHIN_BAND = 2; _BELOW_MINIMUM = 3; _ABSENT = 4 }`. Both in
  `proto/postpilot/v1/quality.proto`, walked on both sides.
- R4 Post status stays a string (`Post.status`, `PostSummary.status`); add `"published"` to
  `internal/post/types.go` beside draft/review/finalized with a BE test pinning the full set, the
  experiment mirror (`experiment.PostStatusFinalized` neighbours) and the FE `PostStatus` union and
  every exhaustive `Record<PostStatus, …>` (steps BY_STATUS → ③, badge tone, filter values).
- R5 RPC `SavePostPublishedUrl(slug, url) returns Post` on PostService; an empty `url` clears.
  Failure reasons (append to `error.proto` after the highest used number, never reusing reserved names
  or numbers): `POST_PUBLISHED_LOCKED` (any write refused on a published post) and
  `POST_PUBLISHED_URL_INVALID` (address fails POST-77). A paste on a draft/review post reuses the
  existing `POST_NOT_FINALIZED = 132`. `PUBLISH_*` names are reserved and forbidden.
- R6 FE slice for the URL field: `features/record-published-url` (the `features/publish-post` path is
  forbidden by `pnpm lint:retirement`). Hook in `entities/post/api` named `useSavePublishedUrl`.

## Post lock (POST-74) and URL (POST-73/75/77/84)
- R7 Every write statement on `posts`/`images`/`videos`/`uploads` that a published post must refuse gets
  a SQL predicate `status <> 'published'` as the backstop, and the service returns a new sentinel
  `post.ErrPostPublished` → `POST_PUBLISHED_LOCKED` before any write: UpdatePostDraft, SavePostContent,
  UpdateGeneratedContent, SavePostGenerationOptions, FinalizePost, ReassignPostVoice, AssignPostTemplate,
  template-answer saves, CreateUpload/ConfirmUpload, DeleteImage/DeleteVideo. An identical
  SavePostContent on a published post is still a no-op (POST-15 idempotence) rather than a refusal.
  DeletePost is NOT locked by status; its existing busy refusal (POST-29, any active job) stays.
- R8 Paste/replace: allowed when status is `finalized` with `content_revision = finalized_revision`, or
  `published` (replace). One statement sets `status='published'`, `published_url`, `published_at=now`.
  Clear: sets `status='finalized'`, both columns NULL. Refused with `ErrPostBusy` while a generate,
  revise or model_experiment job targets the post; allowed while learn_voice or extract_memory runs.
  CHECK constraints on the new columns: `published_url` and `published_at` both NULL or both NOT NULL.
- R9 URL validation (shared BE list, FE pre-check mirrors it exactly): scheme http/https, host
  `blog.naver.com` or `m.blog.naver.com` (case-insensitive), no userinfo, no port, non-empty path;
  stored as `https://blog.naver.com` + path (+ query if present), trimmed. Max length a `limits.go`
  constant (2048).
- R10 Generation refuses a published post in Start and StartRevision via a `Published bool
  json:"-"` on `generation.PostInput` (the tag keeps experiment snapshot hashes unchanged). The
  experiment context refuses applying a winner to a published post and refuses an editor-origin write
  comparison start on one; lab comparisons that write nothing stay allowed (MODEL-31).
- R11 VOICE learning on a published post: `LearningSnapshot`'s `status == finalized` gate becomes
  `status IN (finalized, published)`; `finalized_revision == content_revision` always holds there.

## 분야 on a post (POST-82)
- R12 `posts.field TEXT NULL` (NULL = 없음). Saved through the draft queue (SavePostDraft
  `optional BlogField field`, presence-aware like the template channel: absent keeps, UNSPECIFIED
  clears), including on create from `/posts/new`. FE `features/select-post-field` renders the
  app-drawn `Listbox` under the template data fields and above 기억 사용.

## Measurement (QUAL)
- R13 New flat context `backend/internal/quality` (ARCH-5 start flat; note in impl notes that ARCH-5's
  list is already stale — clip and memory are missing — and no ARCH edit blocks this). Service with
  Deps (store, PostSource port, Phrases store, clock). `proto/postpilot/v1/quality.proto`:
  `QualityService { GetPostMeasurement(slug) ; GetAccountQuality(slug) }`. Registered in serve.go;
  `cmd/api/wiring_test.go` handler count 21 → 22.
- R14 QUAL reads posts only through a consumer-owned port `PostSource` implemented in
  `cmd/api/adapters_quality.go` over `post.Service`: `Post(ctx,user,slug)` → {revision, content,
  contentLanguage, nouns, status} and `Published(ctx,user,limit)` → []{slug, revision, content,
  contentLanguage, nouns, publishedAt} ordered by `published_at DESC` (new post store query
  `ListPublishedPostsByUser`).
- R15 Storage: `post_measurements(post_slug TEXT PK, user_id TEXT NOT NULL, content_revision INTEGER
  NOT NULL, measure_version INTEGER NOT NULL, char_count INTEGER NOT NULL, photo_count INTEGER NOT
  NULL, distinct_block_types INTEGER NOT NULL, avg_sentence_length REAL NULL, repetition_share REAL
  NULL, top_noun TEXT NULL, title_relevance REAL NULL, computed_at TEXT NOT NULL, FOREIGN KEY
  (post_slug, user_id) REFERENCES posts(slug, user_id) ON DELETE CASCADE)`. Self-only metrics (M3, M4)
  are computed lazily on read when the row is missing or its revision/version differs, then upserted.
  M2 and the aggregate are computed at read (QUAL-4). The FK cascade covers POST-85; the FE
  invalidates quality queries after delete, URL save and content save.
- R16 Nouns (GEN-55): `posts.content_nouns TEXT NULL` (JSON array, NULL = none), written by
  SetGeneratedContent from a write answer; a revise result passes nil which KEEPS the stored value
  (presence semantics). Containment (QUAL-7/9): NFC-normalize, split on whitespace into 어절, trim
  leading/trailing punctuation; Korean — an 어절 contains a noun when it starts with it; English — a
  word equals it case-insensitively. `content_language` NULL → Korean. No tokenizer dependency.
- R17 M1: over content titles of the last 100 published posts; candidate nouns = union of those
  posts' stored nouns; winner = noun contained in the most titles (ties: longer noun, then
  lexicographic); value = titles containing it / titles considered.
- R18 M2: token windows of 8 consecutive 어절 hashed across TEXT/HEADING/QUOTE/LIST text of a post; the
  "others" are the last 20 published posts excluding the post itself; share = runes of tokens inside
  any matched window / runes of all tokens; account value = median over those 20; the named run for
  the rule text = the maximal matched run standing in the most of the 20, ties by longer then earliest.
- R19 M3: per post, repetition share = occurrences of the most contained noun in the body / occurrences
  of all the post's nouns in the body; title relevance = of the nouns contained in the title, the share
  also contained in the body. Absent when the post has no stored nouns.
- R20 M4: char count = NFC runes of TEXT/HEADING/QUOTE/LIST text; photo count = IMAGE blocks with a
  non-empty file; distinct block types among TEXT HEADING IMAGE VIDEO QUOTE LIST; average sentence
  length over TEXT blocks only (Korean runes per sentence, English words per sentence; split on
  . ! ? … and newlines with a decimal-number guard). Minimum 3 published posts for the account value.
- R21 Bands, minimums, windows (100, 20), run length 8, measure_version, stopword lists, the batch
  interval default and the rule texts are Go constants in `internal/quality` (limits.go, stopwords.go,
  rules.go). The server computes every verdict; the FE mirrors no band (T292 precedent).
- R22 `GetAccountQuality(slug)` returns, per metric: verdict, the value(s), the band, the minimum, the
  published count, and for an OVER_BAND metric the exact rule text rendered in THAT post's target
  language (so the toggletip quotes what ticking adds, QUAL-13/14). `GetPostMeasurement(slug)` returns
  M2/M3/M4 values with verdicts for the post's current revision; every value `optional` (QUAL-40).
- R23 Quality ticks: `posts.quality_rules TEXT NULL` (JSON array of metric ids) saved through
  SavePostGenerationOptions with a presence wrapper message; Post carries the current ticks. At enqueue
  generation calls a consumer port `QualityRules.RulesFor(ctx,user,slug,ticked,lang) []string` (cmd/api
  adapter → quality) that keeps only ticks whose metric is OVER_BAND right now and returns their
  rendered texts; the texts freeze into the payload and into the write-experiment snapshot.

## Phrase batch (QUAL-17/38/41/42)
- R24 `field_phrase_lists(field TEXT PK, phrases TEXT NOT NULL CHECK(json_valid), corpus_size INTEGER
  NOT NULL, refreshed_at TEXT NULL, next_refresh_at TEXT NOT NULL)`. An in-process ticker pass started
  in serve.go (billing renewal precedent — state in impl notes why this is not an ARCH-11 job record:
  it has no owner and generation_jobs requires one) with per-field durable `next_refresh_at`, catch-up
  on boot, interval env `QUALITY_PHRASE_REFRESH_INTERVAL` default 24h, disabled entirely when
  `NAVER_SEARCH_CLIENT_ID` or `NAVER_SEARCH_CLIENT_SECRET` is unset (a legal, tested mode). A failed
  page keeps the last successful list and reschedules that field; per-field errors are joined.
- R25 Naver wrapper `backend/internal/naversearch` (fxrate precedent): GET
  `https://openapi.naver.com/v1/search/blog.json?query=…&display=100&start=1|101|201&sort=sim` with the
  two headers, per-request timeout, strips `<b>`/`</b>` and applies `html.UnescapeString` before
  returning plain title/description; tested against httptest. The implementer reads the official
  Naver docs before coding (ARCH-33). Declare the env vars wherever the repo declares secrets
  (DEPLOY.md, docker-compose, env examples).
- R26 Phrase extraction, ONE implementation `internal/quality/phrases.go`: NFC, whitespace 어절 with
  punctuation trim, runs of 2–5 tokens, drop runs made only of stopwords (`stopwords.go`, ko and en),
  frequency = number of items (a title or a description) containing the run, same-count subsumption
  (drop a run contained in a longer run of equal count), top 50 by count desc, then fewer tokens, then
  lexicographic. Generation reads the first 30 through a consumer port `Phrases.For(ctx, field)`;
  an empty or missing list == no 분야 (QUAL-41).

## Prompt and answer (GEN)
- R27 Code constants in `internal/generation/prompts.go`: the two title prohibitions (GEN-49) and the
  tag rule (GEN-50) join the static rules; when a template title area is present the prohibitions are
  worded to bind only the model-written `<write>` parts (TMPL-52). The ticked quality rules render as
  their own section after the static rules and before `[한국어 자연 문체 기준선]`, closed by a line
  saying a 지침 outranks them (GEN-51). The frozen phrase list renders in per-post material together
  with the instruction describing the `replacements` answer array, present only when phrases are
  present (keeps `TestWriteSystemPrefixIsStableAcrossPostMaterial` true). `nouns` is requested in every
  write answer. Precedence sentences (`templatePrecedence`, `guidelinePrecedence`) are rewritten per
  GUIDE-35/36: 문체·종결어미 stay with the voice, a concrete substitution ranks guideline > template >
  profile. Golden fixtures are regenerated deliberately and the diff reviewed.
- R28 Write answer schema gains `nouns: string[]` (≤ 40, always) and `replacements: [{surface: title|tag|
  body, index: int, source: string, phrases: string[≤3]}]` (≤ 20, only when phrases were given).
  Parsed and bounded server-side; persisted by R16 and R29; carried through writeCandidate,
  RunWriteCandidate and an applied A/B winner (GEN-4). The revise answer is unchanged.
- R29 Candidates: `posts.replacement_candidates TEXT NULL` (JSON), written by SetGeneratedContent from a
  write answer (nil from revise keeps them). NOT inside PostContent (it is copied into machine_baseline,
  exports and learning). Exposed on Post as `repeated ReplacementCandidate` with enum
  `ReplacementSurface { _UNSPECIFIED; _TITLE; _TAG; _BODY }`, `index`, `source`, `phrases`.
- R30 Revise (GEN-57): no phrase list; the guideline port is asked with a revise flag and omits the
  preset line.
- R31 Template title area in the brief (GEN-52): inside `[글 템플릿: name]`, after the legend and before
  the body fence, one fixed sentence that the JSON `title` follows this form, then the rendered title
  area between its own `---` fences; zero bytes when the title area is empty; no `지침` wording.

## Guidelines (GUIDE)
- R32 `fields` scope: rebuild `guidelines` so the scope CHECK admits `'fields'` (0014 pattern
  `CHECK (scope IN (...))`), add `guideline_fields(guideline_id, user_id, field, composite FK,
  UNIQUE(guideline_id, field))`; validation via the FieldDirectory port; resolution per GUIDE-14.
- R33 Preset: `guideline_presets(user_id PK, enabled INTEGER NOT NULL, updated_at)` and
  `guideline_preset_fields(user_id, field, PK(user_id, field))`, outside `guidelines` (GUIDE-39). The
  preset text is a Go constant (GUIDE-30), never a row. ListGuidelines' answer carries the preset
  state; `UpdateGuidelinePreset` is a presence patch (enabled, fields). `ForPrompt(user, templateID,
  field *string, forRevision bool) []string` appends the preset line LAST only when enabled, the post's
  field is in the preset's set, and not forRevision.
- R34 /guidelines renders the preset as a pinned row above the owner's list with a `Switch`-semantics
  control inside THEME-29's primitives, a 적용할 분야 multi-select from `entities/blog-field`, and copy
  saying it yields to the owner's rules; with no 분야 picked the row asks for one.

## Templates (TMPL)
- R35 `templates.title_area TEXT NOT NULL DEFAULT ''`; one tokenizer, parse restricted per area;
  `ParseError` gains `Area`; reason `not_in_title` lands in the shared parser fixture first
  (cases.json) so Go and TS stay in lockstep; `TEMPLATE_ASK_MAX_PER_BODY` counts both areas, title
  first; labels unique across both, checked inside the update transaction; config
  `TEMPLATE_TITLE_AREA_MAX_CHARS` 200 mirrored to FE `shared/config`. Title-area asks appear first
  among ①'s data fields.
- R36 Builder reuses TemplateComposition with an area variant; 원문 shows a second textarea above the
  body with its own error and counter; 형식 안내 stays body-only.

## Frontend shared
- R37 New `shared/ui` primitives on one extracted `useAnchoredPanel` hook (the logic Listbox already
  has): `InlinePopover` (native `<button>` trigger, whitespace-normal, text-left, break-words; press
  opens; hover opens additionally only under `(hover: hover) and (pointer: fine)`; Escape closes) and
  `Toggletip` (sibling of a checkbox's label so a press never toggles the box). Inline marks inside
  prose take WCAG 2.5.8's inline exception and the task says so; tag chips grow their hit area under
  `pointer-coarse:` to THEME-23's 44px.
- R38 i18n: reuse the `posts` and `guidelines` namespaces; ko and en carry identical keys; slice strings
  live in each slice's config segment per ARCH-16 (LANG-7's resource path is stale; do not add a
  namespace).
- R39 On ②: measurements render in one row above the article; replacement marks and measurements are
  hidden on a published post; on ① the brief's quality section is omitted on `/posts/new` (no slug).

## Verification and sequencing
- R40 Every BE task runs the complete ARCH-26 line locally, INCLUDING `test -z "$(gofmt -l .)"`, because
  CI does not run gofmt. Touching queries → `pnpm gen:sql`; touching proto → buf generate for both
  sides; sqlc query files stay ASCII-only (repo memory). Every FE task runs the FE gate named in ARCH.
  `internal/clip/store` has a known pre-existing flake (TestTheSoundSettingDoesNotInvalidate… and
  CLIP_SOURCE_UNAVAILABLE) — not a regression.
- R41 Migrations take the next free number AT IMPLEMENTATION TIME (HEAD's latest is 0076); tasks that add
  migrations or touch the same proto file are chained by dep so two never race for one number.
- R42 A final optional task extends `cmd/seed`/`internal/devseed` with published posts, nouns, candidates
  and a seeded phrase list so every new surface is visible locally without Naver keys.
