import type { PostContent } from '@/shared/api'
import type { ExportFormat } from '../config/guidance'
import type { CopiedFeedback, ManualCopy } from './copy-target'

/** What one copy control's status line reports: its copy landed, it fell back to a manual
 *  selection, or nothing at all. */
export type CopyStatus = 'copied' | 'manual' | undefined

/** The copy feedback the statuses are derived from, as `useCopyFeedback` holds it. */
export interface CopyFeedbackState {
  copied: CopiedFeedback | undefined
  manualCopy: ManualCopy | undefined
}

/** The texts on screen right now, which every status is compared against. */
export interface ExportTexts {
  content: PostContent
  format: ExportFormat
  output: string
  outputWithTags: string
  hashtags: string
}

/** Every text copy's status line, and whether the Naver tab's raw field is revealed.
 *
 *  Derived, never stored: a confirmation cannot outlive the value it confirmed, and a dismissed
 *  fallback cannot resurrect when an edit is undone — so no effect has to reset state over a
 *  prop. */
export function exportCopyStatus(
  { copied, manualCopy }: CopyFeedbackState,
  { content, format, output, outputWithTags, hashtags }: ExportTexts,
) {
  // The Naver tab shows the rendered post; the raw marker text exists only on the clipboard —
  // except while a refused copy needs a visible selection to fall back to. The other three
  // formats are markup meant to be read as source, so they keep the raw field always. The value
  // comparison is what dismisses the fallback when the content changes under it.
  const outputFellBack =
    manualCopy?.source === content &&
    ((manualCopy.target === 'output' && manualCopy.value === output) ||
      (format === 'naver' &&
        manualCopy.target === 'outputWithTags' &&
        manualCopy.value === outputWithTags))
  const fallbackOutput =
    outputFellBack && manualCopy?.target === 'outputWithTags' ? outputWithTags : output
  const rawFieldVisible = format !== 'naver' || outputFellBack

  const title: CopyStatus =
    copied?.target === 'title' && copied.value === content.title
      ? 'copied'
      : manualCopy?.target === 'title' &&
          manualCopy.value === content.title &&
          manualCopy.source === content
        ? 'manual'
        : undefined
  const outputStatus: CopyStatus =
    copied?.target === 'output' && copied.value === output
      ? 'copied'
      : outputFellBack && manualCopy?.target === 'output'
        ? 'manual'
        : undefined
  const outputWithTagsStatus: CopyStatus =
    format === 'naver' && copied?.target === 'outputWithTags' && copied.value === outputWithTags
      ? 'copied'
      : format === 'naver' && outputFellBack && manualCopy?.target === 'outputWithTags'
        ? 'manual'
        : undefined
  // Both comparisons are what make the staleness rule hold by DERIVATION: a confirmation shown
  // for one tag list cannot survive a content change to a different one, and the content-identity
  // check stops a dismissed fallback resurrecting when an edit is undone.
  const tags: CopyStatus =
    copied?.target === 'tags' && copied.value === hashtags
      ? 'copied'
      : manualCopy?.target === 'tags' &&
          manualCopy.value === hashtags &&
          manualCopy.source === content
        ? 'manual'
        : undefined

  return {
    outputFellBack,
    /** What the raw field holds: the combined body while that copy is the one that fell back. */
    fallbackOutput,
    rawFieldVisible,
    title,
    output: outputStatus,
    outputWithTags: outputWithTagsStatus,
    tags,
  }
}

/** Whether one caption's copy fell back, which is also what mounts its field. */
export function captionFellBack(
  { manualCopy }: CopyFeedbackState,
  content: PostContent,
  marker: number,
  caption: string,
): boolean {
  return (
    manualCopy?.target === `caption:${marker}` &&
    manualCopy.value === caption &&
    manualCopy.source === content
  )
}

/** One caption's line, on the same two derivations every other copy status uses: a confirmation
 *  cannot outlive the value it confirmed, and a dismissed fallback cannot resurrect when an edit
 *  is undone. */
export function captionCopyStatus(
  state: CopyFeedbackState,
  content: PostContent,
  marker: number,
  caption: string,
): CopyStatus {
  const { copied } = state
  if (copied?.target === `caption:${marker}` && copied.value === caption) return 'copied'
  if (captionFellBack(state, content, marker, caption)) return 'manual'
  return undefined
}
