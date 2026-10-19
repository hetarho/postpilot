# FORMAT
> Notation rules for every doc under spec/. Write and read by these rules only. Never invent notation not defined here — define it here first, then use it.

## Principles
1. Primary reader is AI. No background prose, no fillers, no repetition. Decisions and reasons only.
2. One decision = one policy unit: a rule that can be read, applied and verified on its own. Split when two policies can change independently; keep together what must change together. Structure conditions as sub-lines or a table — never by joining them into one longer line. Removing line breaks is not concision. If something is ambiguous, don't write prose — register it as a [?] decision.
3. Short ≠ omitted. Everything needed to decide (decisions, reasons, constraints, open items) must be present.
4. Docs first. Record state changes in STATE.md before reasoning or implementing.
5. Truth order: content → ssot/*.md, progress → STATE.md, history → git log and tasks/done/ (STATE log is a hint, not truth). Fix mismatches on sight.
6. ssot/ holds outcomes only. No request traces ("user asked", "as discussed"), no interview history, no rationale beyond the decision line's ← reason, no sections beyond the skeleton. Task files are disposable and may carry request context — ssot/ never does.
7. Docs describe the current product only. When a feature leaves the product, its decisions, flows, constraints and runbooks are deleted — never kept as [x], as a retirement SSOT, or as a "migrated from / legacy / formerly / was→now" note, a past task id or a past date. History lives in git log; tasks/, review/ and ideation/ are work records and keep finished work under the rules below.

## ID
| target | form | rules |
|---|---|---|
| SSOT domain | 2-6 uppercase (AUTH, ARCH) | file ssot/<ID>.md |
| decision | <ID>-<n> (AUTH-3) | n is never reused, even after its line is deleted: a new n is the highest the SSOT has ever used + 1 (`git log -p` shows a deleted one) |
| task | T### (T012) | file tasks/T###.<slug>.md (moved to tasks/done/ at done) · slug=kebab-case · numbering=max existing+1 counting tasks/done/ · never reused |
| review finding | Fn (F3) | scoped to its review doc · n is permanent, never reused |

## Notation
- decision line: `- <ID>-<n> [o|?|x] <content>` + ` ← <reason>` only when there was a trade-off
  e.g. `- AUTH-2 [o] session: JWT 15m + refresh 30d ← minimize mobile re-login`
- decision block = the decision line plus the indented (2 spaces) sub-lines under it — sub-bullets or a table — carrying conditions, exceptions, enumerated values. Sub-lines have no ID of their own: the block is one decision, revised and referenced as one. Use them when the conditions do not fit one readable line; two reasons (` ← ` twice) or three or more ` · ` groups mean two decisions, not one long one.
  e.g. `- AUTH-4 [o] rate limit per account ← abuse without blocking shared offices`
       `  - login 5/min, password reset 3/hour`
       `  - over limit ⇒ 429 + retry-after, never a silent drop`
- [o] decided / [?] open / [x] rejected·deferred — a standing decision not to do something the product does not have; a decision whose feature was removed is deleted, not marked [x]
- change kind: + added / ✎ modified / - removed (e.g. `AUTH-2✎`)
- reference to a decision: `→!AUTH-3` requires (this rule cannot be applied without it) · `→=AUTH-3` canonical (the target owns the rule this line points to) · `→~AUTH-3` related (background, looked up on demand) · `→AUTH-3` unclassified. One marker covers a `/` list or a `–` range (`→!AUTH-3/5`, `→AUTH-3–6`); `→AUTH` is a whole domain; a bare `AUTH-3` counts as unclassified.
  - write the kind when you write or revise the decision; never reclassify old references in bulk by guessing
  - a brief pulls requires and canonical targets transitively, follows unclassified ones one hop from a cited or required decision and lists the rest, and lists related ones without their text
- finding line: `- Fn [?|o|x] P1|P2|P3 <where>: <what>` + ` ← <why it matters>`; append ` →T###` once a task exists. P1 = correctness/security risk or blocks every change · P2 = slows every change · P3 = nice to have. A functional bug is a finding whose <what> starts with `bug:`
- acceptance check: `- [ ]` open → `- [v]` done (never mark done with x — [x] means rejected in SSOT)
- rev: rN. +1 per policy change, one chg line (`- rN YYMMDD <ID>-n✎ summary`). First write: `- r1 YYMMDD initial`. A ✎ summary MUST keep the old value as `old→new` (e.g. `BM-11✎ limit 100→50`); a `-` summary states what was removed. Wording-only editing (doc-review) is not a policy change: no rev, no chg line. chg holds only changes not yet consumed into tasks (revs tasked+1..rev): create-task deletes the lines it consumes, and an empty chg is `-`
- date: YYMMDD (260905)
- task st: `todo` → `doing@date.tag` → `done@date`. Stuck: `blocked@date` (one-line reason in the task's ## result). tag = 2-4 chars chosen by the claiming session
- empty value: `-` (never leave a cell blank)
- log·chg: `- YYMMDD text`, one line each, newest on top
- flow: A → B(x|y) → C — order and branches
- commands (verify etc.): backtick code, runnable as-is, no inline commentary

## Skeletons (section order fixed; only (opt) may be omitted; no sections beyond the skeleton)
- ssot: `# <ID> <name>` / `> rN | <one-line purpose>` / `## decisions` / `## flow`(opt) / `## constraints`(opt) / `## chg`
- task: `# T### <title>` / `> st:.. | ssot:<decision IDs, space-separated> | base:<ID>@rev(per referenced domain, space-separated) | dep:T### or -` / `## goal` / `## acceptance` / `## impl notes` / `## result`(filled at done·blocked — empty means untouched)
- ideation: `# IDEATION <slug>` / `> st:.. | <one-line want>` / `## vision` / `## explored` / `## shape` / `## domains` / `## open` — file ideation/<slug>.md, slug=kebab-case
- review: `# REVIEW <slug>` / `> st:.. | scope:<paths or all> | at:<git sha7 or -> | base:ARCH@rev` / `## summary` / `## findings` / `## notes` — file review/<slug>.md, slug=kebab-case (recommended `<scope>-<YYMMDD>`)
- STATE: `## cfg` / `## ideation`(opt) / `## ssot` / `## review`(opt) / `## tasks` / `## next` / `## log`

## Reading
- Read by purpose: the rule in force → that SSOT's decisions·flow·constraints · what changed → that SSOT's chg, only the rev range you need · how something was built and verified → the one tasks/done/ file you are investigating.
- Task context comes from `vibe brief <T###>` (a work-group worker gets the same brief from its runner): the task; the current rules of each cited domain (cfg brief `domain`) or only the cited decisions (`cited`); every decision they require or delegate to; decisions that reference a cited one; the global rules (cfg always); the flow·constraints of each domain it draws from; the chg lines after a stale base — all from one source, the working tree or one commit (`--ref`). Planning asks for `<ID>-n` (with who references it), `<ID>` or `<ID>@rev` (the chg lines after rev and the decisions they changed). Open an original only for what the brief reports missing, unfollowed or related that the work needs.
- Completeness is per block: a decision block runs to the next top-level bullet or heading, and a doc without its last section (ssot `## chg`, task `## result`) has an unbounded last block. A brief is whole only when its last line `end … chars:N` is visible and it says complete:yes. A cut-off read or brief is re-read before anything is judged.
- Read one doc per call; read a large doc section by section. Editing a doc (planning, doc-review) means reading that doc whole.
- tasks/done/ is an archive of history: never read it in bulk, and never as a source of current rules (its st·base·ssot are as of completion). Open it to investigate a named task, the change behind a regression, or what was verified before. Listing filenames (numbering, dep satisfaction) is not reading them.
- reports/ is history too: open only the reports whose inbox lines are still unresolved, newest first.

## State rules
- STATE cfg `brief`: the scope of a task brief. `domain` (absent ⇒ domain): every current decision of each cited domain, whole. `cited`: only the cited decisions and what they reference — switch to it only when tasks cite every decision their implementation must obey.
- STATE cfg `always`: the global rules every task brief carries — decision IDs or whole domains, space-separated (`ARCH-1 ARCH-4 SEC`); `-` = none; absent ⇒ every ARCH decision. Keep it to rules every task must follow (conventions, test policy, verify tiers) and decide membership when an ARCH decision is added.
- STATE ssot row `id|rev|tasked|pending|[?]`: rev=current, tasked=rev consumed into tasks, pending=unconsumed delta summary (`AUTH-5+ AUTH-2✎`), [?]=open decision count. tasked < rev ⇒ create-task target.
- tasked=0 (new domain) ⇒ pending is always `all`, meaning everything up to the current rev. Later changes are absorbed into `all`.
- STATE tasks row's ssot cell holds domains only (`ARCH BM`) — decision IDs live in the task file's quote line.
- doing·done tasks are immutable. Exception: the implementing session updating its own task's st·checks·result. Content changes become a new task.
- STATE tasks table holds remaining work only (todo·doing·blocked). At done: set st `done@date` in the file, move it to tasks/done/, delete the STATE row, leave one log line. A dep absent from the table is satisfied iff tasks/done/ holds that task's file.
- Planning changes go only through update-ssot(rev+1) → STATE pending → create-task. Never edit tasks directly.
- doc-review touches ssot/ wording only: no rev, no chg, no pending, no task — so editing never invents work. Anything that changes a condition, number, negation, obligation or scope is a planning change and belongs to update-ssot; anything ambiguous stays as written and is reported.
- ideation st: `open@date` → `ready@date` → `converted@date` (in the quote line + STATE ideation row `id|st`). Explored items reuse [o]/[x]/[?]. A domains line converted into a SSOT gets `→<ID>`; the doc becomes converted when every [o] domain has one.
- review st: `open@date` → `ready@date` → `converted@date` (in the quote line + STATE review row `id|st`). Findings start as [?]; the user adopts [o] / rejects [x] ← reason. A [o] finding turned into a task gets `→T###`; the doc becomes converted when every [o] finding has one (no [o] at all ⇒ converted at once).
- review-sourced tasks: ssot = the ARCH decisions the change enforces, `-` if none; base = ARCH@rev always; first impl-notes line `- from review/<slug> Fn`; acceptance keeps behavior unchanged (existing tests still pass). They never move ssot rev/tasked/pending.
- STATE next: 1-3 lines. Every skill updates next on exit.
- STATE log: delete beyond 20 lines — history lives in git log and task files.

## Language
- Every spec/ doc is written in English. Code, identifiers, and paths stay as-is.
- cfg.lang = the user's language: interviews, confirmations, and reports happen in it. cfg.docs = doc language (default en).
- Sole exception: spec/NARRATIVE.md (written by create-narrative) is a human-facing prose doc in the user's language — FORMAT notation does not apply to it. It is a derived view of ssot/: it never introduces decisions, and on conflict ssot/ wins.
