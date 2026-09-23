import type { Transport } from '@connectrpc/connect'

/** Every quality entry of one account. Per account like every other directory, so an account
 *  switch on the same device never reads the previous account's numbers. */
export function qualityQueriesKey(transport: Transport, ownerId: string) {
  return ['quality', transport, ownerId] as const
}

/** One post's measurement at one content revision (QUAL-3). Keyed by the revision so a generation
 *  result, which calls no content-save hook, still reads fresh numbers; a string because a bigint
 *  cannot be hashed into a query key. */
export function postMeasurementQueryKey(
  transport: Transport,
  ownerId: string,
  slug: string,
  revision: bigint,
) {
  return [...qualityQueriesKey(transport, ownerId), 'measurement', slug, String(revision)] as const
}
