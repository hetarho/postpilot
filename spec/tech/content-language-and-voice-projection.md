# Tech — Content language and voice projection

The durable language/projection contract for
[plan 13](../plan/13.multilingual-interface-and-target-langua.md). It keeps UI locale, requested output language,
existing-content provenance, and voice-learning language separate so a locale switch cannot rewrite a post and an
English post cannot contaminate a Korean voice.

## 1. Four values, three durable concepts

The browser has a UI `Locale`; domain state has three language roles:

| Value                   | Owner                | Meaning                                                                            |
| ----------------------- | -------------------- | ---------------------------------------------------------------------------------- |
| UI locale               | browser/app provider | language of Postpilot chrome; never sent as domain authority                       |
| `Post.target_language`  | post                 | language of the next ordinary generation/write A/B                                 |
| `Post.content_language` | post                 | provenance of current machine-established canonical content; absent before content |
| `Voice.source_language` | voice                | immutable language from which that isolated profile learns                         |

The public proto uses a closed enum:

```proto
enum ContentLanguage {
  CONTENT_LANGUAGE_UNSPECIFIED = 0;
  CONTENT_LANGUAGE_KOREAN = 1;
  CONTENT_LANGUAGE_ENGLISH = 2;
}
```

Domain and storage values are canonical tags `ko` and `en`. Proto/domain/SQL mappers are the only conversion sites.
`UNSPECIFIED` maps to absence only for `content_language` and stage projections where language is inapplicable. It is
invalid for post creation, voice creation, generation/write snapshots, and language-sensitive profile work.

This is declared provenance, not language detection. The application does not inspect prose and silently rewrite a
language field.

## 2. Post state transitions

A new-post screen initializes target from the resolved UI locale, but the create request carries the concrete enum.
The server knows no browser preference and substitutes no default. Existing migrated posts backfill Korean.

Target changes are presence-aware fields on the draft snapshot:

```text
create: target required
update + field absent: preserve target
update + field present and valid: replace target
```

The request still carries the latest complete title/memo snapshot; presence awareness does not make those scalar
fields a sparse patch. The draft queue gives target selection the same newest-wins protection as voice/purpose
assignment. A delayed title/memo request cannot carry an old target back over a newer choice. Target updates do not change content JSON,
observations, status, revisions, machine baseline, voice/purpose assignment, or finalization.

`content_language` changes only when canonical machine content changes:

| Operation                           | Target                       | Content language                         |
| ----------------------------------- | ---------------------------- | ---------------------------------------- |
| target select                       | replace                      | preserve                                 |
| title/memo/photo/voice/purpose edit | preserve                     | preserve                                 |
| ordinary generation success         | preserve current post target | set to job's frozen target               |
| apply write A/B winner              | preserve current post target | set to experiment's frozen target        |
| manual block edit                   | preserve                     | preserve                                 |
| AI revision success                 | preserve                     | preserve frozen current content language |
| finalize/export/publish             | preserve                     | preserve                                 |

A target can change while an old-language job is queued. The old job still lands with its frozen language; the post
then deliberately shows `target != content_language`, meaning the next full generation will switch it. This is not a
race or stale result.

`SetGeneratedContent`/winner application receives content and language in one post-context operation so there is no
observable machine result without matching provenance. Revision requires content language and writes the same value
with its new baseline. Manual editing does not claim to detect a translation; if the user manually rewrites an entire
post into another language, provenance remains the last machine language until a future explicitly designed feature
allows changing it.

## 3. Voice source language

`Voice.source_language` is required and immutable.

- Existing voices and `adduser`'s default voice are Korean.
- New voice UI defaults the field from UI locale but sends an explicit value.
- Rename/default/delete/restore preserve it.
- There is no update-source-language RPC. A different source is a new isolated voice, preserving [I4].
- Samples pasted into a voice are user-declared evidence in that source language; the form names the expectation and
  no detector runs.

Voice/profile reads and the post's `VoiceRef` expose the source so the frontend can explain whether learning is
eligible. The backend remains authoritative and repeats the check immediately before enqueue and before applying
frozen work.

## 4. Same-language and portable projection

Generation asks voice for a projection with both source and frozen target. Voice—not generation or frontend—owns the
field selection.

### Same language

When `target == source`, preserve the complete current prompt contract:

- structured lexical, endings/register, syntax, structure, and axes;
- active contrast rules;
- legacy/manual styleguide and user rules;
- ranked imported and finalized excerpts; and
- the current ending-consecutive constraint.

### Cross language

When `target != source`, return only traits with meaning that survives translation:

- intro and closing pattern;
- paragraph sentence-count range;
- heading and list habits;
- emoji habit; and
- involvement, narrativity, persuasion overtness, abstractness, addressee focus, and humor axes.

Exclude all of the following:

- preferred/banned words and patterns;
- base register, ending distribution/signatures/bans, and ending constraints;
- sentence-length/connective/nominalization/passive fields;
- free-form styleguide and unclassified user rules;
- lexical, endings, and syntax contrast rules;
- imported/finalized excerpts and their facts/phrasing; and
- any translated or generated substitute for excluded data.

Structure-layer rules are not automatically portable merely because their enum says `structure`: existing statements
are authored in one language and may encode language-specific wording. Only the structured fields listed above cross
the boundary. Axes cross as numeric values.

The returned projection records `source_language`, `target_language`, and `portable=true|false` for prompt assembly
and tests. Prompt assembly labels a portable section and states that target language wins over purpose/profile language
conflicts. Purpose and memo remain verbatim user inputs; they do not join the voice profile.

## 5. Generation and durable freezing

`StartGeneration` reads the owned post after draft flush and validates a target. It freezes target into the generation
payload beside target length and purpose. The handler uses only payload target even if the post changes while queued.
Legacy queued payloads without language decode as Korean during the compatibility migration; newly encoded payloads
may not omit it.

Write prompt assembly uses target to:

- state the required prose/title/summary/tags/alt/caption language;
- request the correct voice projection; and
- set the content-language value passed with a successful result.

The LLM provider port remains language-blind. It receives prompt text/schema/model ref as today and no provider-specific
language option crosses the boundary.

### Observation independence

Observation is a semantic photo-fact stage, not a translation stage. Target language is absent from its prompt builder,
observation-only experiment hash, candidate request, and persisted `Observation` shape. Batching, filename matching,
actual visible text, scene/mood/object facts, and contact-sheet display remain unchanged.

Consequences:

- changing target alone does not clear or translate observations;
- otherwise-identical Korean- and English-target runs form the same observation requests;
- a prepared write A/B may feed the same fact snapshot to both target-language writers; and
- `visible_text` remains what the photo contains, not a translated rendering.

The full write snapshot still includes target because target changes writer input and the experiment hash even though
the observation sub-input is invariant.

## 6. Write experiments and revision

A write A/B snapshot freezes post, voice id/source, target, portable/full projection, purpose, target length, images,
and observations once. Candidate prompts differ only by explicit model. Retry reads the same snapshot. Experiment
detail records target; applying a winner passes target to the post context with the chosen content.

Analyze experiments use the selected voice's source language and source-only corpus. Observe experiments remain
target-independent as described above.

The current leaderboard remains private to `(account, stage)` for every stage. Write target is retained in immutable
experiment metadata and input hash for reproducibility but does not partition the ranking projection.

Revision uses `content_language`, never `target_language`:

1. resolve owned content and require non-absent provenance;
2. freeze content language, voice, purpose, instruction, content, and attachments into the durable revision payload;
3. request the same-language or portable profile relative to content language;
4. explicitly instruct the writer to preserve that language and make only the requested local edit; and
5. persist revised content/baseline with the same content language.

A revision instruction asking for translation conflicts with the hard preserve-language rule. The UI explains that
language changes use target + full generation/A/B. Revision neither mutates target nor silently becomes a translation
job.

## 7. Learning eligibility

Finalization is a content boundary and stays language-neutral. Post-derived voice mutation is a separate gate:

```text
post has content language
AND post voice is active
AND machine baseline belongs to that voice
AND post.content_language == voice.source_language
```

The equality check precedes event insertion, durable enqueue, rule append, or provider call for:

- finalize-and-learn/retry;
- sentence feedback/evidence;
- revision `save_as_rule`; and
- post-backed rule comparison/profile validation input.

Mismatch is a normal `FailedPrecondition` with stable reason `VOICE_CONTENT_LANGUAGE_MISMATCH`. It creates no partial
event or evidence. The frontend can pre-disable from projections for clarity, but the backend check is authoritative.
Finalization, manual edit, copy, platform export, and explicit publishing remain available.

Learning events freeze content/source language with their post/voice/baseline revisions. A retry follows the event and
cannot become eligible because target, assignment, or browser locale later changed.

## 8. Language-aware profile analysis

The source language selects one analysis strategy inside voice.

### Korean

Retain current deterministic sentence segmentation, average character length, paragraph range, and
`다 / 해요 / 습니다 / 기타` ending distribution. Korean prompts retain the PRD-required ending priority and current
never-use/first-person analysis.

### English

Reuse punctuation/newline segmentation, add deterministic average word count, and calculate terminal cadence without
pretending Korean endings apply. The structured analyzer covers:

- formality/register and contraction use;
- preferred/avoided vocabulary and connective style;
- sentence length in words, passive/nominal tendency;
- statement/question/exclamation/fragment cadence;
- intro/closing, paragraph, heading/list, and emoji habits; and
- the same six axes.

`VoiceSyntax` gains an optional average-word measurement (or an equally explicit typed field); existing character
measurement remains for backward compatibility. UI labels/unknown values are localized, while analyzed profile values
remain in the voice source language.

Samples and authored sources entering a corpus already inherit/record the source language. Any legacy source row is
backfilled Korean. Analyze comparison, validation, rule extraction, and semantic relation prompts select their
language-specific template from the frozen source; they do not share evidence across voices or languages.

## 9. Persistence and migration

SQLite stores canonical tags with checks:

```sql
voices.source_language TEXT NOT NULL CHECK (source_language IN ('ko','en'))
posts.target_language  TEXT NOT NULL CHECK (target_language IN ('ko','en'))
posts.content_language TEXT CHECK (content_language IS NULL OR content_language IN ('ko','en'))
model_experiments.target_language TEXT
  CHECK (target_language IS NULL OR target_language IN ('ko','en'))
```

The migration backfills voices/targets as Korean. Existing content/machine baselines receive Korean provenance;
contentless drafts remain null. Frozen queued generation/write payloads without language are interpreted/backfilled as
Korean so deployment does not change their prompt behavior.

If `posts` must be rebuilt, follow the repository's established migration discipline: goose `NO TRANSACTION`, disable
foreign keys outside an explicit transaction, rebuild all constraints/indexes/triggers, run `foreign_key_check`,
commit, and restore foreign keys. A migration inconsistency fails boot ([I7]). No data or dependent aggregate may be
dropped by the rebuild.

## 10. Verification

The implementation keeps golden tests at the boundaries:

- proto enum ↔ domain tag ↔ SQL round trips, rejecting unspecified/unknown values;
- create/update presence semantics and newest-wins target saves;
- target changes preserve content/baseline/observations byte-for-byte;
- job/experiment freezing across a later target change and retry;
- atomic content + content-language persistence for ordinary/A/B/revision results;
- observation request/sub-snapshot equality across target languages;
- exact same-language/full and cross-language/portable profile snapshots;
- no cross-language sample, excerpt, lexical/endings/syntax rule, or free-form guidance in a portable prompt;
- learning mismatch creates zero events/jobs/provider calls;
- Korean measurements remain unchanged and English measurements/prompt validation are deterministic; and
- migration backfill preserves every existing post, voice, profile, job, and experiment relationship.

Frontend tests separately prove that UI locale only chooses defaults for a **new** post and never mutates an existing
post's target or content provenance.
