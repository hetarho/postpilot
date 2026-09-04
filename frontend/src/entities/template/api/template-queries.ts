// Query keys and the proto→domain mappers for the template entity.
import type { Transport } from '@connectrpc/connect'
import type { ProtoTemplate, ProtoTemplateRef } from '@/shared/api'
import { emptyTemplateRef, type Template, type TemplateRef } from '../model/types'

export function toTemplate(template: ProtoTemplate): Template {
  return {
    id: template.id,
    name: template.name,
    description: template.description,
    body: template.body,
    postCount: template.postCount,
    createdAt: template.createdAt,
    updatedAt: template.updatedAt,
  }
}

/** An unset ref is 없음, not a missing value: the post simply has no template. */
export function toTemplateRef(ref: ProtoTemplateRef | undefined): TemplateRef {
  if (!ref) return emptyTemplateRef()
  return { id: ref.id, name: ref.name }
}

/** The directory is per account, like the voice directory: an account switch on the same
 *  device must never read the previous account's briefs. */
export function templatesQueryKey(transport: Transport, ownerId: string) {
  return ['templates', transport, ownerId] as const
}
