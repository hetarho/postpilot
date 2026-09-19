import { useVoiceProfile, type VoiceRef } from '@/entities/voice'
import { DeletedVoiceWarning, VoiceWarning } from '@/widgets/voice-warning'

/** Below the memo, not above it: three wrapped lines of undismissable warning at the top of the
 *  editor pushed the writing field a fifth of a 640px screen down, for every user who has not
 *  trained a profile yet — and chrome is small, quiet and at the edges (§0). It still sits above
 *  글 생성, which is what it is a caveat about.
 *
 *  A deleted voice is the other caveat, and the louder one: nothing AI will run until it is
 *  restored or the post is moved. Its profile is not read — a tombstone's emptiness is not the
 *  point. */
export function EditorVoiceWarning({ ownerId, voice }: { ownerId: string; voice: VoiceRef }) {
  if (voice.deleted) {
    return (
      <div className="mt-6">
        <DeletedVoiceWarning ownerId={ownerId} voice={voice} />
      </div>
    )
  }
  if (!voice.id) return null
  return <EmptyProfileWarning ownerId={ownerId} voiceId={voice.id} />
}

function EmptyProfileWarning({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { profile } = useVoiceProfile(ownerId, voiceId)
  const warning = <VoiceWarning profile={profile} voiceId={voiceId} />
  if (!profile) return null
  return <div className="mt-6 empty:hidden">{warning}</div>
}
