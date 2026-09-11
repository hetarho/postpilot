import { useRef, useState } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { clipProjectsKey, type ClipProject } from '@/entities/clip-project'
import { Button, Typography, buttonStyles } from '@/shared/ui'

// Key this component by result.createdAt: a new output gets its own single
// automatic URL refresh. No Blob, source file or persistent browser storage.
export function ClipResult({ project, ownerId }: { project: ClipProject; ownerId: string }) {
  const { t } = useTranslation('clips')
  const transport = useTransport()
  const cache = useQueryClient()
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
      await cache.invalidateQueries(
        { queryKey: [...clipProjectsKey(transport, ownerId), 'detail', project.id] },
        { throwOnError: true },
      )
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

/** ③ 클립 완성's one committing control, docked (CLIP-40). It is an `<a>` rather than a button
 *  because the download IS a navigation to a presigned object; it lives here beside the preview
 *  that mints the same URL, and the page puts it in the step's bar. */
export function ClipDownloadAction({ project }: { project: ClipProject }) {
  const { t } = useTranslation('clips')
  if (!project.result?.downloadUrl) return null
  return (
    <a
      href={project.result.downloadUrl}
      className={buttonStyles({ variant: 'cta', className: 'w-full sm:w-auto' })}
    >
      {t('generation.download')}
    </a>
  )
}
