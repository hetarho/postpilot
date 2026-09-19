import { useRef, useState } from 'react'
import { Download } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { ClipNoticeList, useRefreshClipProjects, type ClipProject } from '@/entities/clip-project'
import { Button, Typography, buttonStyles } from '@/shared/ui'

// Key this component by result.createdAt: a new output gets its own single
// automatic URL refresh. No Blob, source file or persistent browser storage.
export function ClipResult({ project, ownerId }: { project: ClipProject; ownerId: string }) {
  const { t } = useTranslation('clips')
  const projects = useRefreshClipProjects(ownerId)
  const [attempt, setAttempt] = useState<'fresh' | 'refreshing' | 'retried' | 'failed'>('fresh')
  const refreshed = useRef(false)
  const refreshing = useRef(false)
  const result = project.result
  async function refresh() {
    if (refreshing.current) return
    refreshing.current = true
    refreshed.current = true
    setAttempt('refreshing')
    try {
      await projects.detail(project.id)
      setAttempt('retried')
    } catch {
      setAttempt('failed')
    } finally {
      refreshing.current = false
    }
  }
  if (!result) return null
  return (
    <section aria-labelledby="clip-result-heading" className="mt-10 space-y-4">
      <Typography id="clip-result-heading" variant="title">
        {t('generation.result')}
      </Typography>
      {result.viewUrl && attempt !== 'failed' && (
        <video
          controls
          preload="metadata"
          src={result.viewUrl}
          aria-label={t('generation.preview')}
          className="max-h-screen w-full rounded-md"
          onError={() => {
            if (!refreshed.current) void refresh()
            else if (!refreshing.current) setAttempt('failed')
          }}
        />
      )}
      <ClipNoticeList
        notices={project.notices}
        language={project.language}
        cuts={project.editing?.plan.cuts}
        withTargets
      />
      {attempt === 'refreshing' && (
        <Typography variant="body" role="status">
          {t('generation.refreshing')}
        </Typography>
      )}
      {(attempt === 'failed' || !result.viewUrl) && (
        <div role="alert">
          <Typography variant="body">{t('generation.previewFailed')}</Typography>
          <Button variant="ghost" onClick={() => void refresh()}>
            {t('project.retry')}
          </Button>
        </div>
      )}
    </section>
  )
}

/** Download the identified successful result. Three shapes for the three places
 *  it is offered: ③'s docked `cta`, a `compact` button in flow, and the `icon`
 *  that sits directly under ②'s preview, where the video it downloads already
 *  says what it is and the revision rides in the control's name (CLIP-149). */
export function ClipDownloadAction({
  project,
  compact = false,
  icon = false,
}: {
  project: ClipProject
  compact?: boolean
  icon?: boolean
}) {
  const { t } = useTranslation('clips')
  if (!project.result?.downloadUrl) return null
  const revision = t('finalization.downloadRevision', { revision: project.renderedPlanRevision })
  if (icon)
    return (
      <a
        href={project.result.downloadUrl}
        aria-label={revision}
        className={buttonStyles({ variant: 'ghost', size: 'icon' })}
      >
        <Download aria-hidden="true" className="size-5" />
      </a>
    )
  return (
    <a
      href={project.result.downloadUrl}
      aria-label={project.finalized ? undefined : revision}
      className={buttonStyles({
        variant: compact ? 'secondary' : 'cta',
        className: compact ? undefined : 'w-full sm:w-auto',
      })}
    >
      {project.finalized ? t(compact ? 'timeline.download' : 'generation.download') : revision}
    </a>
  )
}
