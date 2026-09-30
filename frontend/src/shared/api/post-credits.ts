import { PostCreditsBasis } from './gen/postpilot/v1/provider_pb'

/** Where a per-post credit figure came from (QUOTA-64): the median of recent real posts, or the
 *  catalog-based estimate a screen labels 예상. */
export type PostCreditsBasisName = 'recent' | 'estimate'

/** The one client mirror of the wire basis (ARCH-3), pinned by a test that walks the enum.
 *  UNSPECIFIED, and any value this build does not know, is no figure at all. */
const basisNames: Partial<Record<PostCreditsBasis, PostCreditsBasisName>> = {
  [PostCreditsBasis.RECENT_USAGE]: 'recent',
  [PostCreditsBasis.ESTIMATE]: 'estimate',
}

export function postCreditsBasisName(value: PostCreditsBasis): PostCreditsBasisName | undefined {
  return basisNames[value]
}
