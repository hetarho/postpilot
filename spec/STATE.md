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
| post-quality-and-related-links | converted@260923 |

## ssot
| id | rev | tasked | pending | [?] |
|---|---|---|---|---|
| ARCH | 11 | 9 | ARCH-52+ ARCH-53+ ARCH-56+ ARCH-57+ | 0 |
| AUTH | 8 | 8 | - | 0 |
| QUOTA | 20 | 20 | - | 0 |
| POST | 15 | 15 | - | 0 |
| VOICE | 3 | 3 | - | 1 |
| GEN | 12 | 12 | - | 0 |
| MODEL | 17 | 17 | - | 0 |
| TMPL | 11 | 11 | - | 1 |
| GUIDE | 6 | 6 | - | 0 |
| EXPORT | 5 | 5 | - | 0 |
| PUB | 6 | 6 | - | 0 |
| LANG | 5 | 5 | - | 0 |
| THEME | 18 | 15 | THEME-19✎ | 0 |
| MKT | 6 | 6 | - | 0 |
| VIDEO | 3 | 3 | - | 0 |
| CLIP | 45 | 40 | CLIP-13✎ CLIP-163+ CLIP-111✎ CLIP-116✎ CLIP-165+ | 2 |
| CDS | 26 | 23 | CDS-17✎ CDS-19✎ CDS-21✎ CDS-84✎ CDS-86..99+ CDS-9✎ CDS-14✎ CDS-20✎ CDS-22✎ CDS-32✎ CDS-44✎ CDS-52✎ CDS-70✎ CDS-71✎ CDS-72✎ CDS-73✎ CDS-75✎ CDS-76✎ CDS-77✎ CDS-79✎ | 1 |
| BILL | 4 | 4 | - | 0 |
| MEM | 3 | 3 | - | 2 |
| QUAL | 4 | 4 | - | 0 |
| GIFT | 2 | 2 | - | 0 |

## review
| id | st |
|---|---|
| diff-260908 | converted@260908 |
| clip-project-update-260914 | converted@260914 |
| clip-failure-visibility-260914 | converted@260914 |
| clip-release-smoke-260914 | converted@260916 |
| arch-260919 | converted@260919 |
| publishing-260922 | converted@260922 |
| published-quality-260924 | converted@260925 |
| clip-narrate-failure-260926 | converted@260926 |

## tasks
| id | title | ssot | dep | st |
|---|---|---|---|---|

## next
- create-task CDS CLIP (CDS r26, CLIP r45): intro/outro presets fit their slots by width and stack by gaps, 8 intro / 7 outro presets with new-project defaults A/B, radial scrim, ① numbered slot drawings; CDS r24–r25 and CLIP r41–r44 deltas stay pending beside them
- review/clip-narrate-failure-260926 is complete (T398, T399): a narration caption fits its named style and the tail trim keeps the 1.2 s cut floor; unmeasured: a rapid phrase in a hook-role style against the canvas width
- GIFT r2 + QUOTA r20 are implemented (T394–T397): operator vouchers issued at /admin/vouchers, redeemed from the public /gift page; next ARCH edit adds `voucher` to ARCH-5's context list; BILL still carries the Toss placeholder and USD pricing (see payment research) before any card rollout
- POST r15 is implemented (T392, T393): /posts pages with server-side search/filter and restores its scroll; ListClipProjects still answers whole and can follow the same shape when needed
- Later: update-ssot CLIP-163, then create-task ARCH CLIP for real-GPU validation, profile approval/automatic selection and concurrency tuning; ARCH r11 and CLIP r44 remain partially consumed, and physical host setup/migration remain operator actions
- The review wave T354..T375 (review/published-quality-260924) is complete, one commit per task (p42); create-task MKT THEME for /about overflow and CLIP CDS THEME for the unconsumed Wanted Sans delta

## log
- 260926 update-ssot CDS CLIP done (rp): CDS r26 — region slots fit their width (shrink to a role floor, then two lines for display/headline/hook/title) instead of CDS-20 counts, blocks stack by fixed ink gaps around their anchor, presets fix size and position only (CDS-88), intro 8 / outro 7 presets (CDS-89..99), radial scrim for centred blocks; CLIP r45 — CLIP-111 lists and new-project defaults A/B with existing projects keeping theirs, CLIP-116 region limit is width, CLIP-165 ① shows numbered slot drawings; no doing tasks affected
- 260926 update-ssot CDS CLIP start (rp): intro/outro presets fix only slot size and position; titles fit the 856 px measure by shrinking to a floor then wrapping instead of per-role character counts; intro 8 / outro 7 presets with project defaults A and B; waiting on hc's uncommitted CDS/CLIP cleanup before editing
- 260926 T399 done (nf): the tail trim stops each cut at the 1.2 s cut floor and falls back to the overlap floor only for an overrun the floors cannot absorb; the multi-source pin moved 15760→15630; BE gate green
- 260926 T399 claimed (nf)
- 260926 T398 done (nf): narration captions are admitted against their named style's line bound (short text, then omission, never a refused clip), the writer is told each style's bound, and a narration refusal names the caption rather than line 0; BE and FE gates green
- 260926 T398 claimed (nf)
- 260926 create-task review/clip-narrate-failure-260926 done: F1 F2 → T398 (narration bound is the named style's, caption refusal names the caption), F3 → T399 (tail trim keeps the 1.2 s cut floor)
- 260926 review clip-narrate-failure-260926: prod job b752f893 died at narrate on narration-2's style bound (copy_limit, line 0) and carried a 1 ms tail cut; the owner asked for the fixes
- 260925 T397 done (gv): /admin/vouchers tab issues from presets or custom numbers (sold/given, message), copies the gift link, lists every voucher with sale/state/redeemer/remaining, revokes behind a confirm; FE gate green
- 260925 T397 claimed (gv)
- 260925 T396 done (gv): public /gift/$token page (GetVoucher view, 받기 for a session, log in/sign up with redirect otherwise), pending gift in localStorage handed back by the authenticated layout; strings under plans (gift.*, redeemVoucher.*); FE gate green
- 260925 T396 claimed (gv)
- 260925 T395 done (gv): voucher context + VoucherService (GetVoucher public, RedeemVoucher session, Issue/List/Revoke master), vouchers table 0085, VOUCHER_* reasons 244..248 with FE copy; BE/FE/codegen gates green
- 260925 T395 claimed (gv)
- 260925 T394 done (gv): voucher lot kind (0084), expiring-first consumption order, OpenVoucherLot/ExpireVoucherLot/VoucherLotStandings in usage, 이용권 label in the account menu; BE/FE/codegen gates green; T395 impl notes now name VoucherLotStandings
- 260925 T394 claimed (gv)
- 260925 create-task GIFT QUOTA done: GIFT r2 + QUOTA r20 → T394 (voucher lot kind, expiring-first order), T395 (voucher context/VoucherService/table), T396 (public /gift page, redeem, sign-in hand-back), T397 (admin vouchers tab); GIFT tasked=2, QUOTA tasked=20
- 260925 create-task GIFT QUOTA start (GIFT r2 all, QUOTA r20: QUOTA-9✎ QUOTA-12✎ QUOTA-58+)
- 260925 update-ssot QUAL dropped: no SSOT or code change; QUAL stays r4
- 260925 update-ssot QUOTA done: r20 QUOTA-12✎ every expiring lot (monthly, voucher, expiring bonus) burns first by expiry, then bonus, then purchased; QUOTA-9✎ voucher redemption joins the credit paths; QUOTA-58+ voucher lot kind; GIFT r2 constraint points at it; no doing tasks affected
