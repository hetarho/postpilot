/** The model catalog's own constants (ARCH-21). */

/** How long the model catalog is trusted before it is re-asked. The usable-model list only
 *  changes when an operator curates it, so refetching it on every mount buys nothing; a few
 *  minutes means a newly enabled model shows up without a reload while the dropdowns stay
 *  instant. */
export const MODEL_CATALOG_STALE_MS = 5 * 60_000

/** The five purposes a model can be registered to, in the order the operator's tabs show
 *  them. The slugs are the wire/DB contract (change 20); the first three feed the user-facing
 *  observe/analyze/write stages, the two generation purposes are settings for features that
 *  do not exist yet. Labels live in the i18n resources under `models:catalog.purposeTab`. */
export const MODEL_PURPOSES = [
  'photo-analysis',
  'style-analysis',
  'writing',
  'image-generation',
  'video-generation',
] as const

export type ModelPurpose = (typeof MODEL_PURPOSES)[number]
