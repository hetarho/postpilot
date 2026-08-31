import { useTranslation } from 'react-i18next'
import { useRuleConfirmations } from '@/entities/voice'
import { VoiceRulesManager } from '@/features/manage-voice-rules'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

export function VoiceRulesPage() {
  const { t } = useTranslation(['voices', 'nav'])
  return (
    <VoiceScreen
      title={t('voice.rules', { ns: 'nav' })}
      description={t('screens.rulesDescription', { ns: 'voices' })}
    >
      {(context) => <RulesPanel {...context} />}
    </VoiceScreen>
  )
}

function RulesPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { t } = useTranslation('voices')
  const { confirmations } = useRuleConfirmations(ownerId, voiceId)
  return (
    <VoiceRulesManager
      ownerId={ownerId}
      voiceId={voiceId}
      profile={profile}
      confirmations={confirmations}
      blocked={voice.deleted ? t('screens.rulesBlocked') : ''}
    />
  )
}
