import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import {
  isTerminal,
  progressLabel,
  progressRatio,
  type GenerationJob,
} from '@/entities/generation-job'
import { postStatusLabel } from '@/entities/post'
import { useSaveStatus, type SaveState } from '@/features/save-draft'
import { ProgressBar, Typography } from '@/shared/ui'

/** The editor's ONE status surface: everything the page has to SAY about its own state, at the top
 *  of the page, and nowhere else (change 15). The docked bar below it holds controls and the reason
 *  a control is refused — nothing that is merely true.
 *
 *  It is two mounts rather than one element because the two halves need different boxes. The BAR
 *  has to survive a draft thousands of pixels tall, which means `sticky` — and a sticky box has to
 *  be in the page's normal flow, so it is the first thing inside the editor's `main`. The LINE
 *  belongs on the row that already carries 목록 and 삭제, where the status badge used to stand.
 *  Both read the same job, the same save state and the same post status, which is what makes them
 *  one surface rather than two indicators. */

/** The 2px track pinned along the top edge of the editor while a job runs.
 *
 *  Sticky, with TWO offsets, because the shell's header has two shapes: on a phone it is not
 *  sticky at all — it scrolls away and gives its height back — so the bar's resting place is the
 *  viewport's top edge, while from `sm:` up the header is `sticky top-0` at `min-h-16` and the bar
 *  has to clear it. It adds no layout height in either: `ProgressBar` paints out of flow, so this
 *  sticky box is zero pixels tall whether the bar is there or not. Full-bleed past the page's own
 *  gutters, because it belongs to the page rather than to its content column. */
export function EditorProgressBar({ job }: { job: GenerationJob | undefined }) {
  const { t } = useTranslation('posts')
  if (!job || isTerminal(job)) return null
  const ratio = progressRatio(job)
  return (
    <div className="sticky top-0 z-10 -mx-4 sm:top-16 sm:-mx-6">
      <ProgressBar label={t('generation.progressAria')} done={ratio?.done} total={ratio?.total} />
    </div>
  )
}

/** One line of `meta` text carrying AT MOST ONE thing, in this precedence: the running job's
 *  stage, then the save state, then the post's own status. A row with two state indicators on it
 *  is what this replaced.
 *
 *  The live region is MOUNTED at all times and only its text changes: a region inserted already
 *  holding its message announces nothing, and this line changes about once a second while someone
 *  is typing — which is also why it is `polite`. */
export function EditorStatusLine({
  job,
  saveState,
  status,
}: {
  job: GenerationJob | undefined
  saveState: SaveState
  status: string
}) {
  const { t } = useTranslation('posts')
  const save = useSaveStatus(saveState)
  // A failing autosave is the only thing this screen has instead of a save button (PRD F-2), so
  // it is unmistakable on the line rather than widened into a notice the row has no room for —
  // and it outranks the job's stage. A generation runs for minutes, and the precedence that put
  // the stage first would have hidden a save that is losing the user's text for all of them.
  const failing = save.state === 'error'
  const running = !failing && job && !isTerminal(job) ? progressLabel(job) : ''
  return (
    <Typography
      variant="meta"
      as="p"
      role="status"
      aria-live="polite"
      aria-label={t('editor.statusAria')}
      className={clsx('min-w-0 truncate', failing && 'text-notice-danger-fg')}
    >
      {failing ? save.label : running || save.label || postStatusLabel(status)}
    </Typography>
  )
}
