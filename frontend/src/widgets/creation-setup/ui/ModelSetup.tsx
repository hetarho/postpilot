import { useIsMutating } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { ActiveModelForm } from '@/features/configure-model-pair'
import type { SetupController } from '@/features/complete-setup'
import { Button, Notice, Typography } from '@/shared/ui'
export function ModelSetup({ controller }: { controller: SetupController }) {
  const { t } = useTranslation('creation')
  const pending = useIsMutating() > 0
  return (
    <div className="space-y-6">
      <ActiveModelForm stage="observe" />
      {controller.modelError && (
        <Notice tone="danger" role="alert">
          {t('setup.modelFailed')}
          <Button variant="ghost" onClick={controller.retryModels}>
            {t('setup.retry')}
          </Button>
        </Notice>
      )}
      <ActiveModelForm stage="analyze" />
      <ActiveModelForm stage="write" />
      <Typography variant="body" role="status" className="text-content-secondary">
        {!controller.modelsReady ? t('setup.models.waiting') : ''}
      </Typography>
      <Button
        variant="cta"
        className="w-full"
        disabled={!controller.modelsReady || pending}
        pending={pending}
        onClick={() => controller.next(controller.modelsReady)}
      >
        {t('setup.next')}
      </Button>
    </div>
  )
}
