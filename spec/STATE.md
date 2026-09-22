# STATE
> spec control tower. Every agent: ① read this file before working ② write the start to log BEFORE questioning/reasoning/implementing ③ reflect every state change here immediately.
> State only. Content truth: ssot/. Task detail: tasks/. Notation: FORMAT.md.

## cfg
- level: mid
- lang: ko
- docs: en

## ideation
| id | st |
|---|---|
| clip-source-observation-visibility | converted@260912 |
| clip-template-as-preset | converted@260917 |
| post-quality-and-related-links | open@260918 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 9 | 9 | - | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 19 | 19 | - | 0 |
| POST | 9 | 9 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 8 | 8 | - | 0 |
| MODEL | 16 | 16 | - | 0 |
| TMPL | 8 | 8 | - | 1 |
| GUIDE | 3 | 3 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 17 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 43 | 40 | CLIP-13✎ | 1 |
| CDS | 25 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 1 | 1 | - | 2 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta
- publishing retirement implementation is complete; DEPLOY.md records the prod checkpoint and receipts at c6af0418, with staging not deployed
## log
- 260922 CI guard repaired; completed and pending retirement guides both pass while missing cleanup evidence arguments and restored runtime fail; 11 regressions and the real retirement gate pass locally; remote verification awaits push
- 260922 CI investigation start; run 35739196970 failed after the deployment-checkpoint documentation commit
- 260922 T321 done; short email-free seed ids and independent automatic-login/saved-id controls; 2328 FE tests, full BE gate, codegen and CI support checks pass (lgn)
- 260922 T321 claimed (lgn)
- 260922 create-task AUTH complete; T321 consumes AUTH r8
- 260922 create-task AUTH start; AUTH r8 login convenience and development accounts
- 260922 update-ssot AUTH complete; AUTH r8 records short email-free seed ids and independent automatic-login/save-id choices; no active tasks overlap
- 260922 update-ssot AUTH start; short dev seed login ids, opt-in automatic login and saved login ids
- 260922 T320 done; the backend gate carries a 30m bound and now reports per-package results (gate)
- 260922 found with T320: internal/clip/store fails TestTheSoundSettingDoesNotInvalidateAnInterruptedCandidate in a package run and passes alone; it was invisible while the package only reported a timeout, and needs its own task
- 260922 T320 claimed (gate)
- 260922 create-task ARCH complete; T320 created from r9
- 260922 update-ssot ARCH complete; ARCH r9 gives the backend gate a 30m bound because cmd/api (1018s) and internal/clip/store (828s) pass the Go default on their own; create-task pending
- 260922 update-ssot ARCH start; the backend verify command expires on Go's per-package default
- 260922 Jev applicability research complete; recommend an offline Korean draft-grounding evaluation first, clip candidate scoring second; official docs and live OpenRouter catalog verified, decisions excluded from current modality query; no inference calls or product/spec decisions changed
- 260922 Jev applicability research start; verify official capabilities against current Postpilot implementation
- 260922 ARCH r8; dev-only fixture tooling placement and its production-image exclusion recorded, implemented ahead of the record in 63f41404 so the delta is already consumed and owes no task
- 260922 obsolete T008 and T177 task records removed; publishing retirement superseded the live smoke and the owner cancelled the remaining manual clip-viewing follow-up
- 260922 T279 done; surviving BlockType, ModelPurpose and language mirrors are generated-enum pinned, shared transport conversion is centralized, and retirement absence remains enforced (enm)
- 260922 T279 claimed (enm)
