import { useInitializeDefaultSelections } from '@/entities/model-catalog'
import { useSession } from '@/entities/session'
import { postReturnDestination } from '@/entities/post'
import { clipReturnDestination } from '@/entities/clip-project'
import { useWritingTestSource } from '@/entities/writing-test'
import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import { Link, Outlet, useNavigate, useRouter, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Clapperboard, FileText, House, Trophy } from 'lucide-react'
import { takePendingGift } from '@/features/redeem-voucher'
import {
  Button,
  ContextualReturn,
  Logo,
  NavigationProvider,
  PromoStage,
  TabBar,
  Typography,
  chipStyles,
  pageStyles,
  typographyStyles,
  useMediaQuery,
  type NavigationLinkProps,
} from '@/shared/ui'
import { AccountMenu } from '@/widgets/account-menu'
import { CreditBadge } from '@/widgets/credit-badge'
import { InterfacePreferences } from '@/widgets/interface-preferences'
import { KitRing, KitSectionHead, useWritingKit, type KitElement } from '@/widgets/writing-kit'
import { endSession } from '../model/end-session'
import {
  entryHref,
  navigationParent,
  readNavigationEntry,
  rememberNavigationEntry,
} from '../model/navigation-entry'
import {
  currentDestination,
  currentSection,
  isFocusedFlow,
  isStructuralNavigationEntry,
  isTopLevelDestination,
  KIT_SECTIONS,
  navigationPathname,
  routeLocation,
  TAB_DESTINATIONS,
  VIDEO_SECTIONS,
  type HubSection,
} from './navigation'
import { NAV_DESKTOP_MEDIA_QUERY, NAV_SCROLL_RESTORE_TIMEOUT_MS } from './navigation-config'

/** Guide: one glyph per tab, so a destination is found by shape before it is read. */
const TAB_ICONS = {
  '/': House,
  '/posts': FileText,
  '/clips': Clapperboard,
  '/ai-models': Trophy,
} as const

function NavigationLink({ href, children, className, onClick, ...rest }: NavigationLinkProps) {
  const navigate = useNavigate()
  return (
    <a
      {...rest}
      href={href}
      className={className}
      onClick={(event) => {
        onClick?.(event)
        if (
          !event.defaultPrevented &&
          event.button === 0 &&
          !event.metaKey &&
          !event.ctrlKey &&
          !event.shiftKey &&
          !event.altKey
        ) {
          event.preventDefault()
          void navigate({ href })
        }
      }}
    >
      {children}
    </a>
  )
}

export function AuthenticatedLayout() {
  const { t } = useTranslation('nav')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const defaults = useInitializeDefaultSelections(ownerId)
  const navigate = useNavigate()
  const location = useRouterState({ select: (state) => state.location })
  const pathname = navigationPathname(location.pathname)
  const search = location.search as Record<string, unknown>
  const router = useRouter()
  const header = useRef<HTMLElement>(null)
  const wide = useMediaQuery(NAV_DESKTOP_MEDIA_QUERY)
  const postSlug = /^\/posts\/([^/]+)$/.exec(pathname)?.[1]
  const clipId = /^\/clips\/([^/]+)$/.exec(pathname)?.[1]
  const postReturn = postSlug
    ? postReturnDestination(ownerId, postSlug === 'new' ? undefined : postSlug)
    : undefined
  const clipReturn = clipId
    ? clipReturnDestination(ownerId, clipId === 'new' ? undefined : clipId)
    : undefined
  const stored = readNavigationEntry(ownerId, location.href)
  const accessibleParent = (href: string) => {
    const candidate = navigationParent(href)
    return candidate?.path === '/admin' && user?.plan !== 'master' ? undefined : candidate
  }
  const explicitParent =
    typeof search.entry === 'string' ? accessibleParent(search.entry) : undefined
  const explicit =
    !isTopLevelDestination(pathname) &&
    explicitParent &&
    explicitParent.path.split('#', 1)[0] !== pathname
      ? entryHref(explicitParent)
      : undefined
  const origin =
    explicit ?? (stored && accessibleParent(entryHref(stored)) ? entryHref(stored) : undefined)
  const current = currentDestination(pathname)
  const metadata = routeLocation(pathname)
  const sourceSlug =
    pathname === '/tests' || pathname.startsWith('/tests/records/')
      ? typeof search.source === 'string'
        ? search.source
        : typeof search.sourcePost === 'string'
          ? search.sourcePost
          : ''
      : ''
  const source = useWritingTestSource(ownerId, sourceSlug)
  const ownedSourceReturn =
    source.isSuccess && source.data?.context.sourcePostSlug === sourceSlug && sourceSlug
      ? `/posts/${encodeURIComponent(sourceSlug)}`
      : undefined
  // A template wizard opened from a post's 템플릿 question (TMPL-32) returns to that post: the
  // owner was writing it, and the template list was never where they came from.
  const templatePost =
    pathname === '/templates/new' && typeof search.post === 'string' ? search.post : ''
  const templatePostReturn = templatePost
    ? templatePost === 'new'
      ? '/posts/new'
      : `/posts/${encodeURIComponent(templatePost)}`
    : undefined
  // The guidelines page opened from a post's 확인 (POST-123) likewise names that post as its way
  // back, though it is a hub section that otherwise has none (THEME-58).
  const guidelinesPost =
    pathname === '/guidelines' && typeof search.post === 'string' ? search.post : ''
  const guidelinesPostReturn = guidelinesPost
    ? `/posts/${encodeURIComponent(guidelinesPost)}`
    : undefined
  const parent = metadata.ancestors.at(-1)
  const returnHref =
    postReturn?.href ??
    clipReturn?.href ??
    ownedSourceReturn ??
    templatePostReturn ??
    guidelinesPostReturn ??
    origin ??
    parent?.href
  const returnName =
    ownedSourceReturn || templatePostReturn || guidelinesPostReturn
      ? t('location.post')
      : parent && returnHref === parent.href
        ? t(`location.${parent.key}`)
        : returnHref
          ? t(`location.${routeLocation(returnHref).current}`)
          : undefined
  const immersive = pathname === '/plans'
  const quiet = pathname === '/setup'
  const setup = pathname === '/setup'
  // The editor, the clip workspace and the writing tests carry their own way back in their top
  // row; every other child route gets exactly one here (THEME-58).
  const ownReturn =
    /^\/(posts|clips)\/[^/]+$/.test(pathname) ||
    pathname === '/spoken-voices/new' ||
    pathname === '/billing/checkout' ||
    pathname === '/billing/method/success' ||
    pathname === '/billing/method/fail' ||
    pathname.startsWith('/tests/records/') ||
    pathname === '/tests' ||
    (pathname.startsWith('/tests/') && pathname !== '/tests/history')

  useEffect(() => {
    const token = takePendingGift()
    if (token) void navigate({ to: '/gift/$token', params: { token }, replace: true })
  }, [navigate])
  useEffect(() => {
    let restore: (() => void) | undefined
    let cleanup: (() => void) | undefined
    const before = router.subscribe('onBeforeNavigate', (event) => {
      cleanup?.()
      cleanup = undefined
      restore = undefined
      const from = event.fromLocation?.href
      const to = event.toLocation.href
      if (!ownerId || !from || from === to) return
      const inherited = readNavigationEntry(ownerId, from)
      const returnParent = navigationParent(to)
      if (inherited && returnParent && entryHref(inherited) === entryHref(returnParent)) {
        restore = () => {
          let finished = false
          let frame = 0
          const complete = () => {
            if (finished) return
            window.scrollTo(0, inherited.scrollY)
            cleanup?.()
          }
          const measure = () => {
            cancelAnimationFrame(frame)
            frame = requestAnimationFrame(() => {
              const height = Math.max(
                document.body.scrollHeight,
                document.documentElement.scrollHeight,
              )
              if (height - window.innerHeight >= inherited.scrollY) complete()
            })
          }
          const observer =
            typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure)
          observer?.observe(document.body)
          const timeout = window.setTimeout(complete, NAV_SCROLL_RESTORE_TIMEOUT_MS)
          cleanup = () => {
            finished = true
            cancelAnimationFrame(frame)
            observer?.disconnect()
            window.clearTimeout(timeout)
          }
          measure()
        }
        return
      }
      const targetEntry = (event.toLocation.search as Record<string, unknown>).entry
      const targetParent =
        typeof targetEntry === 'string' ? navigationParent(targetEntry) : undefined
      const fromParent = navigationParent(from)
      const explicitFrom =
        targetParent && fromParent && entryHref(targetParent) === entryHref(fromParent)
      const structural = isStructuralNavigationEntry(to, from)
      const entry =
        explicitFrom || (structural && navigationParent(from))
          ? from
          : structural && inherited && isStructuralNavigationEntry(to, entryHref(inherited))
            ? entryHref(inherited)
            : undefined
      if (entry)
        rememberNavigationEntry(
          ownerId,
          to,
          entry,
          currentDestination(event.fromLocation?.pathname ?? '/') ?? '/',
          navigationParent(from) ? window.scrollY : (inherited?.scrollY ?? 0),
          undefined,
          !!explicitFrom,
        )
    })
    const resolved = router.subscribe('onResolved', () => {
      restore?.()
      restore = undefined
    })
    return () => {
      before()
      resolved()
      cleanup?.()
    }
  }, [router, ownerId])
  useEffect(() => {
    if (explicit && (!stored || entryHref(stored) !== explicit))
      rememberNavigationEntry(ownerId, location.href, explicit, current ?? '/', 0, undefined, true)
  }, [explicit, stored, ownerId, location.href, current])
  useLayoutEffect(() => {
    const node = header.current
    if (!node) return
    const measure = () =>
      document.documentElement.style.setProperty(
        '--spacing-chrome',
        `${node.getBoundingClientRect().height}px`,
      )
    measure()
    const observer = typeof ResizeObserver === 'undefined' ? undefined : new ResizeObserver(measure)
    observer?.observe(node)
    window.addEventListener('resize', measure)
    return () => {
      observer?.disconnect()
      window.removeEventListener('resize', measure)
      document.documentElement.style.removeProperty('--spacing-chrome')
    }
  }, [])

  // A screen on a `PageFrame` draws its way back in its own column and claims it; the shell
  // draws one here only for a screen that has not (THEME-75).
  const [returnClaims, setReturnClaims] = useState(0)
  const claimReturn = useCallback(() => {
    setReturnClaims((count) => count + 1)
    return () => setReturnClaims((count) => count - 1)
  }, [])
  const navigationValue = {
    ancestors: metadata.ancestors.map((item) => ({
      href: item.href,
      label: t(`location.${item.key}`),
    })),
    returnTo: returnHref && returnName ? { href: returnHref, label: returnName } : undefined,
    Link: NavigationLink,
    claimReturn,
  }
  const section = currentSection(pathname)
  const backLink =
    !!guidelinesPostReturn ||
    (!setup &&
      !ownReturn &&
      !isTopLevelDestination(pathname) &&
      (navigationValue.ancestors.length > 0 || !!navigationValue.returnTo))
  // A hub section's return heads the hub's title, above the page, so it stays the shell's.
  const framedNavigation = { ...navigationValue, hasReturn: backLink && !section }
  const backNav = backLink && returnClaims === 0 && (
    <nav
      aria-label={t('returnLabel')}
      className="mx-auto w-full max-w-5xl px-4 pt-2 sm:px-6 lg:px-8"
    >
      <ContextualReturn className="-ml-4" />
    </nav>
  )
  // Guide: the tab bar is the thumb's home row on every screen except a focused flow, where the
  // flow's own bottom bar owns the thumb zone.
  const tabBar = !isFocusedFlow(pathname)
  const tabs = TAB_DESTINATIONS.map(({ to, labelKey }) => ({
    href: to,
    label: t(labelKey),
    icon: TAB_ICONS[to],
    current: current === to,
  }))
  return (
    <NavigationProvider value={framedNavigation}>
      <div
        data-plans-shell={immersive || undefined}
        data-tab-bar={tabBar ? 'shown' : undefined}
        className={clsx(
          // `overflow-x-clip`: a step's one-time slide-in starts 1.5rem to the right, and on a phone
          // that first frame must not widen the layout viewport. Clip, not hidden, so no scroll
          // container is made and the sticky header still sticks.
          'bg-surface-base text-content-primary relative isolate flex min-h-full flex-col overflow-x-clip',
          tabBar && 'pb-nav',
        )}
      >
        {immersive && <PromoStage viewport />}
        <header
          ref={header}
          className={clsx(
            'min-h-header sticky top-0 z-30 has-[[aria-expanded=true]]:z-40',
            immersive
              ? 'bg-surface-raised/70 backdrop-blur-xl'
              : 'bg-surface-base/90 backdrop-blur-xl',
          )}
        >
          {/* Guide: one slim row — the brand, then credits and the account. The destinations live
              in the bottom tab bar, where the thumb is. */}
          <div className="h-header mx-auto flex w-full max-w-5xl items-center gap-1 px-4 sm:gap-2 sm:px-6 lg:px-8">
            <Link
              to="/"
              aria-label={t('home')}
              className="inline-flex min-h-11 min-w-11 shrink-0 items-center"
            >
              <Logo className="h-5 max-w-full sm:h-6" />
            </Link>
            <div className="ml-auto flex shrink-0 items-center gap-0 lg:gap-2">
              {!quiet && wide && <CreditBadge />}
              {/* Theme and language live inside the avatar at every width (owner 2026-11-17). */}
              <AccountMenu
                preferences={<InterfacePreferences layout="rows" />}
                onLoggedOut={() => {
                  endSession()
                  void navigate({ to: '/login', replace: true })
                }}
              />
            </div>
          </div>
        </header>
        {/* Guide: a hub's pages (the writing kit's four elements, 영상 and what shapes a video)
            open on the hub's large title, with the sibling pages as a chip row under it that
            scrolls sideways rather than wrapping. The page keeps its own (visually hidden) `h1`,
            so the title is the sighted reader's copy of the hub's name, not a second heading in
            the outline. */}
        {/* A hub page holds a back link only when a post opened it, and then it heads the page,
            over the hub's title (THEME-58). */}
        {section && backNav}
        {section && (
          // The directory column's own width, so the title and chips line up with the list.
          <div className={pageStyles({ width: 'wide', className: 'pt-2 pb-0 sm:pt-6 sm:pb-0' })}>
            <p
              aria-hidden="true"
              className={typographyStyles({ variant: 'largeTitle', className: 'truncate' })}
            >
              {t(`hub.${section.hub}`)}
            </p>
            <HubChips section={section} ownerId={ownerId} />
            {section.hub === 'kit' && (
              <KitSectionHead element={section.key} ownerId={ownerId} className="mt-4 sm:mt-6" />
            )}
          </div>
        )}
        <div className="flex min-w-0 flex-1 flex-col">
          {!section && backNav}
          {defaults.isError && (
            <div className="mx-auto w-full max-w-7xl px-4 py-3 sm:px-6 lg:px-8">
              <Typography variant="body" role="status">
                {t('aiPreparingFailed')}{' '}
                <Button variant="ghost" onClick={defaults.retry}>
                  {t('retryAI')}
                </Button>
              </Typography>
            </div>
          )}
          <Outlet />
        </div>
        {tabBar && <TabBar items={tabs} ariaLabel={t('primary')} />}
      </div>
    </NavigationProvider>
  )
}

/** A hub's sibling pages as one chip row. The writing kit's chips each carry their element's ring,
 *  read from the same lists as the home graphic, so the kit reads the same on every page of it. */
function HubChips({ section, ownerId }: { section: HubSection; ownerId: string }) {
  const { t } = useTranslation('nav')
  return (
    <nav
      aria-label={t('sections')}
      className="-mx-4 mt-4 flex gap-2 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:px-0"
    >
      {section.hub === 'kit' ? (
        <KitChips current={section.key} ownerId={ownerId} />
      ) : (
        VIDEO_SECTIONS.map(({ key, href }) => (
          <Link
            key={key}
            to={href}
            aria-current={section.key === key ? 'page' : undefined}
            className={chipStyles({ current: section.key === key })}
          >
            {t(`video.${key}`)}
          </Link>
        ))
      )}
    </nav>
  )
}

function KitChips({ current, ownerId }: { current: KitElement; ownerId: string }) {
  const { t } = useTranslation('nav')
  const kit = useWritingKit(ownerId)
  return KIT_SECTIONS.map(({ key, href }) => (
    <Link
      key={key}
      to={href}
      aria-current={current === key ? 'page' : undefined}
      className={chipStyles({ current: current === key, className: 'gap-2 pl-2.5' })}
    >
      <KitRing reading={kit.entries[key].reading} size="sm" />
      {t(`kit.${key}`)}
    </Link>
  ))
}
