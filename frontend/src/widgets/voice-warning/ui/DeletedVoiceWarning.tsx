import { voiceRefLabel, type VoiceRef } from '@/entities/voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { Notice } from '@/shared/ui'

/** A post whose voice was deleted (spec/policy/posts.md): still readable, editable and exportable,
 *  but every AI action is refused by the server until the voice is restored or the post moved.
 *  Both ways out are offered here — restore in place, reassign through the picker above it. */
export function DeletedVoiceWarning({ ownerId, voice }: { ownerId: string; voice: VoiceRef }) {
  return (
    <aside>
      <Notice tone="warning" role="status">
        <span className="w-full min-w-0">
          <span className="font-medium break-words">{voiceRefLabel(voice)}</span> — 이 글은 읽고
          직접 고치고 내보낼 수 있어요. AI 생성·수정·학습은 말투를 복원하거나 위에서 다른 말투로 바꾼
          뒤에 할 수 있어요.
        </span>
        <RestoreVoiceButton
          ownerId={ownerId}
          voiceId={voice.id}
          variant="ghost"
          className="text-notice-warning-fg shrink-0 underline"
        />
      </Notice>
    </aside>
  )
}
