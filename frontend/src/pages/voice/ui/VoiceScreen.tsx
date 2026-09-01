import type { ReactNode } from 'react'
import { useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FailureNotice } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { type Voice, type VoiceProfile, useVoiceProfile } from '@/entities/voice'
import { Typography, typographyStyles } from '@/shared/ui'

export interface VoiceScreenContext {
  profile: VoiceProfile
  /** The voice as the profile response names it — the same row the layout shows. */
  voice: Voice
  ownerId: string
  voiceId: string
}

/** The frame every voice tab shares: THIS voice's profile query, its two non-content states, and
 *  the tab's own heading. The profile is the one read all five tabs need, so it stays shared here;
 *  every other list is fetched by the tab that renders it. The page's `h1` is the voice's name in
 *  the layout, so a tab's title is an `h2`. */
export function VoiceScreen({
  title,
  description,
  children,
}: {
  title: string
  description?: ReactNode
  children: (context: VoiceScreenContext) => ReactNode
}) {
  const { t } = useTranslation(['voices', 'common'])
  const { voiceId } = useParams({ from: '/authenticated/voices/$voiceId' })
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
  return (
    <main className="mt-6 pb-12">
      <Typography variant="title">{title}</Typography>
      {description && (
        <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
          {description}
        </Typography>
      )}
      <div className="mt-8">{children({ profile, voice: profile.voice, ownerId, voiceId })}</div>
    </main>
  )
}
