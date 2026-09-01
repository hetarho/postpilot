import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useVoiceValidations, voiceValidationState } from '@/entities/voice'
import { ValidateVoiceProfile } from '@/features/validate-voice-profile'
import { formatPercent } from '@/shared/lib'
import { Typography, typographyStyles } from '@/shared/ui'
import { VoiceScreen } from './VoiceScreen'

export function VoiceValidationsPage() {
  const { t } = useTranslation(['voices', 'nav'])
  return (
    <VoiceScreen
      title={t('voice.validations', { ns: 'nav' })}
      description={t('screens.validationDescription', { ns: 'voices' })}
    >
      {({ profile, voice, ownerId, voiceId }) => (
        <>
          <ValidateVoiceProfile
            voiceId={voiceId}
            profile={profile}
            blocked={voice.deleted ? t('screens.validationBlocked', { ns: 'voices' }) : ''}
          />
          <ValidationRecords ownerId={ownerId} voiceId={voiceId} />
        </>
      )}
    </VoiceScreen>
  )
}

function ValidationRecords({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { t } = useTranslation(['voices', 'common'])
  const { validations, isPending } = useVoiceValidations(ownerId, voiceId)
  if (isPending)
    return (
      <Typography variant="body" className="text-content-tertiary mt-8">
        {t('state.loading', { ns: 'common' })}
      </Typography>
    )
  if (validations.length === 0) {
    return (
      <Typography variant="body" className="text-content-tertiary mt-8">
        {t('screens.noValidations', { ns: 'voices' })}
      </Typography>
    )
  }
  return (
    <section className="mt-8">
      <Typography variant="title" as="h3">
        {t('screens.validationHistory', { ns: 'voices' })}
      </Typography>
      <ul className="mt-2 space-y-2">
        {validations.map((validation) => {
          const status = t(`validation.status.${voiceValidationState(validation.status)}`, {
            ns: 'voices',
          })
          const copy =
            validation.totalCount > 0
              ? t('validation.historyEntryWithScore', {
                  ns: 'voices',
                  version: validation.profileVersion.toString(),
                  status,
                  rate: formatPercent(validation.yCount / validation.totalCount),
                })
              : t('validation.historyEntry', {
                  ns: 'voices',
                  version: validation.profileVersion.toString(),
                  status,
                })
          return (
            <li key={validation.id}>
              <Link
                to="/voices/$voiceId/validations/$id"
                params={{ voiceId, id: validation.id }}
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-link-fg inline-flex min-h-11 items-center underline',
                })}
              >
                {copy}
              </Link>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
