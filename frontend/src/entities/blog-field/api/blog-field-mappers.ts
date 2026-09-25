import { ProtoBlogField } from '@/shared/api'
import { NO_BLOG_FIELD, type BlogFieldChoice, type BlogFieldId } from '../model/blog-field'

const TO_PROTO: Record<BlogFieldChoice, ProtoBlogField> = {
  [NO_BLOG_FIELD]: ProtoBlogField.UNSPECIFIED,
  restaurant: ProtoBlogField.RESTAURANT,
  cafe: ProtoBlogField.CAFE,
  domestic_travel: ProtoBlogField.DOMESTIC_TRAVEL,
  fashion_beauty: ProtoBlogField.FASHION_BEAUTY,
  product_review: ProtoBlogField.PRODUCT_REVIEW,
  parenting_marriage: ProtoBlogField.PARENTING_MARRIAGE,
  pets: ProtoBlogField.PETS,
  interior_diy: ProtoBlogField.INTERIOR_DIY,
  daily_life: ProtoBlogField.DAILY_LIFE,
}

const FROM_PROTO = new Map<ProtoBlogField, BlogFieldChoice>(
  Object.entries(TO_PROTO).map(([choice, wire]) => [wire, choice as BlogFieldChoice]),
)

/** UNSPECIFIED is 없음. A number this build does not know is undefined, never a guess at a
 *  분야 (ARCH-3). */
export function blogFieldFromProto(value: ProtoBlogField): BlogFieldChoice | undefined {
  return FROM_PROTO.get(value)
}

export function blogFieldToProto(choice: BlogFieldChoice): ProtoBlogField {
  return TO_PROTO[choice]
}

/** A read's 분야: UNSPECIFIED is 없음, and a number this build does not know fails the read
 *  rather than reading as 없음 (ARCH-3) — the next whole-set save would otherwise erase it. */
export function requireBlogField(value: ProtoBlogField): BlogFieldChoice {
  const choice = blogFieldFromProto(value)
  if (choice === undefined) throw new Error(`unsupported blog field enum: ${String(value)}`)
  return choice
}

/** A 분야 in a set that must name one — a guideline's or the preset's. The server filters
 *  UNSPECIFIED out of every such set, so one arriving is a read this build does not understand,
 *  and it fails like an unknown number. */
export function requireBlogFieldId(value: ProtoBlogField): BlogFieldId {
  const choice = requireBlogField(value)
  if (choice === NO_BLOG_FIELD) throw new Error('a 분야 set names 없음')
  return choice
}
