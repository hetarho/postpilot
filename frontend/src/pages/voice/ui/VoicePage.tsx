import { StructuredProfileEditor } from '@/features/edit-voice-profile'
import { VoiceScreen } from './VoiceScreen'

export function VoicePage() {
  return (
    <VoiceScreen
      title="말투"
      description="완성한 글을 직접 확정할 때마다 종결어미와 리듬을 한 편씩 배웁니다. 아무 글도 없어도 첫 글을 바로 만들 수 있어요."
    >
      {({ profile, ownerId }) => <StructuredProfileEditor ownerId={ownerId} profile={profile} />}
    </VoiceScreen>
  )
}
