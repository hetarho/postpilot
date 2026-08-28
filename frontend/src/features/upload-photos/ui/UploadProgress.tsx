import type { UploadBatchState } from '../model/upload-batch'

interface UploadProgressProps extends UploadBatchState {
  /** The post the photos need is still being created (a new draft's first pick). */
  creatingPost?: boolean
}

/** "변환 중 3/8" while files convert, then "올리는 중 5/8" — the visible progress PRD §6.2
 *  asks for, since eight HEICs on an older phone take seconds. Nothing once the batch is
 *  settled: the strip and the skipped list say the rest. */
export function UploadProgress({ items, completed, creatingPost }: UploadProgressProps) {
  const active = items.filter((item) => item.status !== 'skipped' && item.status !== 'failed')
  const total = active.length + completed
  const converting = active.filter(
    (item) => item.status === 'selected' || item.status === 'converting',
  ).length

  let label = ''
  if (creatingPost) label = '글을 만드는 중…'
  else if (converting > 0) label = `변환 중 ${total - converting}/${total}`
  else if (active.length > 0) label = `올리는 중 ${completed}/${total}`

  return (
    <p role="status" aria-live="polite" className="text-xs text-neutral-500">
      {label}
    </p>
  )
}
