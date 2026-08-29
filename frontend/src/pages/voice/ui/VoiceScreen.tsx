import type { ReactNode } from 'react'
import { FailureNotice } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { type VoiceProfile, useVoiceProfile } from '@/entities/voice-profile'

/** The frame every voice tab shares: the account's profile query, its two non-content states, and
 *  the screen's own heading. The profile query is the one read all five tabs need, so it stays
 *  shared here; every other list is fetched by the tab that renders it. */
export function VoiceScreen({
  title,
  description,
  children,
}: {
  title: string
  description?: ReactNode
  children: (context: { profile: VoiceProfile; ownerId: string }) => ReactNode
}) {
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { profile, isPending, isError, refetch } = useVoiceProfile(ownerId)

  if (isError) {
    return (
      <main className="mt-6">
        <FailureNotice error="문체 프로필을 불러오지 못했어요." onRetry={refetch} />
      </main>
    )
  }
  if (isPending || !profile) {
    return <main className="text-content-tertiary mt-6 text-sm">불러오는 중…</main>
  }
  return (
    <main className="mt-6 pb-12">
      <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
      {description && (
        <p className="text-content-secondary max-w-measure mt-2 text-sm leading-relaxed">
          {description}
        </p>
      )}
      <div className="mt-8">{children({ profile, ownerId })}</div>
    </main>
  )
}
