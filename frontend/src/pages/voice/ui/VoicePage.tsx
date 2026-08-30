import { StructuredProfileEditor } from '@/features/edit-voice-profile'
import { VoiceScreen } from './VoiceScreen'

export function VoicePage() {
  return (
    <VoiceScreen
      title="프로필"
      description="이 말투로 완성한 글을 직접 확정할 때마다 종결어미와 리듬을 한 편씩 배웁니다. 아무 글도 없어도 첫 글을 바로 만들 수 있어요."
    >
      {({ profile, voice, ownerId, voiceId }) => (
        <StructuredProfileEditor
          ownerId={ownerId}
          voiceId={voiceId}
          profile={profile}
          readOnly={voice.deleted}
        />
      )}
    </VoiceScreen>
  )
}
