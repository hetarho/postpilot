# Language policy

## Vocabulary and supported values

PostPilot supports exactly Korean (`ko`) and English (`en`). The public protobuf enum is
`UNSPECIFIED = 0`, `KOREAN = 1`, and `ENGLISH = 2`; owning backend contexts persist only checked
`ko|en` tags. `UNSPECIFIED` represents absence at a wire boundary and is never a business-language
fallback.

Four values are intentionally independent:

- the browser-owned **UI locale** controls interface copy and formatting;
- a post's **target language** controls its next full generation or write A/B run;
- a post's nullable **content language** records the provenance of its current machine content; and
- a voice's immutable **source language** declares the language of its evidence and profile.

PostPilot neither detects nor translates authored text. Regional language variants, account-synced
locale preferences, and a multilingual sub-profile inside one voice are out of scope.

## Browser locale

The locale is resolved synchronously before React renders:

1. an exact valid value in `localStorage['postpilot.locale']`;
2. the first supported primary tag in `navigator.languages`; or
3. Korean.

Storage failures are ignored. A locale change updates i18next, `<html lang>`, title, description,
and locale-aware date, relative-time, and number helpers; it then attempts to persist the canonical
tag. It changes no URL, session, query cache, post, voice, or server preference. The app-drawn locale
menu uses a 44px icon trigger and `menuitemradio` options labelled with the autonyms `한국어` and
`English`; it marks the active option with a check and announces a selection politely. The control
appears on login, the authenticated shell, and the public `/about` header through the same shared
preferences widget rather than being forked per shell.

Resources are bundled TypeScript catalogs with no runtime fetch or translation service. Their exact
namespace topology is `common`, `auth`, `nav`, `posts`, `voices`, `purposes`, `guidelines`, `models`,
`publishing`, `errors`, `plans`, and `marketing`. The `marketing` namespace carries every visible
label, accessible name and metadata string of the public `/about` page at full ko/en parity, and
that page's locale changes never alter its URL — there is no locale prefix and no automatic locale
redirect ([public-marketing](public-marketing.md)). Korean is also the static/no-JavaScript document
fallback, and that fallback is product-level: it is neither route-specific nor localized, because it
is the only metadata a client that refuses JavaScript ever sees. The installed product contract is i18next 26.4.0 and react-i18next 17.0.12; no detector or
HTTP backend is installed.

## Post language transitions

The 목표 언어 picker lives in the editor's writing brief, beside the models, the voice, the 용도 and
the target length ([posts.md](posts.md) *Editor presentation*); its frozen-job note and its
mismatch note are unchanged by that placement.

`SavePostDraft` requires a concrete target on create. On update, an absent target preserves the
stored value and a present valid target replaces it through the same newest-wins queue as title and
memo. The draft request carries the latest complete title/memo snapshot; target presence does not
turn it into a sparse text patch. A target selection preserves content, observations, revisions,
machine baseline, status, finalization, voice/purpose assignment, and content language.

Ordinary generation and write A/B freeze the current target before provider work. Successful full
generation or winner application atomically writes canonical content, its machine baseline, and the
frozen target as content provenance without overwriting a newer post target. Revision freezes the
existing content language and never uses a later target as its output language. Manual edits,
target selection, finalization, copy/export, and publishing preserve provenance. Consequently an
older frozen run may legitimately leave `target_language != content_language`.

Standalone HTML and Markdown exports derive language metadata from content provenance, never from
the newer target. Platform exports preserve content bytes and do not translate them.

## Voice source and prompt projection

Voice creation requires an explicitly present supported source language. The wire field is optional
only so absence can be distinguished from enum zero; the service rejects both. Existing voices and
the bootstrap default voice are Korean, and no RPC changes source language after creation.

For equal source and target languages, prompt projection includes the complete structured profile,
legacy/manual guidance, active and manual rules, and ranked excerpts. Cross-language projection is
tagged with source, target, and `portable=true` and includes only:

- intro and closing pattern;
- paragraph sentence-count range;
- heading, list, and emoji habits; and
- involvement, narrativity, persuasion overtness, abstractness, addressee focus, and humor axes.

It excludes lexical and ending/register fields, language-specific syntax and connectives, free-form
guidance and rules, contrast rules, samples, authored excerpts, and translated substitutes. Korean
analysis retains deterministic ending measurements. English uses its own prompt/schema and measures
word length, register and contractions, connectives, passive/nominal tendencies, cadence, structure,
and the six axes. Corpora contain evidence declared for that voice's immutable source language only.

## Learning safety

Every post-backed learning boundary requires content provenance, an active assigned voice, a matching
machine-baseline voice, and:

```text
content language == frozen source language == active voice source language
```

Finalize-and-learn/retry, sentence feedback, revision `save_as_rule`, rule comparison, and profile
validation reject a mismatch before an event, rule, evidence row, job, or provider call is created.
Finalization itself, manual editing, copy, every export, and explicit publishing remain available.

## Stable failures

Synchronous Connect errors attach exactly one `AppErrorDetail` with an allowlisted
`UPPER_SNAKE_CASE` reason and display-safe string params. Missing, duplicate, unknown, or malformed
details and missing/extra params become localized `UNKNOWN_FAILURE`; raw Connect messages are never
translation input. Durable work exposes `Failure { reason, params, technical_detail }`. The product
explanation comes from the stable reason, while optional inert diagnostic text is available only
behind the labelled technical-detail disclosure.

Stores encode params as a JSON object, reject malformed/non-object structured rows, project legacy
raw errors as `UNKNOWN_FAILURE` plus technical detail, and clear all failure fields together on a
retry or success. Language-boundary reasons are `POST_TARGET_LANGUAGE_REQUIRED`,
`POST_TARGET_LANGUAGE_UNSUPPORTED`, `VOICE_SOURCE_LANGUAGE_REQUIRED`,
`VOICE_SOURCE_LANGUAGE_UNSUPPORTED`, `CONTENT_LANGUAGE_REQUIRED`, and
`VOICE_CONTENT_LANGUAGE_MISMATCH`.

## Persistence and configuration

Migration `0012_languages_and_failures.sql` adds checked language provenance and structured durable
failures, deterministically backfills existing Korean data, and leaves contentless drafts without
content provenance. Language/fallback/storage/catalog/prompt/projection values are code and product
contracts, not deployment knobs. Job 32 adds no environment value, network service, language-only
job kind, detector, or translation provider.
