import { useTranslation } from 'react-i18next'
import { ClipNoticeList } from '@/entities/clip-project'
import { AppFailureMessage, Button, ProgressBar, Typography } from '@/shared/ui'
import type { BrowserRenderState } from '../api/useBrowserRender'

export function ClipBrowserRenderStatus({
  state,
  cancel,
}: {
  state: BrowserRenderState
  cancel: () => void
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
        {busy && (
          <Button variant="ghost" pending={state.phase === 'cancelling'} onClick={cancel}>
            {t('render.progress.cancel')}
          </Button>
        )}
      </div>
      {state.failure && (
        <div role="alert">
          <AppFailureMessage failure={state.failure} />
        </div>
      )}
      <ClipNoticeList notices={state.notices} />
    </div>
  )
}
