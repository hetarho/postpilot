import { Outlet } from '@tanstack/react-router'
import { TabLinks, type TabLink } from '@/shared/ui'

/** The five voice tabs, in one list so the row and the routes cannot drift — the same reason
 *  `AuthenticatedLayout` keeps one `DESTINATIONS`. These are sub-navigation inside 말투, not a
 *  fourth top-level destination. */
const VOICE_TABS: readonly TabLink[] = [
  { to: '/voice', label: '말투' },
  { to: '/voice/versions', label: '버전 기록' },
  { to: '/voice/import', label: '기존 글 가져오기' },
  { to: '/voice/rules', label: '대조 규칙' },
  { to: '/voice/validations', label: '프로필 검증' },
]

export function VoiceLayout() {
  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <p className="text-content-tertiary text-xs font-medium tracking-wide uppercase">Voice</p>
      <TabLinks items={VOICE_TABS} ariaLabel="말투 설정" className="mt-3" />
      <Outlet />
    </div>
  )
}
