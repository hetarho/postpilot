import { useTranslation } from 'react-i18next'
import { useSetDefaultVoice, useVoices } from '@/entities/voice'
import {
  Button,
  ChoiceRow,
  FieldMessage,
  ListGroup,
  LoadingState,
  Notice,
  Typography,
} from '@/shared/ui'

/** The account's voices as one pick, the 기본 checked — 기본으로 설정 as a list (VOICE-12): choosing
 *  a made voice makes it the 기본, saved at once, and `onChosen` runs once the server has it. A
 *  voice still being made is listed with how far it has come and cannot be chosen, as it cannot be
 *  the 기본 (VOICE-32). A refused save says so under the list, and the 기본 stays what it was:
 *  nothing is shown as chosen before the server answers. */
export function DefaultVoiceChoices({
  ownerId,
  label,
  onChosen,
}: {
  ownerId: string
  /** The list's accessible name. */
  label: string
  onChosen?: () => void
}) {
  const { t } = useTranslation(['voices', 'common'])
  const { active, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  const setDefault = useSetDefaultVoice(ownerId)
  const saving = setDefault.isPending ? setDefault.variables?.voiceId : undefined
  if (isError)
    return (
      <Notice tone="danger" role="alert">
        <span>{t('voiceLoadFailed')}</span>
        <Button
          variant="ghost"
          onClick={refetch}
          pending={isFetching}
          className="text-notice-danger-fg underline"
        >
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Notice>
    )
  if (isPending) return <LoadingState variant="list" label={t('state.loading', { ns: 'common' })} />
  if (active.length === 0)
    return (
      <Typography variant="body" as="p" role="status" className="text-content-secondary">
        {t('defaultChoice.empty')}
      </Typography>
    )
  const choose = (voiceId: string, isDefault: boolean) => {
    if (isDefault) return onChosen?.()
    setDefault
      .setDefault(voiceId)
      .then(() => onChosen?.())
      // The refusal is said under the list, from the mutation's own error.
      .catch(() => undefined)
  }
  return (
    <div className="flex flex-col gap-2">
      <div role="radiogroup" aria-label={label}>
        <ListGroup>
          {active.map((voice) => (
            <li key={voice.id} data-autofocus={voice.isDefault || undefined}>
              <ChoiceRow
                mode="radio"
                checked={voice.isDefault}
                name={voice.name}
                meta={
                  voice.made ? undefined : t('row.learning', { percent: voice.readinessPercent })
                }
                disabled={!voice.made || (saving !== undefined && saving !== voice.id)}
                pending={saving === voice.id}
                onPick={() => choose(voice.id, voice.isDefault)}
              />
            </li>
          ))}
        </ListGroup>
      </div>
      {setDefault.isError && <FieldMessage>{setDefault.errorMessage}</FieldMessage>}
    </div>
  )
}
