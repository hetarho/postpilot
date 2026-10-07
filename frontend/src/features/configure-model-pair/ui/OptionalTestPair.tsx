import { useTranslation } from 'react-i18next'
import { useModels, useModelSetup } from '@/entities/model-catalog'
import { Button, Disclosure, Notice, Typography } from '@/shared/ui'
import { ModelPairForm } from './ModelPairForm'

export function OptionalTestPair() {
  const { t } = useTranslation('models')
  const setup = useModelSetup()
  const models = useModels()
  const failed = setup.isError || models.isError
  return (
    <Disclosure title={t('testPair.title')} headingLevel={3}>
      <Typography variant="body" className="text-content-secondary mt-3">
        {t('testPair.help')}
      </Typography>
      {failed ? (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('testPair.failed')}
          <Button
            variant="ghost"
            className="mt-3"
            onClick={() => {
              setup.refetch()
              models.refetch()
            }}
          >
            {t('activeState.retry')}
          </Button>
        </Notice>
      ) : setup.isPending || models.isPending ? (
        <Typography variant="body" role="status" className="mt-4">
          {t('testPair.loading')}
        </Typography>
      ) : (
        <div className="mt-6 space-y-8">
          {(['observe', 'write'] as const).map((stage) => (
            <section key={stage} aria-label={t('testPair.stage', { stage: t(`stage.${stage}`) })}>
              <Typography variant="fieldTitle" as="h3" className="mb-4">
                {t('testPair.stage', { stage: t(`stage.${stage}`) })}
              </Typography>
              <ModelPairForm stage={stage} />
            </section>
          ))}
        </div>
      )}
    </Disclosure>
  )
}
