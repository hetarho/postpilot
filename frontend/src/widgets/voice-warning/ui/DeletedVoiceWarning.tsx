import { Trans, useTranslation } from 'react-i18next'
import { voiceRefLabel, type VoiceRef } from '@/entities/voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { Notice } from '@/shared/ui'

/** A post whose voice was deleted (spec/policy/posts.md): still readable, editable and exportable,
 *  but every AI action is refused by the server until the voice is restored or the post moved.
 *  Both ways out are offered here — restore in place, reassign through the picker above it. */
export function DeletedVoiceWarning({ ownerId, voice }: { ownerId: string; voice: VoiceRef }) {
  const { t } = useTranslation('voices')
  return (
    <aside>
      <Notice tone="warning" role="status">
        <span className="w-full min-w-0">
          <Trans
            t={t}
            i18nKey="warning.deletedPost"
            values={{ voice: voiceRefLabel(voice) }}
            components={{ voice: <span className="font-medium break-words" /> }}
          />
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
