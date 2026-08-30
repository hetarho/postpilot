import { useVoiceVersions } from '@/entities/voice'
import { VoiceVersionHistory } from '@/features/edit-voice-profile'
import { VoiceScreen, type VoiceScreenContext } from './VoiceScreen'

export function VoiceVersionsPage() {
  return (
    <VoiceScreen
      title="버전 기록"
      description="분석, 직접 수정, 규칙 반영은 모두 이 말투의 새 버전으로 쌓입니다. 복원해도 예전 기록은 지워지지 않아요."
    >
      {(context) => <VersionList {...context} />}
    </VoiceScreen>
  )
}

/** Its own component so the versions query is issued by the screen that renders it — mounting the
 *  profile tab must not fetch this list. */
function VersionList({ ownerId, voiceId, voice, profile }: VoiceScreenContext) {
  const { versions, isPending } = useVoiceVersions(ownerId, voiceId)
  // "아직 저장된 버전이 없어요" while the list is still in flight would be a claim about the
  // voice, not about the request.
  if (isPending) return <p className="text-content-tertiary text-sm">불러오는 중…</p>
  return (
    <VoiceVersionHistory
      ownerId={ownerId}
      voiceId={voiceId}
      profile={profile}
      versions={versions}
      readOnly={voice.deleted}
    />
  )
}
