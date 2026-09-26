import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { type VoiceProfile, isEmptyProfile } from '@/entities/voice'
import { Notice, buttonStyles } from '@/shared/ui'

/** The caveat below the memo: this voice has learned nothing yet. Links to THAT voice, since a
 *  post's voice is what the draft will be written in. */
export function VoiceWarning({
  profile,
  voiceId,
}: {
  profile: VoiceProfile | undefined
  voiceId: string
}) {
  const { t } = useTranslation('voices')
  if (!profile || !isEmptyProfile(profile)) return null

  return (
    <aside>
      <Notice tone="warning">
        {/* `w-full` drops the link onto its own line instead of leaving it inline at the end of
            the third wrapped row, where it was an ~84 × 20 target (THEME-23). */}
        <span className="w-full min-w-0">{t('warning.empty')}</span>
        {/* The one thing to press in this box used to be its greyest, smallest text: `link-fg`
            resolves to `content-secondary` against the notice's gold. As a ghost button it takes
            the 44px floor with its own horizontal padding, and the notice's own foreground keeps
            it inside the THEME-18 contract. The underline is its resting affordance — ghost has no
            fill until it is pressed, and there is no hover on a phone (THEME-28). */}
        <Link
          to="/voices/$voiceId"
          params={{ voiceId }}
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
