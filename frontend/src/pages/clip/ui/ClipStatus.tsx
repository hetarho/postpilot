import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import {
  isTerminal,
  progressLabel,
  progressRatio,
  type GenerationJob,
} from '@/entities/generation-job'
import { clipState, clipStateLabel, type ClipProject } from '@/entities/clip-project'
import type { ClipUploadState } from '@/features/upload-clip-sources'
import { ProgressBar, Typography } from '@/shared/ui'

/** The clip workspace's ONE status surface (CLIP-38, THEME-39): everything the page has to SAY
 *  about its own state, at the top of the page, and nowhere else. The docked bar below holds
 *  controls and the reason a control is refused — nothing that is merely true.
 *
 *  Two mounts rather than one element, for the same reason the editor's are: the BAR has to
 *  survive a page taller than the viewport, which means `sticky`, and a sticky box has to sit in
 *  the normal flow; the LINE belongs on the row that already carries 목록 and 삭제하기. Both read
 *  the same job, the same upload, the same correction and the same project, which is what makes
 *  them one surface rather than two indicators. */

/** What the line needs to know about the correction draft, so it can say the project is waiting
 *  on a save or on a render without importing the correction feature's whole hook. */
export type CorrectionStatus = 'clean' | 'dirty' | 'unrendered'

/** What the page knows about its own saving. A prop rather than a hook so T098 can replace the
 *  source — the explicit save today, an autosave queue next — without touching the precedence. */
export interface SaveStatus {
  failing: boolean
  label: string
}

/** `idle` is the only phase the line stays quiet for: nothing has been picked, so what the screen
 *  has to say is the project's own state and the picker's own button is the instruction. Every
 *  other phase — including `owned` and `finished`, which report that the local originals were
 *  released — is a statement about this attempt. */
const SILENT_PHASES: ReadonlySet<ClipUploadState['phase']> = new Set(['idle'])

/** The 2px track pinned along the page's top edge while a job runs or sources upload.
 *
 *  Sticky with the one `top-chrome` offset (THEME-24) and full-bleed past the page's gutters,
 *  because it belongs to the page rather than to its content column. It adds no layout height:
 *  `ProgressBar` paints out of flow, so this box is zero pixels tall whether the bar is there or
 *  not. Determinate from the stage's own ratio while a job runs, and from the bytes actually
 *  PUT while sources upload — a 200 MB selection over cellular is minutes of a bar that would
 *  otherwise only say "올리는 중". */
export function ClipProgressBar({
  job,
  upload,
}: {
  job: GenerationJob | undefined
  upload: Pick<ClipUploadState, 'phase' | 'entries'>
}) {
  const { t } = useTranslation('clips')
  const running = job && !isTerminal(job)
  const uploading = upload.phase === 'reading' || upload.phase === 'uploading'
  if (!running && !uploading) return null
  const ratio = running ? progressRatio(job) : undefined
  const bytes = upload.entries.reduce((sum, entry) => sum + entry.metadata.bytes, 0)
  return (
    <div className="top-chrome sticky z-10 -mx-4 sm:-mx-6 lg:-mx-8">
      <ProgressBar
        label={running ? progressLabel(job) : t(`source.phase.${upload.phase}`)}
        done={
          running
            ? ratio?.done
            : upload.entries.reduce((sum, entry) => sum + entry.metadata.bytes * entry.percent, 0)
        }
        total={running ? ratio?.total : bytes * 100}
      />
    </div>
  )
}

/** One line of `meta` text carrying AT MOST ONE thing, in CLIP-38's precedence: a failing save,
 *  the running job's stage, the source upload's phase, a correction that is unsaved or
 *  unrendered, the save state, then the project's own state.
 *
 *  A failing save leads because this screen has no save button once the settings autosave (T098)
 *  and a clip generation runs for minutes — the precedence that put the stage first would hide a
 *  save that is losing the user's answers for all of them.
 *
 *  The live region is MOUNTED at all times and only its text changes: a region inserted already
 *  holding its message announces nothing. */
export function ClipStatusLine({
  project,
  job,
  upload,
  correction,
  save,
}: {
  project: ClipProject | undefined
  job: GenerationJob | undefined
  upload: Pick<ClipUploadState, 'phase'>
  correction: CorrectionStatus
  save: SaveStatus
}) {
  const { t } = useTranslation('clips')
  const running = job && !isTerminal(job)
  const message = project?.finalized
    ? clipStateLabel('finished')
    : save.failing
      ? save.label
      : running
        ? progressLabel(job)
        : !SILENT_PHASES.has(upload.phase)
          ? t(`source.phase.${upload.phase}`)
          : correction === 'dirty'
            ? t('correction.dirty')
            : correction === 'unrendered'
              ? t('correction.needsRender')
              : save.label ||
                (job?.status === 'cancelled'
                  ? t('cancellation.cancelled')
                  : job?.status === 'failed'
                    ? t('generation.failedAt', { stage: progressLabel(job) })
                    : project
                      ? clipStateLabel(clipState(project))
                      : '')
  return (
    <Typography
      variant="meta"
      as="p"
      role="status"
      aria-live="polite"
      aria-label={t('project.statusAria')}
      className={clsx('min-w-0 truncate', save.failing && 'text-notice-danger-fg')}
    >
      {message}
    </Typography>
  )
}
