import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { unmadeVoiceAIReason, type VoiceRef } from '@/entities/voice'
import { Notice, buttonStyles } from '@/shared/ui'

/** A post whose voice is not made yet (POST-25): readable, editable and exportable like any
 *  other, but every AI action is refused until the voice is made or the post moved. The way to
 *  make it is that voice's own 학습 글, so the notice links there; the other way out is the
 *  picker above it. */
export function UnmadeVoiceWarning({ voice }: { voice: Pick<VoiceRef, 'id'> }) {
  const { t } = useTranslation('voices')
  return (
    <aside>
      <Notice tone="warning" role="status">
        {/* `w-full` drops the link onto its own line instead of leaving it inline at the end of
            the wrapped sentence, where it would be a small target (THEME-23). */}
        <span className="w-full min-w-0">{unmadeVoiceAIReason()}</span>
        {/* A ghost button, not the notice's `link-fg`: it takes the 44px floor with its own
            padding, the notice's own foreground keeps it inside the THEME-18 contract, and the
            underline is its resting affordance — there is no hover on a phone (THEME-28). */}
        <Link
          to="/voices/$voiceId"
          params={{ voiceId: voice.id }}
          className={buttonStyles({
            variant: 'ghost',
            className: 'text-notice-warning-fg shrink-0 underline',
          })}
        >
          {t('warning.learn')}
        </Link>
      </Notice>
    </aside>
  )
}
