# Tech — UI localization and error contract

The implementation contract for [plan 13](../plan/13.multilingual-interface-and-target-langua.md): how a static Vite
SPA chooses its UI language, owns translations without changing URLs, and turns backend failures into localized copy
without treating backend prose as an API.

This document owns the technical mechanism. Product rules—supported languages, browser ownership, and which actions
are allowed—land in `spec/policy/languages.md` when the plan is implemented.

## 1. Boundaries

UI locale answers **which language Postpilot's interface uses**. It does not answer which language a post is written
in; that contract is [content language and voice projection](content-language-and-voice-projection.md).

The runtime supports exactly:

```ts
type Locale = "ko" | "en";
```

The static SPA owns locale resolution and translation. The API does not inspect `Accept-Language`, localize Connect
messages, store an account preference, redirect to a locale URL, or render frontend markup. User-authored values,
generated content, observation facts, model names, filenames, and external diagnostic text are data and remain
verbatim.

## 2. Locale resolution

Resolution is synchronous and deterministic:

```text
1. Read localStorage["postpilot.locale"].
2. If it is exactly ko or en, use it.
3. Otherwise inspect navigator.languages in order and take the first supported primary subtag.
4. Otherwise use ko.
```

Tags are normalized only for matching: case-insensitive, with everything after the first `-` ignored. Thus `en-US`
matches `en`; an unsupported `ja-JP` does not. A malformed storage value is ignored, not repaired. The canonical
value written after an explicit selection is always `ko` or `en`.

The storage key and fallback are product contracts rather than deployment tuning. No env variable controls them.
Storage access is guarded so privacy mode or a denied storage API falls back to browser preference without preventing
the app from mounting.

Locale is resolved in the entry path before `createRoot`. The same resolved value initializes i18next and immediately
sets:

- `document.documentElement.lang`;
- the localized `<title>`;
- the localized meta description; and
- the locale consumed by date, relative-time, and number formatters.

This ordering prevents the first accessible tree from claiming Korean while rendering English. The static
`index.html` retains Korean as its no-JavaScript/failure fallback.

An explicit switch calls one operation that changes i18next language, updates document metadata/`lang`, writes the
preference, and notifies React. It never navigates or invalidates server/query data.

## 3. i18next integration and catalog ownership

Job 32 followed `/library-setup` against the implementation-time official React/Vite documentation and locks
`i18next` 26.4.0 plus `react-i18next` 17.0.12. Only those two runtime packages are needed. Resources are bundled;
`i18next-browser-languagedetector` and `i18next-http-backend` would duplicate the resolver and add a network failure
mode, so they are not installed.

Initialization and catalog configuration belong to the app provider segment:

```text
frontend/src/app/providers/i18n/
  index.ts
  locale.ts
  metadata.ts
  resources/
    ko/
    en/
```

Namespaces follow product surfaces rather than FSD slices: `common`, `auth`, `nav`, `posts`, `voices`, `templates`,
`models`, `publishing`, `errors`, and `marketing`. Keeping them in the app provider gives one complete audit surface
and avoids making lower slices export configuration back upward. Lower layers import `useTranslation` from the
third-party package; they do not import `app/providers`.

Keys express meaning, not source sentences:

```text
auth.login.submit
posts.language.currentMismatch
errors.VOICE_CONTENT_LANGUAGE_MISMATCH
```

Interpolation carries dynamic user/data values. Pluralization is handled by i18next rules; components do not build
sentences by concatenating fragments. React performs output escaping, and translations never opt into unescaped HTML.
Rich copy uses component composition/`Trans` only where links or emphasis are semantically required.

TypeScript resource augmentation derives valid keys from the canonical resource shape. A topology test walks every
namespace and requires Korean and English to have identical leaf keys and compatible interpolation placeholders.
Missing-key logging may assist development, but CI is the enforcement boundary. Korean is the runtime fallback for
an unexpected missing key; a key name is never deliberately shown to users.

Long marketing copy belongs to the `marketing` resource namespace. The future `/about` page composes translation
keys rather than defining a second locale runtime or embedding duplicate Korean/English objects in the page slice.

## 4. Formatting and document behavior

Formatting helpers accept locale explicitly or read the initialized locale through one stable adapter. They do not
instantiate a file-scope Korean formatter as the current relative-time helper does.

- Absolute dates use `Intl.DateTimeFormat` with the active locale.
- Relative times use `Intl.RelativeTimeFormat`; thresholds remain existing product behavior, not translator copy.
- Counts/costs use `Intl.NumberFormat`; persisted numeric values do not change.
- Stored RFC3339 timestamps and UTC/server semantics remain unchanged.

`<html lang>` changes on every selector action. Focus stays on the selector, the current route/search/hash stays
unchanged, and a polite status announces the selected language. The selector labels options with autonyms (`한국어`,
`English`) so either locale remains discoverable.

The control lives in `features/change-locale`. Login, the authenticated shell, and the public header compose it. It is
not a route, settings page, domain entity, or bottom-navigation destination. Generic select/popover primitives remain
in `shared/ui`.

## 5. Stable application failures

Connect status codes are too coarse for localized copy, while raw error strings are unstable and can leak internal
or provider details. The transport adds a stable reason detail:

```proto
message AppErrorDetail {
  string reason = 1;
  map<string, string> params = 2;
}

message Failure {
  string reason = 1;
  map<string, string> params = 2;
  string technical_detail = 3;
}
```

`AppErrorDetail` belongs to synchronous Connect failures. `Failure` belongs to durable rows/projections such as
generation jobs, experiment candidates, voice comparisons/validations, and publishing jobs. Reasons are stable
UPPER_SNAKE_CASE identifiers in a backend registry, for example:

```text
VOICE_NAME_TAKEN
VOICE_CONTENT_LANGUAGE_MISMATCH
POST_BUSY
MODEL_RATE_LIMITED
MODEL_OUTPUT_INVALID
UNKNOWN_FAILURE
```

Handlers still select the semantically correct Connect code (`InvalidArgument`, `FailedPrecondition`, `NotFound`, …)
and attach one reason detail. The raw Connect message is a safe English operator fallback and is never the primary UI
copy. Domain behavior returns typed/sentinel errors; the RPC edge maps them to Connect code + reason. Domain packages
do not import proto/connect types.

`params` contains display-safe structured values needed by a translation, such as `max`, `actual`, or a stable model
label. It does not carry SQL, stack traces, prompts, private post text, cookies, provider keys, or arbitrary HTML.
Each reason has a documented allowed parameter set. Missing/extra parameters cannot turn into markup or execute code.

The frontend's generated-code boundary in `shared/api` parses details into:

```ts
interface AppFailure {
  reason: string;
  params: Readonly<Record<string, string>>;
  technicalDetail?: string;
}
```

Presentation translates `errors.${reason}`. An unknown reason, missing/duplicate detail, malformed parameter set, or
legacy row maps to localized `errors.UNKNOWN_FAILURE`, regardless of the Connect code. Code must not recover meaning
by substring-matching `rawMessage`.

The same parser is used by mutation controls and route failures. The root route owns an eagerly available localized
error component so even a lazy-route exception discards arbitrary `error.message`, offers an explicit router
invalidation retry, and cannot fall through to TanStack Router's diagnostic default UI. Device decoding/direct upload
network classification may remain local, but Connect create/confirm failures retain their structured app detail.

## 6. Durable failure persistence

App-owned durable failures persist reason and params as first-class fields. `params` is encoded as a JSON object at
the store edge and decoded/validated before entering the domain projection. A separate technical detail may retain a
provider/agent message useful to the operator. It is never the only user explanation and is shown, if at all, behind
a labelled technical-detail disclosure.

Migration keeps legacy raw error columns long enough to read old rows. Old non-empty failures map to
`UNKNOWN_FAILURE` plus their existing text as technical detail. New writes populate structured fields. Once every
reader is structured, deprecated raw wire fields are no longer rendered and may be removed by a later explicit
change rather than silently retemplated.

The failure owner decides the reason:

- `internal/llm` normalizes provider classes; job/experiment maps them to product reasons.
- job owns interruption/panic/missing-handler reasons.
- post/voice/template own validation and lifecycle reasons.
- publishing owns agent/lease/ambiguous-commit reasons.

Cross-context code does not parse another context's prose.

## 7. Testing contract

The localization job proves:

- storage > `navigator.languages` > Korean precedence, including malformed/denied storage;
- primary-subtag matching and URL/session stability;
- initialization updates `lang`, metadata, and resources before the first app render;
- catalog key and placeholder parity across Korean/English;
- every locale selector mount uses the same feature and persists one canonical value;
- dates, relative times, counts, validation, status, and accessibility labels change locale;
- every registered backend reason maps to both catalogs;
- unknown/legacy errors use a localized fallback and never render raw detail as the sole message;
- technical detail is escaped and absent unless explicitly available;
- frontend lint/FSD/style/build and existing Korean behavior remain green.

Review includes all current routes at 360px and desktop widths. English can be wider by word count; Korean remains the
layout stress case for non-breaking labels. Neither locale may force a nested scroller or shrink a touch target.
