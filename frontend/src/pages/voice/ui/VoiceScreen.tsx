import type { ReactNode } from 'react'
import { useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FailureNotice } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import {
  type Voice,
  type VoiceProfile,
  useVoiceProfile,
  VoiceMaterialFreshness,
  voiceMaterialFreshness,
} from '@/entities/voice'
import { Typography, typographyStyles } from '@/shared/ui'

export interface VoiceScreenContext {
  profile: VoiceProfile
  /** The voice as the profile response names it — the same row the layout shows. */
  voice: Voice
  ownerId: string
  voiceId: string
}

/** The frame every voice tab shares: THIS voice's profile query, its two non-content states, and
 *  the tab's own heading. Both local panels read the profile, so it stays shared here;
 *  every other list is fetched by the tab that renders it. The page's `h1` is the voice's name in
 *  the layout, so a tab's title is an `h2`. */
export function VoiceScreen({
  title,
  description,
  children,
}: {
  title: string | ((context: VoiceScreenContext) => string)
  description?: ReactNode
  children: (context: VoiceScreenContext) => ReactNode
}) {
  const { t } = useTranslation(['voices', 'common'])
  const { voiceId = '' } = useParams({ strict: false })
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { profile, isPending, isError, refetch } = useVoiceProfile(ownerId, voiceId)

  if (isError) {
    return (
      <main className="mt-6">
        <FailureNotice
          message={t('screens.profileLoadFailed', { ns: 'voices' })}
          onRetry={refetch}
        />
      </main>
    )
  }
  if (isPending || !profile) {
    return (
      <main
        className={typographyStyles({ variant: 'body', className: 'text-content-tertiary mt-6' })}
      >
        {t('state.loading', { ns: 'common' })}
      </main>
    )
  }
  const context = { profile, voice: profile.voice, ownerId, voiceId }
  const hasFreshness = !['current', 'unmade'].includes(voiceMaterialFreshness(profile))
  return (
    <main className="mt-4 pb-4 sm:mt-6 sm:pb-12">
      <Typography variant="title" className={profile.made ? 'sr-only sm:not-sr-only' : undefined}>
        {typeof title === 'function' ? title(context) : title}
      </Typography>
      {description && (
        <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
          {description}
        </Typography>
      )}
      {hasFreshness && (
        <div className="sm:mt-6">
          <VoiceMaterialFreshness profile={profile} />
        </div>
      )}
      <div className={hasFreshness || !profile.made ? 'mt-4 sm:mt-8' : 'sm:mt-8'}>
        {children(context)}
      </div>
    </main>
  )
}
