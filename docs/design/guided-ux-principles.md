# Guided creation UX: research and application

Current product policy is defined in [THEME](../../spec/ssot/THEME.md), [AUTH](../../spec/ssot/AUTH.md), [VOICE](../../spec/ssot/VOICE.md), [EDIT](../../spec/ssot/EDIT.md) and [ARCH](../../spec/ssot/ARCH.md). This document records public-source evidence and its application. The proposed stage grouping is a postpilot design decision, not a claim that a source mandates these exact stages or that private company guidelines were accessed.

## Public sources

| Source | Source principle, paraphrased | Application |
|---|---|---|
| [Toss: eight writing principles](https://toss.tech/article/21022) | Explain the immediate next screen, remove redundant text, use familiar speech and respect informed choices. | The question, hint and action describe the chosen method; retain explicit credit estimates. |
| [Toss: six error-message principles](https://toss.tech/article/21021) | Help users understand the situation and take a useful recovery action. | Preserve input and distinguish input, network, active-work and target-conflict recovery. |
| [Toss: UX writing interview](https://toss.im/tossfeed/article/uxwriter-interview) | Writing includes content structure and shared component rules, rather than a final wording pass. | Bind focused-flow policy in SSOT and express hierarchy through reusable controls. |
| [NNG: progressive disclosure](https://www.nngroup.com/articles/progressive-disclosure/) | Reveal secondary complexity when needed; staged tasks should avoid forcing repeated movement between interdependent actions. | Separate purpose/selection/publication; keep related preview and conversation together when editing. |
| [NNG: minimize cognitive load](https://www.nngroup.com/articles/minimize-cognitive-load/) | Reduce irrelevant visual work and reuse recognizable patterns and prior input. | Preserve drafts, automatic recommended models and one active task. |
| [GOV.UK: form structure](https://www.gov.uk/service-manual/design/form-structure) | Start from one decision or question per screen and group related questions when evidence supports it. | Personal method selection is separate from writing, questionnaire answers and analysis. |
| [GOV.UK: designing questions](https://www.gov.uk/service-manual/design/designing-good-questions) | Ask for necessary information with clear reasons and understandable situations. | Keep the ten real-writing questions and explain why a pasted owner-written piece helps. |
| [GOV.UK: question pages](https://design-system.service.gov.uk/patterns/question-pages/) | Clear headings, explicit Back and retained prior answers support confidence. | Back preserves scoped draft/selection; ten answers have a known saved-count total. |
| [GOV.UK: buttons](https://design-system.service.gov.uk/components/button/) | Competing primary actions and unexplained disabled controls obscure what to do next. | Peer methods navigate directly; unavailable later analysis is absent; current input errors explain correction. |
| [GOV.UK: check answers](https://design-system.service.gov.uk/patterns/check-answers/) | Review before submission improves confidence, with contextual changes and retained answers. | Selecting or editing an AI suggestion is distinct from saving the setting. |
| [GOV.UK: error summaries](https://design-system.service.gov.uk/components/error-summary/) | Connect consistent error explanations to the relevant inputs. | Focus invalid current input after an explicit attempt and provide matching inline guidance. |
| [Apple HIG: writing](https://developer.apple.com/design/human-interface-guidelines/writing) | Use clear context-specific language and meaningful action names. | Template structure, guideline direction and writing character have different questions/examples. |
| [Material: buttons](https://developer.android.com/develop/ui/compose/components/button) | Button emphasis communicates relative action importance. | Continue/save are emphasized; Back, skip and optional changes are quieter. |
| [Material: canonical adaptive layouts](https://developer.android.com/develop/ui/compose/layouts/adaptive/canonical-layouts) | Related content can coexist in wider layouts while narrow layouts present it sequentially. | Desktop offers useful preview/work panes; mobile presents one current task with preserved state. |
| [W3C: status messages](https://www.w3.org/WAI/WCAG22/Understanding/status-messages.html) | State changes can be announced without moving focus. | Loading and saving announce truthful status; only real stage navigation moves heading focus. |
| [W3C: focus not obscured](https://www.w3.org/WAI/WCAG22/Understanding/focus-not-obscured-minimum.html) | Fixed content must not fully obscure the focused element. | Actions follow inputs; validate touch, keyboard, zoom and small-screen reach. |

## Applied journey

Personal voice learning presents a method choice, its own collection screen, a review of actual saved material/readiness, explicit analysis and confirmed use. The first screen has no disabled analysis action. Existing made voices retain further learning and private-material management as secondary actions.

AI configuration authoring presents purpose, eight comparable results, selected-result review and explicit publication. Refinement is an optional task with its preview beside the conversation on desktop. Chat is not required before saving a valid result. An existing setting loads a captured draft through an explicit no-completion action.

One current heading explains the active goal; one emphasized action explains the immediate next outcome. Back, choosing another method, more material and advanced editing have distinct lower-emphasis paths. Empty space is not filled with repeated explanations or unrelated feature lists.

## State and verification

[XState statecharts](https://stately.ai/docs), [guards](https://stately.ai/docs/guards), [React actors](https://stately.ai/docs/xstate-react), [invocation](https://stately.ai/docs/invoke) and [persistence](https://stately.ai/docs/persistence) ground the implementation. Invoked work may restart from a persisted actor snapshot, so recovery reads durable owner/session/job facts instead. A view change or actor stop never implies a provider call or server cancellation.

Acceptance checks cover stage-specific visible controls, preserved Back inputs, optional chat-free publication, explicit personal analysis, duplicate and stale events, confirmed durable recovery and owner isolation. Browser checks include both themes, phone/desktop widths, 320px reflow, 200% zoom, touch and keyboard focus. Simulated transports establish interaction behavior; they do not establish provider semantic quality or product-user research results.
