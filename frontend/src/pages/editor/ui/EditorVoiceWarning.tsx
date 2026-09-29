import type { VoiceRef } from '@/entities/voice'
import { DeletedVoiceWarning, UnmadeVoiceWarning } from '@/widgets/voice-warning'

/** Below the memo, not above it: three wrapped lines of undismissable warning at the top of the
 *  editor pushed the writing field a fifth of a 640px screen down — and chrome is small, quiet
 *  and at the edges (THEME-8). It still sits above 글 생성, which is what it is a caveat about.
 *
 *  Nothing AI runs on a post whose voice is deleted or not made yet, until it is restored or made
 *  or the post is moved (POST-25). A deleted voice is told first: restoring it is the way out
 *  whether or not it was ever made. A post with 말투 없음 has nothing to warn about. */
export function EditorVoiceWarning({
  ownerId,
  voice,
}: {
  ownerId: string
  voice: VoiceRef | undefined
}) {
  if (!voice) return null
  if (voice.deleted) {
    return (
      <div className="mt-6">
        <DeletedVoiceWarning ownerId={ownerId} voice={voice} />
      </div>
    )
  }
  if (!voice.made) {
    return (
      <div className="mt-6">
        <UnmadeVoiceWarning voice={voice} />
      </div>
    )
  }
  return null
}
