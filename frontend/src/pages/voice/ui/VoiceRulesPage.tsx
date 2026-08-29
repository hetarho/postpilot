import { useRuleConfirmations } from '@/entities/voice-profile'
import { VoiceRulesManager } from '@/features/manage-voice-rules'
import { VoiceScreen } from './VoiceScreen'

export function VoiceRulesPage() {
  return (
    <VoiceScreen
      title="대조 규칙"
      description="후보 규칙은 생성에 쓰이지 않습니다. 서로 다른 글에서 근거가 3번 모인 활성 규칙만 적용됩니다."
    >
      {({ profile, ownerId }) => <RulesPanel ownerId={ownerId} profile={profile} />}
    </VoiceScreen>
  )
}

function RulesPanel({
  ownerId,
  profile,
}: {
  ownerId: string
  profile: Parameters<typeof VoiceRulesManager>[0]['profile']
}) {
  const { confirmations } = useRuleConfirmations(ownerId)
  return <VoiceRulesManager ownerId={ownerId} profile={profile} confirmations={confirmations} />
}
