# INFRA hosting and durability
> r2 | Backend cost and durability, with browser-owned media work and explicitly bounded Max server-render capacity on the existing CPU deployment.

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
- INFRA-11 [o] browser preparation and rendering process only the authenticated owner's media on that owner's device; server-side processors remain machines rented under the operator's hosting accounts.
  - no peer rendering, cross-account media distribution, acquaintance-owned worker or marketplace-hosted worker processes user footage
  - privacy disclosures distinguish local browser computation, private storage and external AI analysis (→CLIP-22 →CLIP-203)
- INFRA-12 [o] ordinary media execution is browser-first; commercial server rendering is Max-only and starts with one active render on the existing CPU deployment (→CLIP-206 →QUOTA-7 →ARCH-52).
  - waiting work is finitely bounded and remains cancellable; overload refuses admission instead of accumulating an unbounded queue
  - browser export never waits for a server rendering slot; lightweight validation and storage costs remain separately measured
  - no second dedicated worker, GPU rental or automatic capacity purchase is required for launch; additional capacity is an owner decision within INFRA-2
- INFRA-13 [o] a worker host may run outside Seoul and apart from the API host (→ARCH-54) ← workers pull jobs and move files through R2, so their distance from users costs nothing
- INFRA-14 [o] a server export shows its actual waiting/running state and configured wait expiry; queue delay, execution time and overload refusals are recorded for capacity decisions without a promised completion time.
  - browser preparation, render and upload durations are recorded separately from server queue delay (→CLIP-207)
- INFRA-15 [o] any added worker host is qualified under sustained rendering load and its explicit CPU/memory budgets; burst-credit exhaustion or provider throttling is included in the benchmark before selection.
- INFRA-16 [o] the API and one CPU media worker may share the existing host with explicit CPU, memory and temporary-disk limits reserving API capacity; other projects receive no postpilot-reserved capacity (→ARCH-52).
  - future dedicated worker hosts carry no unrelated projects
- INFRA-17 [?] whether measured Max queue delay, validation cost and sustained resource use justify another hosting plan within INFRA-2; existing CPU placement remains the default until a benchmark supports a change

## flow
- host loss: `/health` fails → operator alert(≤5 min) → fresh host from the repository → database restored to its latest point → secrets from the inventory → DNS switched → `/health` passes(≤4 h from the failure)

## chg
- r2 261006 INFRA-11✎ hosting machines only→owner browser plus operator-hosted server processors; INFRA-12✎ two dedicated renders at launch→browser-first and one bounded Max CPU render; INFRA-14✎ worker-claim wait→separate browser/server timings; INFRA-15✎ two sustained workers→measured added capacity; INFRA-16✎ separate worker host→bounded existing-host colocation; INFRA-17✎ host selection→benchmark-triggered expansion
- r1 261002 initial
