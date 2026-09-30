# QUOTA plans, credits, metering
> r26 | Paid plans grant daily AI credits and monthly bonuses with model and server-export entitlements; free models cost no credits, while paid AI work reserves and settles confirmed usage at a job-frozen KRW conversion. Billing and cash refunds belong to BILL.

## decisions
- QUOTA-1 [o] every account carries exactly one plan `free | light | basic | pro | max | master`; free is the provisioning default, light/basic/pro/max are paid offers and master is operator-only. Stored plans and wire mappings reject unknown values without renumbering existing enum identities.
- QUOTA-2 [o] a commercial tier decides daily credits, monthly bonus, cumulative model access and monthly successful server-export allowance (→QUOTA-7); master alone grants administrative access. Automatic publishing is unavailable to every tier.
- QUOTA-3 [o] subscriber changes through BILL and master-only support assignment (`AdminService.SetUserPlan`, `api setplan`) use the same entitlement rules (→QUOTA-35 →QUOTA-42). Support assignment never charges a payment method or creates a second grant for an unchanged period.
- QUOTA-4 [o] the last master cannot be demoted on either path; the guard is a condition inside the UPDATE statement, not a count taken before it ← two concurrent demotions must not both commit
- QUOTA-5 [o] the acting plan is resolved once per request on the session path, from the row (→AUTH-15)
- QUOTA-6 [o] one integer product credit represents KRW 1 of AI cost at the disclosed job-frozen service rate (→QUOTA-59), not a model token or the retail purchase price. Preserve provider micro-USD usage separately; fixed plan grants and KRW checkout prices do not fluctuate with FX.
- QUOTA-7 [o] the code-owned offer is:
  | tier / ko name | monthly KRW | annual KRW | daily credits | monthly bonus | paid model ceiling | monthly server exports |
  |---|---|---|---|---|---|---|
  | free / 무료 | 0 | - | 0 | 0 | none; free models only | 0 |
  | light / 라이트 | 1900 | 19000 | 15 | 290 | value | 2 |
  | basic / 베이직 | 4900 | 49000 | 45 | 510 | balanced | 6 |
  | pro / 프로 | 9900 | 99000 | 85 | 1070 | premium | 15 |
  | max / 맥스 | 29900 | 299000 | 235 | 3170 | top | 60 |
  - paid grades are cumulative; all tiers may use separately managed free models (→MODEL-57 →MODEL-68)
  - pro alone is recommended; master is not sold and keeps QUOTA-16's operator exemption
  - daily and monthly figures are separate entitlements, never an immediately spendable monthly sum; BILL owns charges and subscription coverage
- QUOTA-8 [o] every number in the ladder is a code-owned product rule in `backend/internal/plan`, published to the client; the frontend hardcodes none ← two deploys must never disagree
- QUOTA-9 [o] signup and payment-method registration grant no credits. Credits arise from paid daily/monthly grants, purchased packs, voucher redemption, fault compensation (→QUOTA-60) or an explicit operator grant; free-model access requires no credit grant.
- QUOTA-10 [o] charge a user-visible AI job once: `C = ceil(sum(confirmed_job_cost_usd) × applied_krw_per_usd)` using QUOTA-59's snapshot.
  - sum all of the job's billable calls before rounding; no fixed base or multiplier; zero confirmed cost yields zero
  - missing pricing is not a free price, and unconfirmed usage is not replaced with the reserved maximum (→QUOTA-21 →QUOTA-48)
  - bounded clip debit remains capped by QUOTA-44; failures and cancellation follow QUOTA-46 and QUOTA-49
- QUOTA-11 [o] a balance is lots, not a counter: one `credit_lots` row per grant (granted, remaining, `expires_at` NULL = never), `CHECK (remaining >= 0 AND remaining <= granted)`, spend and refund statements guarded by the amount available ← the never-negative guarantee rests on SQL, not on Go alone
- QUOTA-12 [o] consume expiring non-purchased lots in earliest-expiry order, then non-expiring promotional lots, then purchased lots; break ties by creation.
  - daily, monthly bonus, voucher and compensation credits participate by their actual expiry, so a sooner-expiring voucher may precede a daily grant
  - expired remainders stay in history and never roll over; unused daily allowance does not accumulate and requires no login claim
  - paid grants renew once per eligible window under QUOTA-37, atomically with balance/admission operations
- QUOTA-13 [o] every LLM-consuming job start (`generate` `revise` `extract_memory` `analyze_voice` `check_voice` `model_experiment`, and a post's storyline jobs →GEN-68 →GEN-69) passes one gate at the shared enqueue seam (`job.Queue.Enqueue`, a consumer-declared Admitter port wired in `cmd/api`); one A/B comparison is one admission even though it fans out to two candidates; clip preparation is the explicitly bounded exception in QUOTA-43
- QUOTA-14 [o] hold: price every planned call at its worst case — an assumed 30 000-token prompt (`holdInputTokens`), except clips freeze each stage's enforceable input allowance under QUOTA-53, at the model's input price plus that call's own completion budget at its output price — run the total through the charge formula, and deduct it from the lots in consumption order, inside one `BEGIN IMMEDIATE` transaction with the admission row, before the job row exists except for QUOTA-43's clip preparation; the owning service states the call count ← only it knows that observation batches photos per call
- QUOTA-15 [o] terminal settlement is once-only against the persisted usage ledger; return unused reservation to its original lots and preserve their original expiries (→QUOTA-61).
  - a non-clip overrun may spend remaining eligible admission-period funds down to zero without debt, never a new period's grant
  - clips retain QUOTA-44 ceilings; unconfirmed cost is absorbed, with no retroactive debit
  - compensation is a separate, once-only credit under QUOTA-60, never a second unused-reservation return
- QUOTA-16 [o] master records admissions and reference AI usage but spends no credit lot and is not refused for model-grade access or insufficient credits. Operator exemption changes neither provider limits nor compatibility, bounded execution, price safety or the 60-second output limit.
- QUOTA-17 [o] a hold whose job row then failed to insert is released in full on a context that outlives the caller's cancellation; a boot sweep settles any hold left open behind an already-terminal job
- QUOTA-18 [o] a refusal is `resource_exhausted` with reason `INSUFFICIENT_CREDITS` carrying `required`, `balance`, `renews_at` (RFC3339); it writes no admission or debit and creates no job row except that QUOTA-43 preserves the existing clip preparation job as failed; the client renders copy from the reason and formats machine values (micro-USD, instants, tier names) with i18next, never from the message string
- QUOTA-19 [o] paid model access follows the purpose-specific registered grade and QUOTA-7's tier ceiling; sufficient credits are a separate admission check.
  - free accounts use only separately managed free models, even when they retain purchased or voucher credit
  - free models are available to all tiers at zero credit cost and zero balance; provider restrictions still apply (→MODEL-68)
  - no credit purchase or voucher unlocks a grade or server exports; each enabled paid feature retains a compatible entry-tier model
  - no fixed model count or access to the complete provider catalog is promised
- QUOTA-20 [o] model responses distinguish plan entitlement, required plan, compatibility and credit affordability. Higher grades remain visible and locked; insufficient balance or a downgrade never deletes a saved selection or history. New work requires an explicit eligible selection; no silent substitution (→MODEL-24 →MODEL-25).
- QUOTA-21 [o] ledger: every server-side LLM call writes one `usage_events` row — prompt/completion tokens, provider-reported reasoning tokens (0 = not reported; kept for diagnosis, never re-priced), `cost_microusd`, `cost_source` resolved reported → estimated → unavailable; a failed call is recorded when the provider reported usage; rows are append-only and kept indefinitely
- QUOTA-22 [o] recording happens at the llm boundary: `cmd/api` wraps the registry every context receives and the worker stamps `usage.Work{user, kind, job}` on the handler context ← every present and future call site is metered by construction
- QUOTA-23 [o] the ledger `stage` is the stage the call named for itself (`observe` · `write` · `analyze`), falling back to ref inference; A/B candidate calls appear in both the experiment tables and the ledger; credit math never joins experiment internals
- QUOTA-24 [o] a post holds at most 30 photos (`UPLOAD_MAX_PHOTOS_PER_POST`, in both config owners); the server refuses the upload that would cross it and the browser reports the excess as skipped before decoding ← it bounds the worst case a hold must price, not storage
- QUOTA-25 [o] master-only surfaces are the AdminService procedures, including account administration and estimator-combo assignment (→QUOTA-39), in the closed masterProcedures set (→AUTH-18); the frontend hides /admin and redirects non-master visitors away from it while the server remains authoritative; no tier exposes a publishing entry, panel or pairing screen
- QUOTA-26 [o] GetMyPlan publishes the acting tier, owned credit lots and expiries, spendable credit, separate daily/bonus entitlements and next reset instants, monthly export used/reserved/remaining counts and renewal, the five offers with monthly/annual KRW prices and recommendation, and estimator rates.
  - a balance read materializes only the current eligible daily/monthly grants, once; it does not grant missed days
  - a free account can inspect retained balances but cannot spend them on a paid model or buy/redeem new credit without paid coverage
- QUOTA-27 [o] the header credit control links to `/plans` and reads GetMyPlan; the account popover explains daily allowance, monthly bonus, purchased/voucher/compensation lots, expiries and next daily/monthly resets separately. Free accounts see free-model access without a fictitious credit reset or unlimited-use claim; master is labelled as operator-exempt.
- QUOTA-28 [o] `/plans` compares free/light/basic/pro/max and is the subscription entry.
  - show monthly or annual KRW billing, the full annual upfront charge and any clearly approximate monthly equivalent; pro alone is recommended
  - each offer shows daily credits, monthly bonus, eligible model groups and monthly server exports separately, plus the 60-second cap and one applicable action
  - stack on phones and reflow five offers without horizontal scroll; retain THEME-37/41's promotional primitives
  - current-tier management and renewal/cancellation/refund details live on BILL's screen; About mirrors the offer without checkout (→MKT-5)
- QUOTA-29 [o] insufficient credit blocks cost-incurring AI work, not compatible free-model work. Manual editing, reading, preview and existing-result download remain available; a new server export independently requires its export entitlement (→CLIP-193).
- QUOTA-30 [x] mid-flight cancellation on exhaustion, per-model or per-stage rate limits, charging non-LLM resources (storage, uploads), retroactive usage reconstruction, team plans — out of scope; payments and the subscription lifecycle are BILL's
- QUOTA-31 [o] renewal and reset instants use calendar Asia/Seoul as a fixed UTC+9 constant, not a tzdata lookup ← the distroless image ships no zoneinfo and KST observes no DST
- QUOTA-32 [o] a planned call carries the completion budget it will actually be sent ← spend follows per-stage budgets (observe from batch size, write/revise from target length up to a ceiling), so a hold pricing every call at `LLM_MAX_TOKENS_DEFAULT` would over-hold observation and under-hold long writes
  - `PlannedCall` carries that figure beside `Ref` and `Count`
  - `worstCaseMicrousd` prices each entry at its own figure
  - a ref used for two stages with different budgets is two entries (entries with equal budgets collapse into one)
  - `CreditsFor` (behind the catalog's `RequiredCredits` and `EstimatePostCredits`) prices with the same per-call budgets, naming the stage it quotes for
  - non-clip `holdInputTokens` is 30 000, clips follow QUOTA-53, and a call declaring no budget uses a completion fallback
- QUOTA-33 [o] hold pricing (QUOTA-32) changes no provider request body and no budget any stage is sent
- QUOTA-34 [o] active paid subscribers may buy one of three fixed packs: KRW 3000 → 1000 credits, KRW 9000 → 3000, KRW 30000 → 10000.
  - no bonus or expiry; purchased credit is consumed last and survives subscription expiry, but does not itself unlock paid models or server exports
  - purchases change neither tier nor reset/billing anchors; BILL owns checkout, payment and cash refunds
- QUOTA-35 [o] paid upgrades keep the existing daily, monthly-benefit and payment anchors.
  - after successful prorated payment (→BILL-5 →BILL-18), raise model rights immediately
  - add `ceil(monthly_bonus_difference × remaining_benefit_month_fraction)` credits expiring at that month's existing end, and `floor(server_export_difference × that_fraction)` counts to the same month
  - today's daily grant is unchanged; higher daily credit starts at the next existing 24-hour boundary
  - a repeated upgrade starts from the then-current tier, without replaying earlier grants; it clears a scheduled downgrade
  - downgrade/cancellation preserves paid access and remaining grants through purchased coverage, then applies the lower/free offer; purchased credit is preserved
- QUOTA-36 [o] production counts are labelled estimates based on eligible assigned pairs and stated item conditions, not entitlements. A monthly illustration states its assumed number of daily grants plus the monthly bonus; daily credit is never represented as available upfront. AI clip estimates and included server-export counts are distinct.
- QUOTA-37 [o] paid daily grants use fixed 24-hour half-open windows from the paid-subscription start instant, independent of login or inactivity.
  - bonus/export periods use anchored calendar months in Asia/Seoul, preserving the original payment day and time; a missing day clamps to month-end and later restores the original day
  - grant only once per eligible window, including coincident daily/monthly boundaries; unused daily and monthly amounts expire with no carryover
  - free accounts have no daily credit grant or free-credit reset clock; failed renewal and paid expiry stop paid grants
  - a new paid subscription after lapse starts a new anchor; ordinary upgrades and renewals never move it
- QUOTA-38 [o] monthly and annual subscriptions carry identical daily and monthly entitlements. Annual payment buys twelve calendar months, with daily grants and one monthly bonus/export allowance at each eligible benefit window, never twelve bonuses or quotas upfront (→BILL-4).
- QUOTA-39 [o] four estimator combos are the model levels `value` · `balanced` · `premium` · `top` (가성비 · 밸런스 · 고급 · 최고, →MODEL-57)
  - each card offers and accepts only a `photo-analysis` registration and a `writing` registration carrying that same per-purpose level, with the server enforcing both registration and level
  - the operator assigns the pair through the third AdminService procedure, and a combo left unassigned, or whose registration is deregistered or relevelled, is left out of the client response rather than falling back ← quoting real prices means the comparison moves with the operator's curation, and an invented price that never moves would be the alternative
- QUOTA-40 [o] publish estimator blog rates per photo/video/1000 finished characters and clip rates per source/finished second, from current catalog prices, the disclosed rounded FX and code-owned token assumptions including 1.5x generation tokens for AI edits. Any estimated base represents model work, not a fixed credit surcharge. Missing prices/rate yield unavailable estimates; actual job admission and settlement remain authoritative.
- QUOTA-41 [o] `/plans` starts with blog-post/clip basis selection and a read-only per-item condition summary; one bottom-right floating "Change conditions" button opens a shared sheet (bottom sheet on mobile, dialog on desktop) containing only per-item inputs, never a model selector or production counts; blog inputs are characters/photos/videos and clip inputs are original-video count and finished-clip duration; the two sets persist independently, switching basis restores its conditions, and every card updates immediately without a request per change
- QUOTA-42 [o] a successful first subscription or re-subscription opens a new paid anchor with the full first daily grant, monthly bonus and monthly export allowance, once. Existing purchased credit and valid non-subscription lots keep their identities and expiries. Paid-to-paid changes follow QUOTA-35 without reopening a fresh cycle.
- QUOTA-43 [o] clip generation creates a durable job, validates retained recovery work and verifies only missing source-analysis work before reserving its remaining AI calls including the frozen response-correction allowance; reservation is atomic and within the approved ceiling, and refusal permits zero AI calls and preserves completed work
- QUOTA-44 [o] clip AI consumes only reserved model refs, call counts and input/completion budgets; only bounded response correction under QUOTA-54 may repeat a call, with no model or supplier fallback or budget increase, and final debit never exceeds reservation or approval while service-owned overage cannot create debt
- QUOTA-45 [o] each clip AI approval binds the source selection, settings, models, frozen budgets, correction allowance, selected reusable checkpoint and the storyline it builds from; only remaining AI work is quoted, changed inputs invalidate approval, and compatible render-only continuation requires no AI reservation
- QUOTA-46 [o] failed AI work charges only confirmed billable usage under QUOTA-10/21; no confirmed usage means zero. Provider fault bears no compensation, while service fault or unknown cause receives QUOTA-60 compensation. Clips additionally retain QUOTA-44 ceilings. Missing usage stays unknown and is never later reconstructed from the reservation.
- QUOTA-47 [o] every clip provider request is restricted to compatible input/processing paths whose applicable text, visual, audio and output prices are covered by the frozen admission and enforceable request limits; a nonzero media price is eligible when covered, unknown or unenforceable applicable pricing is refused rather than treated as free, automatic supplier fallback and silent limit relaxation are forbidden, and the absence of an eligible path stops the attempt without issuing that call ← a provider unit-price limit cannot by itself guarantee a total user-credit ceiling
- QUOTA-48 [o] a clip quote accounts for the documented billing units of its bounded text, visual and audible inputs and generated output, including applicable reasoning, caching and conditional prices, without double-counting overlapping usage; optional billable features outside the quoted work remain disabled, and an estimated final cost uses only sufficiently specified reported usage rather than charging the reserved maximum as if it were measured use

- QUOTA-49 [o] accepted owner cancellation charges confirmed AI usage C only, with no additional cancellation fee. Return unused reservation under its original expiry; clips cap C at their reserved and approved amount. Cause/settlement races follow QUOTA-52.
- QUOTA-50 [o] approval, cancellation and accounting surfaces explain confirmed usage only, ordinary reservation returns and any separate half-cost compensation. Show usage debit, compensation amount/expiry and resulting net credit effect distinctly where applicable; raw provider usage is never rewritten as compensation or a cancellation fee. Operator shadow amounts are explicitly not debited.
- QUOTA-51 [o] cancellation before reservation or without confirmed billable usage costs zero; render-only cancellation spends neither AI credits nor a successful server-export count. Master records reference usage without credit debit or a compensating credit lot.
- QUOTA-52 [o] accepted cancellation is a durable settlement cause, including after restart; cancellation/completion races and repeated requests settle once, already-completed jobs keep their normal settlement, and unconfirmed in-flight provider usage never causes a later extra debit

- QUOTA-53 [o] clip approvals freeze 30 000 input tokens per observation request and 64 000 per composition request, including correction attempts; quote/reservation, endpoint qualification and request admission share that allowance, and a frozen policy carrying no input allowance is held to 30 000 and cannot be enlarged silently

- QUOTA-54 [o] clip quotes reserve at most four requests per missing observation chunk and at most four for a missing composition (one initial plus three response corrections); a frozen policy carrying no correction allowance allows one, every issued request is independently metered, reused work is never charged again, and unused correction allowance is refunded under the settlement and cancellation rules

- QUOTA-55 [o] sales copy distinguishes KRW 1 AI-cost denomination from the fixed retail top-up packs and distinguishes daily allowance from monthly bonus. Annual pricing states ten monthly payments for twelve months (about 16.7% off twelve monthly payments), not a 20% discount. No misleading at-par purchase or immediate monthly lump-sum claim is shown.
- QUOTA-56 [o] estimator levels remain value/balanced/premium/top in order. A level outside the compared tier is locked with the required tier, not shown as usable production; a missing/unpriced/incompatible eligible pair has an unavailable explanation. Free models are described with provider-limited availability, never an infinite job estimate from dividing by zero.
- QUOTA-57 [o] clip estimates cover source analysis plus flow/narration writing from the assigned photo-analysis/writing pair, requiring video input and structured output for the observer and structured output for the writer. Originals assume 60 seconds each, visibly stated; count is 1..20 and finished duration 15..60 seconds. An estimate is neither provider qualification nor an approved reservation and changes no production quote or settlement policy.
- QUOTA-58 [o] an eligible voucher redemption opens one voucher lot expiring its stated validity after redemption (→GIFT-9); revocation voids only its unspent remainder (→GIFT-10). Subscription lapse does not pause that expiry or grant paid-model access.
- QUOTA-59 [o] select the previous business day's published KRW/USD reference in Asia/Seoul; `applied_rate = ceil(reference_rate / 10) × 10` KRW per USD.
  - snapshot the source, publication date, reference and applied rate before paid AI work starts; keep it through the admitted job’s internal retries and settlement, and disclose reference versus applied rate; a new admitted job selects its own snapshot
  - when the required rate cannot be verified, use the last confirmed publication only if its date is at most seven calendar days old, with a temporary-rate notice
  - without an eligible rate, refuse new cost-incurring AI work; free-model work, existing frozen jobs and fixed-KRW purchases are unaffected
  - subscription prices, purchased packs and grant counts never change with this rate
- QUOTA-60 [o] service-fault or unknown-cause failure issues `ceil(C / 2)` compensation credits from the confirmed usage debit C, once per job; C=7 yields 4 credits and net burden 3.
  - the compensation is a separate lot valid for seven days from issuance, even if the originating daily/monthly lot expired
  - consume it by expiry order; ordinary unspent reservation returns separately without new expiry
  - zero confirmed charge means zero compensation; retain raw supplier cost and display compensation separately
  - this grants credit, not cash or model rights; cash refunds remain BILL-11's
- QUOTA-61 [o] an admitted job retains the credit/export period secured at start across daily/monthly resets and paid-coverage expiry; newly issued grants are untouched. Unused reserved credit retains its original expiry and does not revive if expired. New jobs and explicit new retries require current entitlement; settlement never charges the same job twice.
- QUOTA-62 [o] successful server exports use a separate monthly allowance under QUOTA-7 and CLIP-193/194, not AI credits. Monthly benefit renewal replaces unused counts without rollover, including during annual coverage; extra-credit purchases and vouchers buy no export counts.
- QUOTA-63 [o] a master account leaves master only through another master's support assignment (`AdminService.SetUserPlan`, →QUOTA-4) or the shell `api setplan`; no other path moves it ← an operator must never lose administrative access to their own click, and the shell stays the recovery path
  - `SetUserPlan` targeting the calling account is refused server-side with its own reason, whatever tier is requested and however many masters exist
  - 계정 관리 shows the caller's own row as a fixed tier label with no tier control; hiding the control is an affordance, the server refusal is the rule
  - no billing write (subscription start, change, renewal, lapse, scheduled change, cancellation end, refund reversal) changes a master account's tier (→BILL-20)

## flow
- paid subscribe → BILL confirms payment → first daily grant + monthly bonus/export window; daily access → materialize the current eligible daily grant once; monthly boundary → expire old bonus/counts and open new entitlements while paid
- AI start → entitlement/compatibility check → free-only work(zero-credit admission) | paid work(freeze FX → estimate → reserve eligible lots) → admitted job → metered calls
- clip AI start → approved frozen ceiling → durable preparation/recovery validation → reserve remaining bounded calls → execution, or refusal with no AI call
- terminal job → once-only confirmed usage debit → return unused original reservation → service/unknown failure(separate seven-day ceil-half compensation) | provider failure/owner cancel(no fee/compensation)
- paid subscriber → pack purchase or explicit voucher redemption → independent lot with its own expiry policy
- upgrade → prorated successful payment → model access + current-month bonus/export deltas → higher daily grant at next existing reset
- downgrade/cancel → keep purchased coverage → lower/free offer; server render admission/completion follows CLIP-194
- tier write on a master account → another master's SetUserPlan(last-master guard) | `api setplan` | self-assignment(refused) | any billing write(tier stays master)

## constraints
- product constants belong to `internal/plan`, not environment variables; AI completion budgets and upload limits retain their owning-context configuration
- source provider costs remain append-only and distinguish reported, estimated-from-reported-usage and unavailable; no estimate from a reservation is billed as observed usage
- clip execution retains QUOTA-43–48 and QUOTA-53/54: frozen call/model/input/completion/correction budgets, no silent fallback or unreserved call; observation/composition completion caps remain 8192/32768 tokens including applicable reasoning
- the server owns quote/approval/reservation ceilings: forged client ceilings or catalog changes never authorize unreserved work; compression savings never justify lower reservations or more calls without a separately validated budget change
- applicable text/visual/audio/output pricing must be known and bounded; price safety never becomes a later user overcharge
- admission, lot mutation, compensation and export settlement are idempotent under retries and concurrent completion; no transaction spans a provider call
- master remains exempt from AI lot debit and model-plan refusal, not from provider limits or bounded execution
- schema/placement: plan rules and RPCs in `backend/internal/plan`; product-agnostic metering, reservations and lot ledger in `backend/internal/usage`; auth owns account plans; clip owns export reservations; billing composes paid coverage through consumer-owned transaction ports
- frontend reads contracts through `entities/plan`; `/plans`, admin plan management, the account menu and header share the published offer and balance semantics

## chg
-
