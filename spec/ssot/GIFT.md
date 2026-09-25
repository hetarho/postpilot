# GIFT vouchers and gift links
> r2 | The operator issues vouchers (이용권) — credits that expire a set number of days after redemption — sold by bank transfer or given free, each delivered as a one-time gift link anyone can open and a signed-in account redeems.

## decisions
- GIFT-1 [o] a voucher grants a fixed number of credits that expire a fixed number of days after redemption; it never changes the account's tier, anchor or subscription ← a tier decides only the monthly grant (→QUOTA-2), so expiring credits deliver the same value without colliding with BILL's subscription lifecycle
- GIFT-2 [o] only the operator issues, lists and revokes vouchers, on the admin surface; they are master-only procedures (→QUOTA-25, →AUTH-18)
- GIFT-3 [o] issuance picks credits and validity from presets equal to the paid rungs — basic 330 · pro 1150 · max 2400 credits for 30 days, read from the code-owned ladder (→QUOTA-8) — or enters both numbers directly
- GIFT-4 [o] every voucher records whether it was sold or given: a sold one carries the KRW amount received and the payer name the operator entered, a given one carries neither; neither figure changes what the voucher grants ← bank-transfer sales need a ledger, and the voucher list is that ledger
- GIFT-5 [o] a voucher carries an optional one-line message shown on its gift page; an empty message shows a default line
- GIFT-6 [o] issuing yields one gift link carrying an unguessable token; the link is a bearer credential redeemable once, by whichever signed-in account redeems it first ← a link a message app can carry beats binding to an email the operator may not have; a leaked unredeemed link is answered by revocation (GIFT-10)
- GIFT-7 [o] an unredeemed link expires 90 days after issuance; an expired unredeemed voucher is replaced by issuing a new one, never extended
- GIFT-8 [o] the gift page is public: without a session it shows the credits, the validity days, the message and the link's state (redeemable · redeemed · expired · revoked); a visitor without a session signs in or signs up (email verification included, →AUTH-34) and lands back on the same page (→AUTH-27)
- GIFT-9 [o] redemption is an explicit action on the gift page, never a side effect of opening it; it is single-use under concurrency — one of two simultaneous redemptions wins and the other sees the voucher as redeemed — and opens one credit lot of the voucher's credits expiring the validity days after the redemption instant
- GIFT-10 [o] the operator may revoke a voucher at any time: an unredeemed one's link stops working; a redeemed one's unspent remainder is voided while spent credits stay spent; money is returned outside the product ← a paid voucher's 7-day refund needs its credits gone, and a bank transfer is returned by hand
- GIFT-11 [o] expiring lots burn first in expiry order — the monthly lot and voucher lots together — then bonus, then purchased (→QUOTA-12) ← a voucher credit left for last would lapse unspent while never-expiring credits burned before it
- GIFT-12 [o] an account may redeem any number of vouchers; each redemption is its own lot
- GIFT-13 [o] a redeemed voucher appears among the account's lots with its kind and expiry (→QUOTA-26); the product sends no mail on issue, redemption or expiry — the operator delivers the link
- GIFT-14 [o] the admin voucher list shows each voucher's issue date, credits, validity, sold amount and payer or given, state, redeeming account and redemption date, and a redeemed voucher's remaining credits; an unredeemed voucher's link can be copied again
- GIFT-15 [x] users buying vouchers or gifting their own credits, card-paid vouchers, vouchers that grant a tier, recipient-bound vouchers, per-account redemption limits, mail on issue/redeem/expiry, bulk issuance, typed coupon codes, partial redemption — out of scope

## flow
- issue: admin → preset | custom(credits, days) → sold(amount, payer) | given → message(optional) → link copied → operator delivers it
- redeem: open link → gift page(redeemable → session(yes → redeem → lot opens | no → sign in / sign up → back → redeem → lot opens) | redeemed | expired | revoked)
- revoke: admin → unredeemed(link stops working) | redeemed(unspent remainder voided)

## constraints
- a voucher lot is a credit lot like any other (→QUOTA-11, →QUOTA-58); its expiry is a read-time predicate and its burn order is QUOTA-12's
- the public gift page reveals only credits, validity, message and state — never the issuer, the sale amount, the payer or the redeeming account
- the link token is the voucher's only secret and is never written to logs
- money received and returned for a sold voucher moves outside the product; the product records the sale (GIFT-4) and issues no receipt or tax document (→BILL-13)

## chg
- r2 260925 constraints✎ QUOTA-12 change required→landed as QUOTA r20 (QUOTA-12, QUOTA-58)
- r1 260925 initial
