// Query keys and the proto→domain mappers for the purpose entity.
import type { Transport } from '@connectrpc/connect'
import type { ProtoPurpose, ProtoPurposeRef } from '@/shared/api'
import { emptyPurposeRef, type Purpose, type PurposeRef } from '../model/types'

export function toPurpose(purpose: ProtoPurpose): Purpose {
  return {
    id: purpose.id,
    name: purpose.name,
    description: purpose.description,
    instructions: purpose.instructions,
    postCount: purpose.postCount,
    createdAt: purpose.createdAt,
    updatedAt: purpose.updatedAt,
  }
}

/** An unset ref is 없음, not a missing value: the post simply has no purpose. */
export function toPurposeRef(ref: ProtoPurposeRef | undefined): PurposeRef {
  if (!ref) return emptyPurposeRef()
  return { id: ref.id, name: ref.name }
}

/** The directory is per account, like the voice directory: an account switch on the same
 *  device must never read the previous account's briefs. */
export function purposesQueryKey(transport: Transport, ownerId: string) {
  return ['purposes', transport, ownerId] as const
}
