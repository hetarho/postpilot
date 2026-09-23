import { ProtoBlogField } from '@/shared/api'
import { NO_BLOG_FIELD, type BlogFieldChoice } from '../model/blog-field'

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
