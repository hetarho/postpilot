# BILL subscriptions, charges, credit purchase
> r7 | Fixed KRW monthly/annual subscriptions, prorated tier upgrades, paid-subscriber credit packs and operator-reviewed refund requests, with payment history on one billing screen.

## decisions
- BILL-1 [o] a subscription belongs to the account that pays for it: a signed-in non-master account registers its own payment method, picks its own tier and term, and cancels on its own; `free` needs no payment method; a master account buys nothing (→BILL-20); the operator path (`api setplan`) survives for support and never touches a payment method (→QUOTA-3)
- BILL-2 [o] charge the fixed VAT-inclusive KRW prices in QUOTA-7 and QUOTA-34; checkout shows the actual amount payable. FX changes AI-use conversion only (→QUOTA-59), never subscription or pack prices.
- BILL-3 [o] subscription payment anchors preserve the initial payment day and time in Asia/Seoul. Monthly benefit boundaries clamp a missing day to month-end and restore the original day later; ordinary renewal never shifts the anchor. Daily grants follow QUOTA-37 rather than calendar midnight.
- BILL-4 [o] monthly coverage costs one listed monthly price; annual coverage costs ten listed monthly prices for twelve calendar months. Both carry the same daily and monthly benefits (→QUOTA-38); annual payment renewal and monthly benefit renewal are distinct displayed dates.
- BILL-5 [o] a monthly tier upgrade charges `ceil((new_monthly_KRW - current_monthly_KRW) × remaining_current_month / current_month_duration)` in whole KRW.
  - payment success opens higher model rights and QUOTA-35's current-month bonus/export differences; the higher daily amount begins at the next existing reset
  - preserve billing/benefit/daily anchors; a repeated upgrade uses the current tier, not the original tier
  - annual upgrades follow BILL-18; no entitlement changes on a failed or unresolved charge
- BILL-6 [o] downgrade and annual→monthly changes apply at the purchased term end; monthly→annual follows BILL-19. The selected paid tier remains available until then. A pending change can be cancelled and a replacement replaces rather than stacks it; scheduling itself triggers no refund.
- BILL-7 [o] cancellation disables auto-renewal while daily/monthly entitlements continue through paid coverage under their ordinary expiry rules. At term end the account becomes free and is not charged; resuming before expiry changes no anchor or grants. Cash refund requests are separate (→BILL-11).
- BILL-8 [o] a finally failed renewal ends paid coverage and returns the account to free without retry, grace or dunning; notify the owner and preserve purchased credits and other lots under their own expiry rules. Unresolved payment is not final failure. Updating a card alone grants nothing; a new successful purchase after lapse starts a fresh payment anchor.
- BILL-9 [o] only an active paid subscriber may buy a fixed credit pack (→QUOTA-34). A purchase changes no tier, anchor, model rights, export count or renewal. Free accounts may inspect retained balances but cannot make a new purchase.
- BILL-10 [o] payment-method registration issues no credit grant (→QUOTA-9).
- BILL-11 [o] refund requests originate on the billing screen and are handled by the operator, independently of scheduled cancellation.
  - a subscription payment requested within seven days with none of its paid benefits used is refunded in full; an untouched credit purchase requested within seven days is also refunded in full
  - accept used or later requests for operator review of the reason and applicable rights; do not blanket-refuse them or promise automatic time-prorated refunds
  - a confirmed refund voids only the entitlements funded by that refunded payment; retain unrelated purchased/voucher value and history
  - record the request, decision, amount and confirmed provider refund once; an unresolved provider refund is not displayed as completed
  - credit compensation for AI faults follows QUOTA-60 and is not a cash refund
- BILL-12 [o] the product mails what a card statement cannot explain — a renewal charged, a renewal failed with the drop to `free`, a scheduled cancellation taking effect, a purchase, a refund — to the account's verified address (→AUTH) and never leaves any of them visible only behind a login
- BILL-13 [o] receipts and tax documents are the payment provider's: the product reproduces its own charge history in the app and issues no cash receipt, tax invoice or business-registration collection of its own
- BILL-14 [o] the authenticated billing screen holds current tier/term, daily/monthly resets and next payment, scheduled changes, payment method, fixed packs, charge/grant history, refund requests/status and cancel/resume. `/plans` owns comparison/entry only (→QUOTA-28); operator refund review is master-only.
- BILL-15 [o] charge, refund, grant and tier-change history is append-only and shows whole-KRW money, tier/term, provider identity and actual timestamps. Credit-use FX snapshots belong to QUOTA-59 and are not fictitious exchange rates attached to fixed-KRW purchases.
- BILL-16 [o] the payment provider must offer KRW recurring card billing, hosted card entry, a refund call and server-to-server notifications; card numbers never reach the product or its logs (a provider token stands for the card) and the provider's notification, never the browser's return from checkout, is what makes a charge real ← a user closing the tab mid-redirect must not lose a subscription they paid for; which provider satisfies this is engineering's choice
- BILL-18 [o] an annual upgrade charges `ceil((new_annual_KRW - current_annual_KRW) × remaining_paid_year / current_paid_year_duration)` without moving annual expiry or the monthly benefit anchor. Current bonus/export top-ups use the remaining benefit-month fraction (→QUOTA-35), future months grant the full higher amounts, and daily credit increases at the next existing reset.
- BILL-17 [x] promotion codes, automatic top-up, postpaid usage, team billing, self-serve tax documents, dunning, automatic prorated refunds and extra server-export sales — out of scope; operator-issued vouchers are GIFT's
- BILL-19 [o] monthly→annual switches at the next monthly renewal and charges the full selected annual price then. Annual→monthly and annual downgrades wait for paid annual expiry; no immediate term conversion or unused-month refund is implied.
- BILL-20 [o] a master account is never charged: subscription start, tier/term change and pack purchase are refused server-side, and `/plans` and the billing screen show operator coverage with no checkout, change or pack action (→QUOTA-63) ← master is not sold (→QUOTA-7), and a paid tier bought by a master would demote it
  - a subscription the account still holds from before promotion ends at its paid term end without a renewal charge, as a cancellation does (→BILL-7), and the tier stays master
  - refund requests for its past payments remain available (→BILL-11); a confirmed refund voids the refunded entitlements, never the master tier
- BILL-21 [o] `/plans`' selected billing term accompanies a new paid-tier subscription entry into checkout; checkout keeps the tier and term selected, then shows the server-confirmed amount and requires the owner's final payment action (→BILL-2 →BILL-16). For an active subscription, a tier upgrade keeps the current term and its applicable prorated quote; a term change follows BILL-19's scheduled timing. A period preview never silently changes an existing subscription.

## flow
- subscribe: `/plans` tier/term selection → master(no checkout, operator coverage) | checkout with selected tier/term → hosted card setup if required → fixed-KRW payment confirmed → paid coverage + QUOTA-42 entitlements
- renew: payment boundary → confirmed payment(next paid term) | confirmed failure(free + notice); monthly benefit boundary → bonus/export renewal only while covered
- upgrade: quote current-tier prorated KRW → confirm payment → immediate model/bonus/export change; next daily reset → higher daily grant
- term switch/downgrade: schedule or replace → effective paid-term boundary → new term/tier and full applicable charge
- cancel: auto-renew off → keep paid benefits until term end → free; resume before expiry → no regrant
- buy: active paid subscription → fixed pack → confirmed charge → purchased lot
- refund: owner request → operator review → provider refund → confirmed history + void the refunded entitlement, without a duplicate refund

## constraints
- signup, verified email and operator accounts without email follow AUTH; a payment method requires the account's verified email
- QUOTA owns credit/export entitlements, prices/grants, pack amounts, reset windows, debit order and fault compensation; billing owns payment coverage and cash refunds
- confirmed server-side provider payment state is authoritative (→BILL-16); repeated notifications and unresolved outcomes must not duplicate charges, grants or refunds
- a refund or subscription change preserves other independently purchased credit and content; scheduled cancellation does not cancel an already-admitted job's original-period settlement
- the product is a paid public one (→ARCH-1 →ARCH-23); which payment/FX adapters satisfy the public contracts is engineering's choice
- placement: `backend/internal/billing`, its store/RPC adapters, `proto/postpilot/v1/billing.proto`, `frontend/src/entities/subscription` and billing/refund features; the payment adapter stays behind consumer-declared ports

## chg
-
