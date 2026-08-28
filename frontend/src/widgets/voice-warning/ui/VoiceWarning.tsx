import { Link } from '@tanstack/react-router'
import { type VoiceProfile, isEmptyProfile } from '@/entities/voice-profile'

export function VoiceWarning({ profile }: { profile: VoiceProfile | undefined }) {
  if (!profile || !isEmptyProfile(profile)) return null

  return (
    <aside className="bg-notice-warning-bg text-notice-warning-fg rounded-md px-3 py-3 text-sm">
      <span>문체 프로필이 비어 있어요 — 말투 탭에서 글 한 편을 학습시키면 내 문체로 나와요.</span>{' '}
      <Link to="/voice" className="text-link-fg hover:text-link-fg-hover font-medium underline">
        말투 학습하기
      </Link>
    </aside>
  )
}
