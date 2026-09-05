# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 1 | 1 | - | 0 |
| AUTH | 1 | 1 | - | 0 |
| QUOTA | 2 | 1 | QUOTA-14✎ QUOTA-32+ QUOTA-33+ | 0 |
| POST | 1 | 1 | - | 0 |
| VOICE | 1 | 1 | - | 1 |
| GEN | 1 | 1 | - | 0 |
| MODEL | 2 | 1 | MODEL-46+ MODEL-47+ MODEL-48+ | 0 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- create-ssot per domain in MIGRATION.md §5 order: AUTH QUOTA POST VOICE GEN MODEL TEMPLATE GUIDE EXPORT PUBLISH LANG THEME MARKETING
- create-task for the domains that carry pending legacy work (PUBLISH QUOTA GEN MODEL MARKETING)

## log
- 260905 create-ssot MODEL r1 (shipped) + r2 (legacy change 29) — pending for create-task, after QUOTA
- 260905 create-ssot GEN r1 (shipped, no pending work)
- 260905 create-ssot VOICE r1 (shipped; one open decision VOICE-7 on retiring 규칙으로 저장)
- 260905 create-ssot POST r1 (shipped, no pending work)
- 260905 create-ssot QUOTA r1 (shipped) + r2 (legacy change 28) — pending for create-task
- 260905 create-ssot AUTH r1 (shipped, no pending work)
- 260905 ARCH r1 no-op (documents the shipped architecture; no code impact)
- 260905 create-architecture start (spec init; migrating from the legacy spec tree — see MIGRATION.md)
