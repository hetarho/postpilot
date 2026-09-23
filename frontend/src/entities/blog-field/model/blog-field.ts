/** A post's 분야: the product's own v1 list, in its order (QUAL-23). The ids are the stable
 *  ASCII identifiers the server stores; the names a person reads live in the `posts`
 *  namespace under `blogField`. */
export const BLOG_FIELD_IDS = [
  'restaurant',
  'cafe',
  'domestic_travel',
  'fashion_beauty',
  'product_review',
  'parenting_marriage',
  'pets',
  'interior_diy',
  'daily_life',
] as const

export type BlogFieldId = (typeof BLOG_FIELD_IDS)[number]

/** 없음, a real answer rather than a missing one: a post need not have a 분야. */
export const NO_BLOG_FIELD = ''

export type BlogFieldChoice = BlogFieldId | typeof NO_BLOG_FIELD

export function isBlogFieldId(value: string): value is BlogFieldId {
  return (BLOG_FIELD_IDS as readonly string[]).includes(value)
}

/** The `posts`-namespace key a choice's label is rendered from. */
export function blogFieldLabelKey(choice: BlogFieldChoice): `blogField.${BlogFieldId | 'none'}` {
  return choice === NO_BLOG_FIELD ? 'blogField.none' : `blogField.${choice}`
}
