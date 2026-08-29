import { Link } from '@tanstack/react-router'
import { type VoiceProfile, isEmptyProfile } from '@/entities/voice-profile'
import { Notice, buttonStyles } from '@/shared/ui'

export function VoiceWarning({ profile }: { profile: VoiceProfile | undefined }) {
  if (!profile || !isEmptyProfile(profile)) return null

  return (
    <aside>
      <Notice tone="warning">
        {/* `w-full` drops the link onto its own line instead of leaving it inline at the end of
            the third wrapped row, where it was an ~84 × 20 target (§4.1). */}
        <span className="w-full min-w-0">
          문체 프로필이 비어 있어요 — 말투 탭에서 글 한 편을 학습시키면 내 문체로 나와요.
        </span>
        {/* The one thing to press in this box used to be its greyest, smallest text: `link-fg`
            resolves to `content-secondary` against the notice's gold. As a ghost button it takes
            the 44px floor with its own horizontal padding, and the notice's own foreground keeps
            it inside the §2.6 contract. The underline is its resting affordance — ghost has no
            fill until it is pressed, and there is no hover on a phone (§6). */}
        <Link
          to="/voice"
          className={buttonStyles({
            variant: 'ghost',
            className: 'text-notice-warning-fg shrink-0 underline',
          })}
        >
          말투 학습하기
        </Link>
      </Notice>
    </aside>
  )
}
