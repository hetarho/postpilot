import type { UploadBatchState } from '../model/upload-batch'

interface UploadProgressProps extends UploadBatchState {
  /** The post the photos need is still being created (a new draft's first pick). */
  creatingPost?: boolean
}

/** "변환 중 3/8" while files convert, then "올리는 중 5/8" — the visible progress PRD §6.2
 *  asks for, since eight HEICs on an older phone take seconds. It ends on a result rather than
 *  on an empty line: a `role="status"` that goes blank announces nothing, and the only other
 *  confirmation is a strip the user has to count. What went wrong is the strip's notice. */
export function UploadProgress({ items, completed, creatingPost }: UploadProgressProps) {
  const active = items.filter((item) => item.status !== 'skipped' && item.status !== 'failed')
  const failed = items.filter((item) => item.status === 'failed').length
  // Failures stay in the denominator. Counting them out shrinks it as they happen, so losing
  // photo four of eight reads "올리는 중 7/7" — the count hides exactly the thing it exists to
  // report, and the user taps 생성 believing all eight are attached.
  const total = active.length + failed + completed
  const converting = active.filter(
    (item) => item.status === 'selected' || item.status === 'converting',
  ).length

  let label = ''
  if (creatingPost) label = '글을 만드는 중…'
  else if (converting > 0) label = `변환 중 ${total - converting}/${total}`
  else if (active.length > 0) label = `올리는 중 ${completed}/${total}`
  else if (completed > 0) label = `${completed}장을 올렸어요`

  // `role="status"` already implies `aria-live="polite"`; declaring both made every file
  // transition a doubled announcement that queues ahead of the user's own gestures (§9).
  return (
    <p role="status" className="text-content-tertiary min-w-0 text-xs">
      {label}
    </p>
  )
}
