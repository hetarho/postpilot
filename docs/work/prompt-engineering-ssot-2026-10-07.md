# Prompt engineering SSOT conversion and implementation handoff

Date: 2026-10-07. Planning change in the main checkout, with no current work attempt. Product code and task files were not edited. This document is a derived handoff; `spec/ssot/` is authoritative.

## Accepted behavior

- Preserve useful long instructions while correcting duplicate context, data/instruction boundaries, stage relevance and contradictory output contracts.
- Review all human-readable post text by semantic phrase: owner input, visual interpretation or AI-added meaning; failed/missing classification is unconfirmed review state.
- Allow visible expressive/sensory AI proposals while retaining supplied meaning and objective-fact safeguards. Neither source color nor plan approval certifies truth.
- Never derive event chronology/movement from photo upload/storage/file/capture order. Real observed video sequence remains evidence.
- Treat tag_count as a 1–10 maximum, default 4; accept fewer/none without padding. Tag quality stays a switchable basic guideline. Stage pruning applies declared code-owned rules; arbitrary scoped owner rules remain verbatim and are not automatically filtered.
- Provide real source/composition inventory and safe own-request inspection, distinguishing current/prepared/captured states and preserving blind identity/retention.
- Keep Korean instructions as the baseline; evaluate instruction language with material/style/output/model conditions held fixed.

## Revision map

| Domain | Revision | Main delta |
|---|---|---|
| GEN |25→26| Evidence contracts, visible AI expression, photo chronology, cap/preservation, stage schemas and evaluation |
| POST |35→36| Phrase/source review, current-result lifecycle, maximum tag UI and private request inspection |
| GUIDE |16→17| Guideline authority, stage applicability, chronology/impression text and tag ownership |
| TMPL |24→25| Literal/instruction/fact boundaries, raw technical-view exception and maximum tag seeds |
| VOICE |14→15| Style-only evidence limits |
| MEM |6→7| Approved facts versus inferred meaning, frozen evidence and explicit approval |
| MODEL |34→35| Inventory, prepared/captured safe requests, blind details, source application and payload expiry |
| EDIT |3→4| Kind/mode-specific composition and bounded latest-draft authority |
| LANG |8→9| Korean baseline and controlled instruction-language evaluation |
| THEME |28→29| Accessible default origin review and separate technical detail |
| EXPORT |10→11| Review/technical metadata excluded from canonical outputs |
| MKT |9→10| AI-assisted speed with owner control, only for shipped behavior |
| QUAL |7→8| Product-owned limit attribution; source labels are not quality/accuracy scores |

No new SSOT domain, output language, model-test factor, paid repair action, automatic publisher, search API or infrastructure was introduced. Existing AUTH/QUOTA/CLIP/ARCH contracts remain authoritative. Unrelated THEME/CLIP/CDS/MEM/INFRA open items remain open.

## Audit disposition

The 31 candidates in [the audit](../research/prompt-audit-2026-10-07.md) remain evidence, not 31 independent implementation assignments.

| Candidates | Disposition |
|---|---|
|1–9,11| Material boundaries, LIST/VIDEO shape, explicit priorities, maximum tags, stage relevance and requested-only revision adopted in GEN/GUIDE/TMPL |
|10,12–15| Instruction/output independence adopted; production instruction baseline, naturalness rules, sparse-material composition bands and stored revision nouns retain current behavior pending evaluation |
|16–21| Kind/mode context, discoverable ownership/roles, units and prepared-versus-issued capture adopted; active path consolidation belongs to existing authoring work |
|22–29| Global stage inventory/contract parity includes existing video language, facts, frozen-story, guideline, speech/caption and consumed-result responsibilities; video chronology, timing qualification and admitted correction rules remain intact |
|30–31| Memory material boundaries and actual-path documentation adopted; existing memory-language question is not silently resolved |

Wire shape, annotation representation, phrase alignment, storage/indexes, exact color tokens and compatible adapter handling are engineering decisions. Deterministic reference checks cannot establish semantic accuracy. Live low-cost model success, human prose/source review and later owner-published views are validation, not missing product choices.

## Existing work and freshness

Development continues on main, one dependency-ready task at a time, under ARCH-70. Read STATE, the selected task and current SSOT before implementation; complete its acceptance, verification, result and main commit before proceeding.

T622/T624/T625/T629 provide the existing shared contracts, creation/history, authoring and test presentation. Their implementation and prior checks do not establish this newer semantic-origin contract. Baseline consistency, local-edit revision fencing and saved-target draft recovery are corrected during the main merge and remain pinned by regressions.

Next `create-task` must reconcile the pending prompt-engineering changes with remaining T623/T626/T627/T628/T630/T631 requirements and dependencies. Reuse their applicable work rather than creating duplicate implementations. Preserve the new SSOT decisions and historical implementation evidence; prior passing tests are not evidence of new origin behavior.

## Verification

This is a documentation-only planning delta. Appropriate checks are spec lint, decision/reference/revision/STATE consistency, local document links and whitespace validation. No product runtime, migration, dependency or deployment changed; implementation behavioral suites and full pre-push CI/media gates belong to their later stages under ARCH-24/31.

- PASS: `pnpm exec haeram-spec-creator lint` (exit 0): 23 SSOTs, 27 remaining tasks, 594 completed tasks. It reports 124 warnings and 13 editorial hints. Warnings include existing incomplete history/result entries and the expected stale task bases after this revision; they are not a runtime behavioral pass and are not suppressed by editing task files.
- PASS: focused document consistency check against Git HEAD/history: thirteen rev increments, 26 modified decisions and 25 new decisions; no reused historical IDs or removed decisions; exact STATE/pending/chg agreement; valid new decision references and local links; converted ideation and twenty-line STATE log.
- PASS: `git diff --check`; task files, product code and AUTH/QUOTA/CLIP/ARCH remain unchanged by this conversion.
- Independent policy reviews checked generation/guideline/template consistency, frontend/export lifecycle and cross-domain privacy/retention. Corrections preserve the existing options API, verbatim owner rules and content/input revision distinction.

No provider call, private live post read, push or deployment occurred. Live origin accuracy and model/language quality remain unverified; implementation tasks must perform impact-selected behavioral checks.
