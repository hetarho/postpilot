import { useInitializeDefaultSelections } from '@/entities/model-catalog'
import { useSession } from '@/entities/session'
import { postReturnDestination } from '@/entities/post'
import { clipReturnDestination, useClipProject } from '@/entities/clip-project'
import { useWritingTestSource } from '@/entities/writing-test'
import { useEffect, useLayoutEffect, useRef } from 'react'
import { Link, Outlet, useNavigate, useRouter, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Bot, CircleUserRound, Files, Palette, SquarePen } from 'lucide-react'
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
  MINE_SECTIONS,
  navigationPathname,
  navigationChannelFromHref,
  routeLocation,
  STYLE_SECTIONS,
  styleKinds,
  TAB_DESTINATIONS,
} from './navigation'
import { NAV_DESKTOP_MEDIA_QUERY, NAV_SCROLL_RESTORE_TIMEOUT_MS } from './navigation-config'

/** Guide: one glyph per tab, so a destination is found by shape before it is read. */
const TAB_ICONS = {
  '/create': SquarePen,
  '/my': Files,
  '/style': Palette,
  '/ai-models': Bot,
  '/settings': CircleUserRound,
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
  const clipQuery = useClipProject(ownerId, clipId && clipId !== 'new' ? clipId : undefined)
  const clipReturn = clipId
    ? clipReturnDestination(
        ownerId,
        clipId === 'new' ? undefined : clipId,
        undefined,
        clipQuery.data?.channel ?? navigationChannelFromHref(location.href),
      )
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
  const storedChannel = stored?.filters.channel
  // 말투's list page belongs to the channel its own address names, and to 네이버 블로그 without
  // one, so a reload or a shared link lands under the same channel (THEME-38).
  const channel =
    pathname === '/voices'
      ? navigationChannelFromHref(location.href)
      : (clipQuery.data?.channel ??
        navigationChannelFromHref(location.href) ??
        (storedChannel === 'instagram_clip' || storedChannel === 'naver_clip'
          ? storedChannel
          : undefined) ??
        (origin ? navigationChannelFromHref(origin) : undefined) ??
        clipReturn?.channel)
  const channelRef = useRef(channel)
  useLayoutEffect(() => {
    channelRef.current = channel
  }, [channel])
  const current = currentDestination(pathname, channel)
  const metadata = routeLocation(pathname, channel)
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
  const parent = metadata.ancestors.at(-1)
  const returnHref =
    postReturn?.href ?? clipReturn?.href ?? ownedSourceReturn ?? origin ?? parent?.href
  const returnName = ownedSourceReturn
    ? t('location.post')
    : parent && returnHref === parent.href
      ? t(`location.${parent.key}`)
      : returnHref
        ? t(`location.${routeLocation(returnHref, channel).current}`)
        : undefined
  const immersive = pathname === '/plans'
  const quiet = pathname === '/' || pathname === '/setup'
  const setup = pathname === '/setup'
  // The editor, the clip workspace and the writing tests carry their own way back in their top
  // row; every other child route gets exactly one here (THEME-58).
  const ownReturn =
    /^\/(posts|clips)\/[^/]+$/.test(pathname) ||
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
      const fromChannel =
        navigationChannelFromHref(from) ??
        (inherited ? navigationChannelFromHref(entryHref(inherited)) : undefined) ??
        channelRef.current
      const structural = isStructuralNavigationEntry(to, from, fromChannel)
      const entry =
        explicitFrom || (structural && navigationParent(from))
          ? from
          : structural &&
              inherited &&
              isStructuralNavigationEntry(to, entryHref(inherited), fromChannel)
            ? entryHref(inherited)
            : undefined
      const parentEntry = entry ? navigationParent(entry) : undefined
      const scopedEntry =
        entry &&
        parentEntry &&
        fromChannel &&
        /^\/video-(templates|guidelines)(?:\/|$)/.test(parentEntry.path)
          ? entryHref({ ...parentEntry, filters: { ...parentEntry.filters, channel: fromChannel } })
          : entry
      if (scopedEntry)
        rememberNavigationEntry(
          ownerId,
          to,
          scopedEntry,
          currentDestination(event.fromLocation?.pathname ?? '/', fromChannel) ?? '/',
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

  const navigationValue = {
    ancestors: metadata.ancestors.map((item) => ({
      href: item.href,
      label: t(`location.${item.key}`),
    })),
    returnTo: returnHref && returnName ? { href: returnHref, label: returnName } : undefined,
    Link: NavigationLink,
  }
  const section = currentSection(pathname)
  const backLink =
    !setup &&
    !ownReturn &&
    !isTopLevelDestination(pathname) &&
    (navigationValue.ancestors.length > 0 || !!navigationValue.returnTo)
  // Guide: the tab bar is the thumb's home row on every screen except a focused flow, where the
  // flow's own bottom bar owns the thumb zone.
  const tabBar = !isFocusedFlow(pathname)
  const tabs = TAB_DESTINATIONS.map(({ to, labelKey }) => ({
    href: to,
    label: t(labelKey),
    icon: TAB_ICONS[to],
    current: current === to,
  }))
  const clipScope = channel ?? navigationChannelFromHref(location.href)
  const chips =
    section?.hub === 'mine'
      ? MINE_SECTIONS.map(({ key, href }) => ({
          key,
          href,
          label: t(`mine.${key}`),
          current: section.key === key,
        }))
      : section?.hub === 'style'
        ? STYLE_SECTIONS.map(({ key, href }) => ({
            key,
            href,
            label: t(`style.${key}`),
            current: section.key === key,
          }))
        : []
  const kinds =
    section?.hub === 'style' && (section.key === 'templates' || section.key === 'guidelines')
      ? styleKinds(section.key, clipScope).map(({ key, href }) => ({
          key,
          href,
          label: t(`kinds.${key}`),
          current: section.kind === key,
        }))
      : []
  return (
    <NavigationProvider value={navigationValue}>
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
              {wide && (
                <div className="flex">
                  <InterfacePreferences />
                </div>
              )}
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
        {/* Guide: a hub's pages (내 글's channels, 내 스타일's sections) open on the hub's large
            title, with the sibling pages as a chip row under it that scrolls sideways rather than
            wrapping. The page keeps its own (visually hidden) `h1`, so the title is the sighted
            reader's copy of the hub's name, not a second heading in the outline. */}
        {section && (
          // The directory column's own width, so the title and chips line up with the list.
          <div className={pageStyles({ width: 'wide', className: 'pt-2 pb-0 sm:pt-6 sm:pb-0' })}>
            <p
              aria-hidden="true"
              className={typographyStyles({ variant: 'largeTitle', className: 'truncate' })}
            >
              {t(`hub.${section.hub}`)}
            </p>
            <nav
              aria-label={t('sections')}
              className="-mx-4 mt-4 flex gap-2 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:px-0"
            >
              {chips.map((chip) => (
                <Link
                  key={chip.key}
                  to={chip.href}
                  aria-current={chip.current ? 'page' : undefined}
                  className={chipStyles({ current: chip.current })}
                >
                  {chip.label}
                </Link>
              ))}
            </nav>
            {kinds.length > 0 && (
              // 템플릿 and 지침 each have a 글 side and a 영상 side: a small two-way switch under
              // the chips rather than a third row of tabs.
              <nav aria-label={t('kinds.aria')} className="mt-1 flex gap-1">
                {kinds.map((kind) => (
                  <Link
                    key={kind.key}
                    to={kind.href}
                    aria-current={kind.current ? 'page' : undefined}
                    className={typographyStyles({
                      variant: 'label',
                      className: clsx(
                        'inline-flex min-h-11 items-center rounded-full px-4',
                        kind.current
                          ? 'bg-badge-accent-bg text-badge-accent-fg'
                          : 'text-content-secondary hover:text-content-primary',
                      ),
                    })}
                  >
                    {kind.label}
                  </Link>
                ))}
              </nav>
            )}
          </div>
        )}
        <div className="flex min-w-0 flex-1 flex-col">
          {backLink && (
            <nav
              aria-label={t('returnLabel')}
              className="mx-auto w-full max-w-5xl px-4 pt-2 sm:px-6 lg:px-8"
            >
              <ContextualReturn className="-ml-4" />
            </nav>
          )}
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
