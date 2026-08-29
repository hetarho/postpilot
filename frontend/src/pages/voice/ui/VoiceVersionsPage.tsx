import { useVoiceVersions } from '@/entities/voice-profile'
import { VoiceVersionHistory } from '@/features/edit-voice-profile'
import { VoiceScreen } from './VoiceScreen'

export function VoiceVersionsPage() {
  return (
    <VoiceScreen
      title="버전 기록"
      description="분석, 직접 수정, 규칙 반영은 모두 새 버전으로 쌓입니다. 복원해도 예전 기록은 지워지지 않아요."
    >
      {({ profile, ownerId }) => <VersionList ownerId={ownerId} profile={profile} />}
    </VoiceScreen>
  )
}

/** Its own component so the versions query is issued by the screen that renders it — mounting the
 *  profile tab must not fetch this list (A4). */
function VersionList({
  ownerId,
  profile,
}: {
  ownerId: string
  profile: Parameters<typeof VoiceVersionHistory>[0]['profile']
}) {
  const { versions, isPending } = useVoiceVersions(ownerId)
  // "아직 저장된 버전이 없어요" while the list is still in flight would be a claim about the
  // account, not about the request.
  if (isPending) return <p className="text-content-tertiary text-sm">불러오는 중…</p>
  return <VoiceVersionHistory ownerId={ownerId} profile={profile} versions={versions} />
}
