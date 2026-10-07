# Clip observation and composition boundary

The actual request inventory is `RequestCompositions`, assembled by the same
builders used by `Service.ObserveChunk`, `Flow`, `Storyline`, `Narrate`,
`SpokenScript` and `Revise`. It reads synthetic material only, performs no model
call, and includes every base mode and its bounded correction variant. Prompt
versions identify the owning stage contract; schema versions hash the declared
output contract. Captured inspection uses the exact issued application request.

| Modes                                                            | Owner and output                                                                                                                                 | Parser and consumer                                                                                           |
| ---------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------- |
| `observe`                                                        | `BuildObservePrompt`: factual full-chunk segments with local source milliseconds; project language for descriptions and verbatim recorded speech | `parseChunk`, then complete source-time analysis merging; no editing decision                                 |
| `storyline`, `storyline-revision`                                | `BuildStorylinePrompt`: ordered plan paragraphs, observation IDs and generated intro/outro slot words                                            | `parseStoryline`, reviewed project storyline; no cut or caption                                               |
| `flow`, `flow-measured-speech`                                   | `BuildFlowPrompt`: direct storyline/slot draft and ordered real-source cuts                                                                      | `parseFlowPlan`, validated edit plan; duration/rates/transitions are resolved locally                         |
| `flow-follow-storyline`, `flow-follow-storyline-measured-speech` | `BuildFlowPrompt`: only cuts from held observed scenes; immutable reviewed story and measured speech when present                                | `parseFlowPlan`; retained story and slot words come from the frozen input                                     |
| `flow-revision`                                                  | `buildFlowRevisionPrompt`: complete replacement footage flow without a replacement story or slot draft                                           | `parseFlowPlan`, then visible-caption or dedicated-spoken revision as admitted                                |
| `narration`, `narration-revision`                                | `BuildNarrationPrompt`/`buildNarrationRevisionPrompt`: visible captions and placement of declared captions on the resolved output timeline       | `parseNarration`, validated independently editable captions; no spoken synthesis                              |
| `spoken-script`                                                  | `BuildSpokenScriptPrompt`: direct storyline/slot draft plus exact natural-speed `spoken_lines`                                                   | `parseSpokenScript`, private spoken draft before separately admitted synthesis and footage selection          |
| `spoken-script-follow-storyline`                                 | `BuildSpokenScriptPrompt`: only `spoken_lines`, keeping reviewed story/slot words                                                                | `parseSpokenScript`, retained reviewed story with the new spoken draft                                        |
| `spoken-script-revision`                                         | `buildSpokenRevisionPrompt`: only the targeted dedicated spoken script                                                                           | `reviseSpoken`, narration correction; displayed captions, audio speed and untargeted words remain independent |

Every writing request names the admitted `Language` target directly. Code-owned
material contracts distinguish explicit owner directions, template form/generated
entry instructions, exact field facts, item associations, observations, retained
documents and measured speech. Substituted facts in generated caption entries are
emitted as typed instruction/fact parts, so imperative-looking answer text never
becomes template direction. Owner video guidelines remain in their frozen order;
new stock metadata selects only declared stage/output responsibilities. Historical
untyped defaults remain intact without matching their prose to today's registry.
Observation receives no writing guidelines.

Composition resolution is local, not another model call. `composition.Parse`
reads the frozen document, `clip.ResolveSelectedComposition` delegates to
`composition.Resolve` for supplied fields/items/cuts, and the AI parsers admit the
result under the existing narrowing and timing rules. There is no current
`Service.Plan`, legacy non-composition writer or composition-plan output schema;
missing composition input is refused before model execution.

New response contracts request consumed fields. Flow duration is computed from
source ranges, allowed fixed rates and transition overlap. Caption identities are
minted by the server, and narration cannot replace its supplied flow. Local
parsers still accept the original typed `duration_ms`, caption `id` and narration
`cuts` echoes; ignored cuts keep the existing notice. Frozen spoken responses can
omit discarded storyline/slot output and still admit correctly typed historical
responses. Required identities, units, references, coverage and timing constraints
remain explicit and validated. A text script never predicts TTS duration.

`completeValidated` preserves the frozen selected model, per-call limits, credit
ceilings and admitted correction count (at most three additional attempts).
Unparseable/schema-invalid responses and admitted observation validation failures
may retry; readable repairable composition violations are handled locally.
Cancellation, exhaustion, truncation, refusals, pricing, transport, authored-input,
media and insufficient-footage failures stop without fallback. Correction feedback
contains allowlisted checks and bounded numeric measurements, never raw candidate
output. Every correction inventory entry is derived from its original request.

Rendering workflows, real-source chronology, endpoint qualification, numeric fact
policies, timing validation, presets and media behavior stay with their existing
owners. Captions receive no post phrase-origin colors. Prompt/schema tests prove
request structure and local consumer compatibility, not model prose quality or
production voice readiness; no provider calls are used for this inventory.

## Private failure diagnostics

Failed clip jobs log `job`, `kind`, `reason`, and known `stage`. Strict provider
failures may additionally carry code-owned operation/error class, status codes and
validated request IDs. Ordinary logs include no provider prose, raw candidate,
prompt, media path/link, credential, authorization header or arbitrary header.
Usage and credit settlement remain the existing admitted accounting behavior.
