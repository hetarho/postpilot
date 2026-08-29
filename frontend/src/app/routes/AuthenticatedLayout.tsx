import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { useLogout, useSession } from '@/entities/session'
import { Button } from '@/shared/ui'
import { endSession } from '../model/end-session'

/** The shell every signed-in screen renders inside.
 *
 *  The header lives here rather than in RootLayout so "only shown when there is a
 *  session" is structural: this component is the authenticated route's own component, so
 *  its guard has already resolved the session before it renders. Putting it at the root
 *  would make /login issue a GetMe it does not need. */
export function AuthenticatedLayout() {
  const { user } = useSession()
  const logout = useLogout()
  const navigate = useNavigate()

  // Where logout lands is the shell's decision, not the entity's — so the navigation
  // lives here rather than in useLogout. Awaiting is what orders it correctly:
  // mutateAsync resolves only after the session cache has been dropped, and navigating
  // first would let the guard read the stale entry and send us right back.
  //
  // Navigating only on success is deliberate. A failed Logout leaves the cookie valid,
  // so leaving would be theatre: the guard on /login would find the live session and
  // bounce the user back in. Better to stay and say it did not work.
  const onLogout = async () => {
    try {
      await logout.mutateAsync({})
    } catch {
      return
    }
    endSession()
    void navigate({ to: '/login', replace: true })
  }

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <header className="bg-surface-raised flex min-h-16 items-center justify-between px-4 sm:px-6">
        <nav className="flex items-center gap-4" aria-label="주요">
          <Link
            to="/posts"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm font-medium tracking-tight"
          >
            Postpilot
          </Link>
          <Link
            to="/voice"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
          >
            말투
          </Link>
          <Link
            to="/ai-models"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center text-sm"
          >
            AI 모델
          </Link>
        </nav>
        <div className="flex items-center gap-3">
          <span className="text-content-tertiary hidden font-mono text-xs sm:inline">
            {user?.id}
          </span>
          <Button
            variant="ghost"
            onClick={() => void onLogout()}
            disabled={logout.isPending}
            className="text-xs"
          >
            {logout.isPending ? '로그아웃 중…' : '로그아웃'}
          </Button>
        </div>
      </header>
      {logout.isError && (
        <p
          role="alert"
          className="bg-notice-danger-bg text-notice-danger-fg px-4 py-3 text-xs sm:px-6"
        >
          로그아웃하지 못했어요. 세션이 아직 살아 있으니 다시 시도해 주세요.
        </p>
      )}
      <div className="flex-1">
        <Outlet />
      </div>
    </div>
  )
}
