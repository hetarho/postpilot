/** The operator's catalog screen's own tuning (ARCH-21). */

/** The provider slugs the operator's catalog screen lifts to the top, in this order; every
 *  other vendor follows alphabetically.
 *
 *  The provider's catalog is ~420 models across ~40 vendors, so an alphabetical list buries
 *  the handful anyone actually reaches for behind two screens of scrolling. The order is
 *  editorial — the vendors whose models are worth exposing on quality or price — and it is a
 *  display preference only: nothing here grants access, and the search and filters reach
 *  every vendor either way. */
export const FEATURED_MODEL_PROVIDERS: readonly string[] = [
  'openai',
  'anthropic',
  'google',
  'deepseek',
  'z-ai',
  'minimax',
  'meta-llama',
  'meta',
  'x-ai',
  'qwen',
  'moonshotai',
  'mistralai',
]

/** The height the operator's catalog list assumes for a row it has not measured yet. Only the
 *  scrollbar depends on it: every mounted row reports its real height, which differs a lot
 *  between a plain candidate and an enabled model carrying two controls. */
export const CATALOG_ROW_ESTIMATE_PX = 132

/** How many catalog rows are mounted beyond the viewport. Enough that a fast flick does not
 *  reach blank space before the next row renders, few enough that the mounted subtree stays
 *  small — which is the whole point of virtualizing a several-hundred-row list. */
export const CATALOG_ROW_OVERSCAN = 6
