import type { PostContent } from '@/shared/api'
import type { CopyImageResult } from '@/shared/lib'

/** `output`, `title` and `tags` are the fixed text copies; a photo and a caption target name the
 *  marker they belong to, so one file carrying two markers is still two independent copies — and
 *  a caption travels on its own control now that the marker carries none (EXPORT-24). It is a
 *  template literal rather than a bare `string`, which would collapse the union and take the
 *  checking with it. */
export type CopyTarget = TextCopyTarget | `photo:${number}:${string}`

/** The copies that have a field to select as their manual fallback. A caption's field, like the
 *  Naver body's, is mounted only once its copy has fallen back. */
export type TextCopyTarget = 'output' | 'outputWithTags' | 'title' | 'tags' | `caption:${number}`

/** The marker number inside a caption target. The prefix is fixed, so this is a slice, not a
 *  parse that could disagree with the type above. */
export function captionMarker(target: `caption:${number}`): number {
  return Number(target.slice('caption:'.length))
}

/** Every way a photo copy can fail, from `copyImage`. `copied` is not one of them. */
export type FailedCopyKind = Exclude<CopyImageResult['kind'], 'copied'>

/** A copy that reached the clipboard. A TEXT copy stores the value that reached the clipboard
 *  beside it, so a confirmation can be dropped by derivation once the content no longer holds
 *  that value; a photo copy has no value to compare. */
export interface CopiedFeedback {
  target: CopyTarget
  value?: string
}

/** A text copy the clipboard refused, left for the user to select by hand. The VALUE that fell
 *  back is stored with it, so the fallback dissolves by derivation the moment the content no
 *  longer matches what was selected; the CONTENT IDENTITY rides along because the value alone
 *  would resurrect a dismissed fallback when an edit is undone (the output string comes back;
 *  the failed copy does not). */
export interface ManualCopy {
  target: TextCopyTarget
  value: string
  source: PostContent
}

/** A photo copy that failed, and how. A photo has no manual fallback at all — there is nothing to
 *  select and hold (see `copyImage`). */
export interface PhotoFailure {
  target: CopyTarget
  kind: FailedCopyKind
}
