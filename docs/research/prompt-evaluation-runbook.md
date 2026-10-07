# Reproducible writing-request evaluation

The offline harness uses the application's current composers and fixed synthetic material. It opens no account storage, reads no deployment credentials and makes no provider calls. Its contract checks establish composition and parser behavior; they do not establish source truth, good prose or a better instruction language. Korean instructions remain the production default.

## Reproduce the offline evidence

Use the versions pinned in `.node-version` and `backend/go.mod`, with the existing lockfile dependencies. From the repository root:

```sh
(cd backend && go run ./cmd/api prompt-inventory) > /tmp/postpilot-prompt-inventory.json
(cd backend && go run ./cmd/api prompt-evaluation) > /tmp/postpilot-prompt-evaluation.json
(cd backend && go test ./internal/generation ./internal/llm -count=1 -timeout 30m)
(cd backend && go test ./cmd/api -run 'Test(PromptInventory|PromptEvaluation|AuthoringInventory)' -count=1 -timeout 15m)
```

The inventory supplies actual ordered application fragments or typed native speech fields, activation conditions, composer/source links and the requested output schema, parser and consumer. Media bodies, signed links and native supplier handles are omitted. Current origin-aware requests and retained admitted post paths have separate prompt/schema identities. This is a current code inventory, never reconstructed historical dispatch.

Keep each report's source-file, composition, schema and fixture hashes alongside the exact source revision. An uncommitted report is tied to those hashes rather than its parent commit alone. Repeating the command on identical source produces the same synthetic evidence. [The current request map](../design/prompt-map-2026-10-07.md) identifies the owning runtime paths. Older audit measurements remain dated findings about their recorded source versions.

[Recorded offline evidence](prompt-evaluation-2026-10-08.json) contains 67 stage/mode/protocol contracts, 71 inventory scenarios, 48 paired instruction conditions and 440 passing deterministic checks, bound to 107 source-file hashes. The full requests and fixed response bytes are reproduced by the commands above; the stored evidence retains their identities and results without duplicating all prompt text. Runtime usage, usable responses, truncation, origin coverage, human source/prose/voice review and publication observations remain unverified. The recorded source revision is the task's parent with dirty-source metadata; exact source hashes bind the implemented candidate.

## Controlled instruction-language condition

`korean-baseline` uses the unchanged Korean composer. `english-common-instructions` replaces only declared code-owned common task/format fragments through an internal developer compiler option. Both conditions preserve the same Korean material, names, numbers, accepted voice and endings, examples, stock/owner policies, output target, attachment references, output schema and synthetic model/budget conditions. The comparison does not use `LanguageEnglish`, translate material, back-translate a result, add a user language choice or add a MODEL-30 writing-test factor.

The fixed material includes explicit owner facts, visual interpretation, selected memory meaning and AI style impressions, unknown objective/chronology claims, repeated quotation and emoji, typed template literal/topic/scoped-fact boundaries, sparse tags and a narrow edit request. Expected-source labels are declared review material. Fixed synthetic responses exercise the real parsers and range/reference checks; they are not model responses. One deliberately unsupported ranking phrase has a structurally compatible memo reference, demonstrating that a valid label and range do not establish semantic support. The harness does not infer whether a generated claim is true.

## Evidence units and human review

| Field                        | Evidence and interpretation                                                                                                                                       |
| ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Characters / UTF-8 bytes     | Exact application text measured locally; media bodies and transport wrappers are excluded.                                                                        |
| Reference-token estimate     | Named application estimate, separately labeled; not a provider tokenizer or actual bill.                                                                          |
| Actual usage / cost          | Unknown offline. A live witness and normal owner usage ledger are required; native speech uses its own admitted billing unit.                                     |
| Usable response / truncation | Unknown without a response. Parsing a fixture establishes its structure only; record refusal, empty output, parse failure and truncation separately on live work. |
| Origin coverage              | Structurally mapped readable ranges; repeated quotes, Unicode offsets and source references must validate. Coverage does not certify semantic support.            |
| Semantic support             | Human assessment against the supplied source labels, including unsupported chronology, objective assertions and impressions; not an automated judge.              |
| Expression / voice           | Human review of naturalness, Korean endings, accepted voice, repetitions, requested scope and useful arrangement.                                                 |
| Publication observations     | Owner-recorded manual publishing/view observations, separate from prose preference and causal or exposure claims.                                                 |

For a future admitted trial, conceal condition/model labels during the initial prose review and keep paired materials fixed. A reviewer records the concrete phrase, supporting source or missing evidence, severity and proposed owner edit. Review names/numbers, taste/experience versus visible interpretation, temporal claims, mixed-origin phrases, tag relevance up to the cap, explicit output language, style-only examples and unchanged revision scope. Evaluate naturalness and accepted voice independently of JSON validity. Do not calculate an automated winner, statistical superiority or search-performance claim.

## Explicit live admission

The shipped command is offline only. `prompt-evaluation --live` refuses; adding execution flags or an owner/model/budget does not turn it into a dispatcher. No authenticated instruction-variant live executor is supplied, and no live language or semantic result is claimed. Creating the offline harness authorizes no paid call.

Before any future live evaluator is implemented or run, record an explicit owner approval, curated eligible model references, the exact source/schema/fixture/condition versions, maximum calls, completion caps and a finite approved usage budget. It must use the normal authenticated generation/writing-test admission, estimate, owner usage metering, immutable snapshots and private capture path. It must not call an SDK directly, use BYOK, bypass free-capacity eligibility or substitute a synthetic answer for unavailable execution. Instruction language remains an internal evaluation option, outside the product's closed single-factor test choices; the existing product UI cannot execute this language comparison.

For ordinary owner-controlled baseline observations, the existing estimate/confirmation flow admits generation or a MODEL-30 test. Keep that result distinct from an instruction-language comparison. Missing owner authentication, credentials, budget, eligible model/free capacity, usable response or human review is an unmet/unverified live gate. Preserve uncertainty after an issued call; never replay a paid request to repair missing evidence.

## External trial worksheet

Copy this table into an owner-controlled document. It is not product telemetry and creates no automatic publication, traffic collection or search API.

| Trial | Source revision + hashes | Material/condition | Owner/model | Approved call and usage bounds | Actual usage/cost | Usable/truncated | Origin coverage | Human support findings | Human expression/voice findings | Owner edits | Manual publication/date | Owner-observed views/date | Limits |
| ----- | ------------------------ | ------------------ | ----------- | ------------------------------ | ----------------- | ---------------- | --------------- | ---------------------- | ------------------------------- | ----------- | ----------------------- | ------------------------- | ------ |
| —     | —                        | —                  | —           | —                              | Unknown           | Unverified       | Unverified      | Unreviewed             | Unreviewed                      | —           | —                       | —                         | —      |

Views are descriptive owner observations. A change in views does not establish that a prompt condition caused it. QUAL metrics and their product-owned bands remain separate from origin labels and this trial.
