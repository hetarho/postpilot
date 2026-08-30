import { Link } from '@tanstack/react-router'
import { useVoiceValidations } from '@/entities/voice'
import { ValidateVoiceProfile } from '@/features/validate-voice-profile'
import { VoiceScreen } from './VoiceScreen'

export function VoiceValidationsPage() {
  return (
    <VoiceScreen
      title="프로필 검증"
      description="직접 실행할 때만 이 말투로 완성한 글 3편의 주제만 중립적으로 요약해 다시 써 봅니다. 프로필은 이 검증으로 바뀌지 않아요."
    >
      {({ profile, voice, ownerId, voiceId }) => (
        <>
          <ValidateVoiceProfile
            voiceId={voiceId}
            profile={profile}
            blocked={
              voice.deleted ? '삭제된 말투는 복원하기 전까지 프로필을 검증할 수 없어요.' : ''
            }
          />
          <ValidationRecords ownerId={ownerId} voiceId={voiceId} />
        </>
      )}
    </VoiceScreen>
  )
}

function ValidationRecords({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { validations, isPending } = useVoiceValidations(ownerId, voiceId)
  if (isPending) return <p className="text-content-tertiary mt-8 text-sm">불러오는 중…</p>
  if (validations.length === 0) {
    return <p className="text-content-tertiary mt-8 text-sm">아직 검증 기록이 없어요.</p>
  }
  return (
    <section className="mt-8">
      <h3 className="font-medium">검증 기록</h3>
      <ul className="mt-2 space-y-2">
        {validations.map((validation) => (
          <li key={validation.id}>
            <Link
              to="/voices/$voiceId/validations/$id"
              params={{ voiceId, id: validation.id }}
              className="text-link-fg inline-flex min-h-11 items-center text-sm underline"
            >
              v{validation.profileVersion.toString()} · {validation.status}
              {validation.totalCount > 0
                ? ` · ${Math.round((validation.yCount / validation.totalCount) * 100)}%`
                : ''}
            </Link>
          </li>
        ))}
      </ul>
    </section>
  )
}
