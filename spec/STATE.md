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

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- create-ssot per domain in MIGRATION.md §5 order: AUTH QUOTA POST VOICE GEN MODEL TEMPLATE GUIDE EXPORT PUBLISH LANG THEME MARKETING
- create-task for the domains that carry pending legacy work (PUBLISH QUOTA GEN MODEL MARKETING)

## log
- 260905 ARCH r1 no-op (documents the shipped architecture; no code impact)
- 260905 create-architecture start (spec init; migrating from the legacy spec tree — see MIGRATION.md)
