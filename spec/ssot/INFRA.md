# INFRA hosting and durability
> r1 | Where postpilot's backend runs, what it may cost, how much data it may lose, how fast it recovers and how much it can render at once

## decisions
- INFRA-1 [o] scope: hosts, database operation, backups, recovery, render capacity and infrastructure cost of postpilot's backend; code-level deployment and media-worker contracts stay in ARCH (→ARCH-32 →ARCH-45 →ARCH-52 →ARCH-54), and the frontend stays static on Cloudflare Workers (→ARCH-4)
- INFRA-2 [o] budget: everything that serves postpilot costs at most ₩50,000 a month including VAT
  - counted: hosts, database, backups, R2 storage, mail and domain
  - not counted: LLM provider spend, which credits recover (→QUOTA), and the share of other projects on a shared host
  - each provider's charge is counted at that month's exchange rate plus the VAT it bills
  - a change that would pass the budget is an owner decision; nothing scales past it on its own
- INFRA-3 [o] a prepaid hosting commitment runs at most 12 months ← most of the discount without binding a host the product may outgrow
- INFRA-4 [o] every provider account carries a billing alert at 80% of its share of the budget ← the end of a free period or a usage charge must not first appear on the invoice
- INFRA-5 [o] the API and its database run in Seoul, in the same region; the database never runs in another region from the API ← every authenticated request makes at least two database round trips before its handler runs, and a round trip is 69 ms to Singapore and 30 ms to Tokyo against under 6 ms inside Seoul (measured 261002)
- INFRA-6 [o] the production database is PostgreSQL (→ARCH-10), operated by the operator on the API host rather than as a managed database service ← no added cost at the current data size, where the cheapest managed plan in Seoul takes about 45% of the budget
- INFRA-7 [o] a total loss of the API host loses at most the last 5 minutes of committed database writes
  - backups leave the host continuously and live in object storage, never only on the host they protect
  - point-in-time restore reaches back 7 days; a deleted account's rows leave the backups once that window passes
- INFRA-8 [o] a total loss of the API host is back in service within 4 hours, restored from backups onto a fresh host; there is no standby host or replica ← zero added cost for a recovery time one operator can meet
  - the operator is alerted within 5 minutes of `/health` failing ← the 4 hours count from the failure, not from someone noticing it
  - the rebuild is a written procedure, rehearsed as a full restore onto a fresh host every quarter and before any host move
- INFRA-9 [o] every host is rebuildable from the repository, the off-host backups and the secret inventory alone; no host holds the only copy of data, configuration or a secret ← INFRA-8's 4 hours and any provider move depend on it
- INFRA-10 [o] R2 objects (photos, clip sources, delivered clips) have no backup beyond the bucket's own durability; an object that a code path or the operator deletes is not recoverable
- INFRA-11 [o] user media is processed only on machines rented under the operator's account from a hosting provider; personal, acquaintance-owned and marketplace-hosted machines never process it ← uploaded footage passes through the worker's disk and memory, and every processor must be one the privacy policy can name
- INFRA-12 [o] two clips render at once from launch, each worker on CPU and memory of its own so neither slows the other (→ARCH-52); more render capacity is an owner decision within INFRA-2 ← one worker makes a second owner wait the 10–20 minutes the first clip takes before their render starts
- INFRA-13 [o] a worker host may run outside Seoul and apart from the API host (→ARCH-54) ← workers pull jobs and move files through R2, so their distance from users costs nothing
- INFRA-14 [o] each clip's wait between its render request and a worker claiming it is recorded where the owner can read it ← the signal for INFRA-12's capacity decision
- INFRA-15 [o] a worker host sustains full CPU for a whole render: no credit-based burstable plan, and a provider's sustained-use throttling is checked against both workers running continuously before its host is chosen ← credit exhaustion slows a render about ninefold and runs it into the media timeout
- INFRA-16 [o] other projects may share the API host but never a worker host, and postpilot's reserved capacity on the API host is not lent to them
- INFRA-17 [?] the API host and the worker hosts are chosen by running the release render benchmark on candidate plans that satisfy INFRA-2, INFRA-3, INFRA-5 and INFRA-15

## flow
- host loss: `/health` fails → operator alert(≤5 min) → fresh host from the repository → database restored to its latest point → secrets from the inventory → DNS switched → `/health` passes(≤4 h from the failure)

## chg
- r1 261002 initial
