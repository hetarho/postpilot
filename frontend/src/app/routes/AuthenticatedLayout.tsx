import { Link, Outlet, useNavigate } from '@tanstack/react-router'
import { useLogout, useSession } from '@/entities/session'
import { discardDraftQueues } from '@/features/save-draft'

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
    // Same reason as in auth-redirect: an unfinished draft must not follow the device to
    // the next account.
    discardDraftQueues()
    void navigate({ to: '/login', replace: true })
  }

  return (
    <div className="flex min-h-full flex-col bg-neutral-950 text-neutral-100">
      <header className="flex items-center justify-between border-b border-neutral-900 px-6 py-3">
        <Link to="/posts" className="text-sm font-medium tracking-tight">
          Postpilot
        </Link>
        <div className="flex items-center gap-3">
          <span className="font-mono text-xs text-neutral-400">{user?.id}</span>
          <button
            type="button"
            onClick={() => void onLogout()}
            disabled={logout.isPending}
            className="rounded-md border border-neutral-800 px-2.5 py-1 text-xs text-neutral-300 disabled:opacity-50"
          >
            로그아웃
          </button>
        </div>
      </header>
      {logout.isError && (
        <p role="alert" className="border-b border-red-900/50 bg-red-950/40 px-6 py-2 text-xs text-red-300">
          로그아웃하지 못했어요. 세션이 아직 살아 있으니 다시 시도해 주세요.
        </p>
      )}
      <div className="flex-1">
        <Outlet />
      </div>
    </div>
  )
}
