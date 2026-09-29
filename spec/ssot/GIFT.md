# GIFT vouchers and gift links
> r3 | Operator-issued expiring credit vouchers with public one-time gift links, redeemed only during paid subscription coverage and never granting model or export rights.

## decisions
- GIFT-1 [o] a voucher grants a fixed number of credits expiring after its redemption validity; it changes no tier, subscription/reset anchor, model grade or server-export count (→QUOTA-19 →QUOTA-58).
- GIFT-2 [o] only the operator issues, lists and revokes vouchers, on the admin surface; they are master-only procedures (→QUOTA-25, →AUTH-18)
- GIFT-3 [o] issuance presets equal QUOTA-7's paid monthly bonuses: 290/510/1070/3170 credits, each valid for 30 days after redemption, read from the code-owned ladder. The operator may still specify credits and validity directly.
- GIFT-4 [o] every voucher records whether it was sold or given: a sold one carries the KRW amount received and the payer name the operator entered, a given one carries neither; neither figure changes what the voucher grants ← bank-transfer sales need a ledger, and the voucher list is that ledger
- GIFT-5 [o] a voucher carries an optional one-line message shown on its gift page; an empty message shows a default line
- GIFT-6 [o] the unguessable gift-link token is a bearer credential redeemable once by the first eligible signed-in paid subscriber; recipient binding is not required. Revocation answers a leaked unredeemed link (→GIFT-10).
- GIFT-7 [o] an unredeemed link expires 90 days after issuance; an expired unredeemed voucher is replaced by issuing a new one, never extended
- GIFT-8 [o] the public gift page shows credits, validity, message and link state. A signed-out visitor signs in/up and returns to it; a free account sees that an active paid subscription is required to redeem. Merely viewing the link never starts validity or redeems it.
- GIFT-9 [o] explicit redemption requires active paid coverage and is single-use under concurrency; one winner opens one voucher lot expiring the specified validity after that instant. Subscription lapse never pauses or extends this expiry; the retained balance needs paid access to be spent.
- GIFT-10 [o] the operator may revoke a voucher at any time: an unredeemed one's link stops working; a redeemed one's unspent remainder is voided while spent credits stay spent; money is returned outside the product ← a paid voucher's 7-day refund needs its credits gone, and a bank transfer is returned by hand
- GIFT-11 [o] a voucher lot burns with the other expiring lots in QUOTA-12's consumption order (→QUOTA-12) ← a voucher credit left for last would lapse unspent while never-expiring credits burned before it
- GIFT-12 [o] an eligible paid subscriber may redeem any number of vouchers; each opens its own lot under GIFT-9.
- GIFT-13 [o] a redeemed voucher appears among the account's lots with its kind and expiry (→QUOTA-26); the product sends no mail on issue, redemption or expiry — the operator delivers the link
- GIFT-14 [o] the admin voucher list shows each voucher's issue date, credits, validity, sold amount and payer or given, state, redeeming account and redemption date, and a redeemed voucher's remaining credits; an unredeemed voucher's link can be copied again
- GIFT-15 [x] users buying vouchers or gifting their own credits, card-paid vouchers, vouchers that grant a tier, recipient-bound vouchers, per-account redemption limits, mail on issue/redeem/expiry, bulk issuance, typed coupon codes, partial redemption — out of scope

## flow
- issue: admin → preset | custom(credits, days) → sold(amount, payer) | given → message(optional) → link copied → operator delivers it
- redeem: open link → public state → signed-in paid coverage(yes → explicit once-only redemption → expiring lot | no → sign in/signup or subscribe, then return; no redemption until eligible) | redeemed/expired/revoked
- revoke: admin → unredeemed(link stops working) | redeemed(unspent remainder voided)

## constraints
- a voucher lot is a credit lot like any other (→QUOTA-11, →QUOTA-58); its expiry is a read-time predicate and its burn order is QUOTA-12's
- the public gift page reveals only credits, validity, message and state — never the issuer, the sale amount, the payer or the redeeming account
- the link token is the voucher's only secret and is never written to logs
- money received and returned for a sold voucher moves outside the product; the product records the sale (GIFT-4) and issues no receipt or tax document (→BILL-13)

## chg
-
