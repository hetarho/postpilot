import { createClient, type Transport } from '@connectrpc/connect'
import { QualityService, type ContentLanguage } from '@/shared/api'
import { toAccountQuality } from './quality-mappers'

/** Every quality entry of one account. Per account like every other directory, so an account
 *  switch on the same device never reads the previous account's numbers. */
export function qualityQueriesKey(transport: Transport, ownerId: string) {
  return ['quality', transport, ownerId] as const
}

/** One post's measurement at one content revision (QUAL-3). Keyed by the revision so a generation
 *  result, which calls no content-save hook, still reads fresh numbers; a string because a bigint
 *  cannot be hashed into a query key. */
/** The account aggregate as one post's brief reads it. The post's target language is in the key
 *  because each rule text is rendered in it, so a language switch is a new read. */
export function accountQualityQueryKey(
  transport: Transport,
  ownerId: string,
  slug: string,
  targetLanguage: ContentLanguage,
) {
  return [...qualityQueriesKey(transport, ownerId), 'account', slug, targetLanguage] as const
}

/** The one read behind the brief's rows, shared by the hook and the prefetch. Mapped inside the
 *  read, so a reading this build cannot name fails the query rather than rendering a guess. */
export function accountQualityQuery(
  transport: Transport,
  ownerId: string,
  slug: string,
  targetLanguage: ContentLanguage,
) {
  return {
    queryKey: accountQualityQueryKey(transport, ownerId, slug, targetLanguage),
    queryFn: () =>
      createClient(QualityService, transport).getAccountQuality({ slug }).then(toAccountQuality),
  }
}

export function postMeasurementQueryKey(
  transport: Transport,
  ownerId: string,
  slug: string,
  revision: bigint,
) {
  return [...qualityQueriesKey(transport, ownerId), 'measurement', slug, String(revision)] as const
}
