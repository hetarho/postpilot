import { useRuleConfirmations } from '@/entities/voice'
import { VoiceRulesManager } from '@/features/manage-voice-rules'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

export function VoiceRulesPage() {
  return (
    <VoiceScreen
      title="대조 규칙"
      description="후보 규칙은 생성에 쓰이지 않습니다. 이 말투의 서로 다른 글에서 근거가 3번 모인 활성 규칙만 적용됩니다."
    >
      {(context) => <RulesPanel {...context} />}
    </VoiceScreen>
  )
}

function RulesPanel({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { confirmations } = useRuleConfirmations(ownerId, voiceId)
  return (
    <VoiceRulesManager
      ownerId={ownerId}
      voiceId={voiceId}
      profile={profile}
      confirmations={confirmations}
      blocked={voice.deleted ? '삭제된 말투는 복원하기 전까지 규칙을 바꿀 수 없어요.' : ''}
    />
  )
}
