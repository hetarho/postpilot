import { Link, Outlet, useParams } from '@tanstack/react-router'
import { useSession } from '@/entities/session'
import { useVoices } from '@/entities/voice'
import { RestoreVoiceButton } from '@/features/restore-voice'
import { Badge, Button, Notice, TabLinks, type TabLink } from '@/shared/ui'

/** The five tabs of one voice, in one list so the row and the routes cannot drift — the same
 *  reason `AuthenticatedLayout` keeps one `DESTINATIONS`. They are sub-navigation inside one voice,
 *  not destinations of their own. */
const VOICE_TABS: readonly Omit<TabLink, 'params'>[] = [
  { to: '/voices/$voiceId', label: '프로필' },
  { to: '/voices/$voiceId/versions', label: '버전 기록' },
  { to: '/voices/$voiceId/import', label: '기존 글 가져오기' },
  { to: '/voices/$voiceId/rules', label: '대조 규칙' },
  { to: '/voices/$voiceId/validations', label: '프로필 검증' },
]

/** The frame of `/voices/$voiceId`: which voice this is, its state, and the tab row. The voice
 *  comes from the directory rather than from the profile so an unknown or foreign id can say so
 *  before any tab asks for a profile that does not exist. */
export function VoiceLayout() {
  const { voiceId } = useParams({ from: '/authenticated/voices/$voiceId' })
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { voices, isPending, isError, isFetching, refetch } = useVoices(ownerId)
  const voice = voices.find((candidate) => candidate.id === voiceId)

  return (
    <div className="mx-auto w-full max-w-2xl px-4 py-8 sm:px-6">
      <Link
        to="/voices"
        className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm underline"
      >
        ← 말투 목록
      </Link>
      {isError ? (
        <Notice tone="danger" role="alert" className="mt-4">
          <span>말투를 불러오지 못했어요.</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            다시 시도
          </Button>
        </Notice>
      ) : isPending ? (
        <p role="status" className="text-content-tertiary mt-4 text-sm">
          불러오는 중…
        </p>
      ) : !voice ? (
        <p role="alert" className="text-notice-danger-fg mt-4 text-sm">
          없는 말투예요.
        </p>
      ) : (
        <>
          <div className="mt-4 flex flex-wrap items-center gap-2">
            <h1 className="min-w-0 text-2xl font-semibold tracking-tight break-words">
              {voice.name}
            </h1>
            {voice.isDefault && <Badge tone="accent">기본</Badge>}
            {voice.deleted && <Badge tone="warning">삭제됨</Badge>}
          </div>
          {voice.deleted && (
            <Notice tone="warning" role="status" className="mt-4">
              <span className="w-full min-w-0">
                삭제된 말투예요. 기록은 볼 수 있지만, 복원하기 전에는 배우거나 고칠 수 없어요.
              </span>
              <RestoreVoiceButton
                ownerId={ownerId}
                voiceId={voice.id}
                variant="ghost"
                className="text-notice-warning-fg shrink-0 underline"
              />
            </Notice>
          )}
          <TabLinks
            items={VOICE_TABS.map((tab) => ({ ...tab, params: { voiceId } }))}
            ariaLabel="말투 설정"
            className="mt-4"
          />
          <Outlet />
        </>
      )}
    </div>
  )
}
