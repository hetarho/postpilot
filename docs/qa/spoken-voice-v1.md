# Korean spoken-voice creation qualification

Date: 2026-10-05. Milestone: T539. Governing decisions: DUB-27,
MODEL-79–82 and QUOTA-69/70.

## Current outcome

**Live qualification is unfulfilled.** The local process has no configured
`ELEVENLABS_API_KEY`; the root development environment has no speech credential
entry. A supplier account plan, installation-specific price evidence, approved
whole-session USD ceiling and human listening assessment have not been supplied.
No live design, confirmation or speech request was made for this qualification.
No voice or narrated-export readiness was published. T540–T549 are implemented
and automatically verified; T550 implements lifecycle qualification while its
real listening/export gates remain deferred. See
[narrated-clip-v1.md](narrated-clip-v1.md) for the separate offline evidence.

The operator harness uses production voice preparation, quotes, admission,
durable jobs, private storage and usage settlement. Its offline tests use a fake
provider and a decoded MP3 tone. They verify request binding, bounded costs,
three new speech requests across two probes, unchanged-segment reuse, stable
restart keys, private report handling and rejection of incomplete evidence.
They do not establish Korean pronunciation, naturalness or voice continuity.
T538's responsive browser review used mocked RPCs and made no supplier calls.

Local verification passed: Go formatting, vet, build and full tests; qualification
fixtures and race checks; 61 deployment tests; lint, FSD, style, publishing
retirement, development scripts, skill synchronization and generated-code
reproducibility. The unchanged frontend had 3,148 passing tests, a production
build and 30 responsive/theme browser views in T538. Spec lint has no errors;
its 39 existing warnings and 15 review hints remain unrelated to this milestone.

## Installation prerequisites

Use the same application build for the running API worker and the operator
binary, with the same database, provider configuration and private media bucket.
The CLI is a job producer: it does not start workers, sweep running work or
recover other applications' jobs. Run it from the backend working directory with
the deployment's normal environment. Configure the secret through
`ELEVENLABS_API_KEY`, never through a report, argument or committed file.

A master account must curate one current immutable speech profile in model
management. Record the exact design/synthesis model pair, connection scope,
settings, supported format, finite character/request/unit bounds and complete
current prices with provenance. Verify that the actual supplier account can
design, confirm and reuse a Korean voice and has a usable voice slot. Missing
fees, multipliers, unit evidence, capacity or connection access block the run.
This document supplies no default supplier plan, exchange rate or tariff.

The supplier's [voice design API](https://elevenlabs.io/docs/api-reference/text-to-voice/design)
returns audition samples with generated candidate identities; the
[voice creation API](https://elevenlabs.io/docs/api-reference/text-to-voice/create)
accepts the chosen generated identity to create a reusable voice. The
[timestamped speech API](https://elevenlabs.io/docs/api-reference/text-to-speech/convert-with-timestamps/)
returns audio and character alignment. These API capabilities do not prove the
quality or eligibility of a particular account/model/profile combination.

The supplier's [Voice Design cost explanation](https://help.elevenlabs.io/hc/en-us/articles/29315418701073-How-much-does-Voice-Design-cost)
describes preview-character billing for the three audition options. Establish
the applicable account tariff, units and confirmation/slot behavior from the
actual installation before approving a run. A missing charge header is not
evidence of a free operation. Check these sources again when the profile or
supplier contract changes; they were reviewed on 2026-10-05.

Start a qualification session for that profile revision through the admin UI.
It is owner scoped and expires after 24 hours. Set a finite approved USD ceiling
for the complete session. Zero is allowed only for a documented free profile;
unknown prices cannot use a zero ceiling.

## Private input and preflight

Create a private directory outside the repository and a regular input file with
permissions `0600`. Root envelope keys are lower case; domain fields use the
capitalized names shown below. Replace the owner and session placeholders with
private values. Initially leave operation IDs empty.

```json
{
  "version": 1,
  "input": {
    "OwnerID": "<master owner>",
    "SessionID": "<qualification session>",
    "DesignOperationID": "",
    "ConfirmOperationID": "",
    "FirstText": "김민지 님, 사과는 12,500원이고 밀가루는 250g입니다. OpenAI를 읽어 주세요.",
    "SecondText": "박준호 님, 우유는 1.5L예요. 오후 2시에 만나요! USB-C도 챙기세요.",
    "ChangedFirstText": "김민지 님, 사과는 13,500원이고 밀가루는 300g입니다. OpenAI를 읽어 주세요."
  }
}
```

Before creating a voice, run the read-only cost preflight:

```sh
api spoken-qualification --mode preflight --input /private/qa/input.json --output /private/qa/preflight.json
```

The preflight requires the current owned session and eligible profile. It sums
the profile's conservative ceilings for one design, one confirmation and three
later speech calls. No supplier generation is submitted. The private output
records the session ceiling and this worst-case ceiling; the latter must fit.
Reduce the explicitly curated input bounds or obtain a revised approved session
if it does not. Do not invent a cheaper tariff to make the estimate fit.

Every production start also reserves its exact maximum USD inside the voice
operation transaction. Concurrent starts share the same session ceiling. All
earlier reservations count, including failed, cancelled and uncertain work;
refunds do not automatically authorize more supplier work in that session.
An older operation without a recorded bound refuses new work in the session.
Previously stored samples remain available; a new session requires explicit
approval and fresh quotes.

## Human creation and explicit probes

Open the admin session's link to `/spoken-voices/new?qualification=…` while
signed in as its owner. Use the normal T538 flow: explicitly choose the model,
describe the voice, keep the representative Korean preview text, save, review
the design quote and approve. Listen to the samples, then select one; listening,
selection and confirmation are separate actions. Review and approve the exact
confirmation quote. Record both published operation IDs in the private input.

Compare the supplier's observed confirmation fee and voice-slot change with
the curated profile. If either is unknown or inconsistent, stop. Repeat sample
playback and verify that no generation operation, supplier request or debit is
added. Save these observations privately for the human review.

Obtain a private exact-input plan:

```sh
api spoken-qualification --mode plan --input /private/qa/input.json --output /private/qa/plan.json
```

The plan verifies the published design and confirmation, acknowledged chosen
sample, exact frozen profile and owned reusable voice. It includes all previous
session reservations plus the three uncached scripts. Inspect `plan.MaximumUSD`,
`plan.ApprovedMaximumUSD`, `plan.NewCalls` and `plan.Digest` privately. USD values
are exact decimal or rational strings; `1/1000` means USD 0.001. A fresh run has
three new speech calls. Approval must refer to this exact plan, not an earlier
model, prompt or profile variant.

After approving that concrete plan, opt in explicitly:

```sh
api spoken-qualification --mode run --input /private/qa/plan.json --output /private/qa/run.json --live --approve '<exact private plan digest>'
```

This enqueues a two-script production probe, waits for audio and known successful
settlement, then changes only the first script. The second script must reuse its
original asset and request evidence; only the changed first script may generate
new audio. The command is bounded to 20 minutes, creates no automatic paid
retry and never overwrites an output file. Its phase keys survive a restart.
On failure it retains a private partial report when possible. Preserve the
original approved plan; rerunning that same plan uses the same operation keys.
For received, uncertain, failed or cancelled work, inspect the production
operation before considering another request. Explicit recovery of known
received metadata does not call the supplier again.

Audit a completed report without new generation:

```sh
api spoken-qualification --mode audit --input /private/qa/run.json --output /private/qa/audited.json
```

Audit reloads the authoritative operations, sample hashes and decoded durations,
frozen binding, distinct request evidence and settled ledger costs. It does not
trust operation fields copied into a report. `report.ActualUSD` covers the
verified design, confirmation and three distinct speech requests in this run;
`plan.MaximumUSD` also includes any other session reservations. Character timing
is retained when supplied; it is not a claim of word-perfect synchronization.

## Listening and readiness publication

Listen to `report.Audition` and each of the three distinct `report.Samples`.
For a sample's private asset ID, the owner's authenticated
`SpokenVoiceService.GetSpokenSampleAccess` returns a short-lived playback ticket;
fetch its URL with that same authenticated session. Playback serves private
stored bytes and does not synthesize audio. Keep the audio and identifiers in
authorized private storage.

Complete `report.Review` in the private audited file after listening. It requires
`Reviewer`, an RFC 3339 `ReviewedAt` after both probes, all three booleans
`AuditionAccepted`, `KoreanAccepted`, `ContinuityAccepted`, and concrete notes
for every field below. A rejection must remain a rejection.

| Field                  | Required observation                                                       |
| ---------------------- | -------------------------------------------------------------------------- |
| `Names`                | 김민지 and 박준호 are intelligible and correctly pronounced.               |
| `PricesNumbers`        | 12,500원, 13,500원, decimals and the time are correctly read.              |
| `Units`                | 250g, 300g and 1.5L are intelligible in context.                           |
| `Punctuation`          | Stops, question/statement rhythm and exclamations are suitable.            |
| `MixedLanguage`        | OpenAI and USB-C are intelligible without dropped syllables.               |
| `OmissionsRepeats`     | Compare the complete script; record any omissions or repetition.           |
| `Clipping`             | Check beginnings, ends, distortion and audible discontinuities.            |
| `Continuity`           | Compare all scripts with the chosen audition for the same perceived voice. |
| `ConfirmationCapacity` | Record observed confirmation fee, unit evidence and voice-slot behavior.   |

Only after the listening criteria and supplier accounting pass may the operator
publish the reviewed report:

```sh
api spoken-qualification --mode publish --input /private/qa/reviewed.json --live
```

Publication reaudits the real artifacts and ledger, checks review ordering
against saved operation timestamps, and records the private report's SHA-256 as
the voice-creation evidence for that exact profile revision. It does not publish
narrated-export readiness. Preserve the reviewed file as the private evidence
referenced by that hash. Record only a dated nonsecret outcome and limitations in
this document; never paste handles, account/request IDs, keys, cookies, live
prices or audio into the repository.

## Evidence status

| Criterion                                                   | Status on 2026-10-05                                                |
| ----------------------------------------------------------- | ------------------------------------------------------------------- |
| Real design → human audition → exact confirmation           | Not run; credentials/account/approval missing.                      |
| Two later Korean scripts and changed-segment reuse          | Not run live; production-job fixture test passes.                   |
| Korean intelligibility, omissions, clipping and continuity  | Unverified; human listening required.                               |
| Actual documented units, confirmation fee and slot behavior | Unverified for an installed supplier account.                       |
| Replay adds no call/debit                                   | Covered by offline playback/reuse tests; live observation required. |
| Whole-session and concurrent-start ceilings                 | Verified by real SQLite fixture tests.                              |
| Profile voice/export readiness                              | Neither was promoted by this work.                                  |

T539 can become complete only when the real criteria pass. T550 separately
qualifies preview and both narrated MP4 export paths.
