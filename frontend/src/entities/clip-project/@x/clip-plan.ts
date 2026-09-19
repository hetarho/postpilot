/** What `clip-project` exposes to `clip-plan` (ARCH-13 @x): a save and a revision answer with
 *  the whole project in it, and the quote a rewrite is approved against. */
export { toClipProject } from '../api/clip-project'
export { toClipRevisionQuote } from '../api/credits'
export type { ClipProject } from '../model/types'
export type { ClipQuote } from '../model/types'
