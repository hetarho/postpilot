import { useTranslation } from 'react-i18next'
import { ClipNoticeList } from '@/entities/clip-project'
import { AppFailureMessage, Button, ProgressBar, Typography } from '@/shared/ui'
import type { BrowserRenderState } from '../api/useBrowserRender'

export function ClipBrowserRenderStatus({
  state,
  cancel,
  retry,
}: {
  state: BrowserRenderState
  cancel: () => void
  retry?: () => void
}) {
  const { t } = useTranslation('clips')
  if (state.phase === 'idle') return null
  const busy = state.phase === 'running' || state.phase === 'cancelling'
  const label =
    state.phase === 'running'
      ? t(`render.progress.${state.progress.stage}`)
      : t(`render.progress.${state.phase}`)
  return (
    <div className="space-y-3" aria-label={t('render.progress.label')}>
      {busy && <ProgressBar label={label} done={state.progress.percent} total={100} />}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Typography variant="meta" role="status">
          {label}
        </Typography>
        {(busy || state.phase === 'upload_pending') && (
          <Button variant="ghost" pending={state.phase === 'cancelling'} onClick={cancel}>
            {t('render.progress.cancel')}
          </Button>
        )}
      </div>
      {state.local && (
        <div className="space-y-2">
          <Typography variant="meta">{t('render.localReady')}</Typography>
          <video
            className="w-full"
            controls
            preload="metadata"
            src={state.local.url}
            aria-label={t('render.localPreview')}
          />
          <div className="flex flex-wrap gap-2">
            <Button
              variant="ghost"
              onClick={() => {
                const link = document.createElement('a')
                link.href = state.local!.url
                link.download = 'clip.mp4'
                link.click()
              }}
            >
              {t('render.localDownload')}
            </Button>
            {state.phase === 'upload_pending' && retry && (
              <Button onClick={retry}>{t('render.retryUpload')}</Button>
            )}
          </div>
        </div>
      )}
      {state.failure && (
        <div role="alert" className="space-y-1">
          {state.refusal && (
            <Typography variant="meta">{t(`render.refusal.${state.refusal}`)}</Typography>
          )}
          <AppFailureMessage failure={state.failure} />
        </div>
      )}
      <ClipNoticeList notices={state.notices} />
    </div>
  )
}
