import { useInitializeDefaultSelections } from '@/entities/model-catalog'
import { useSession } from '@/entities/session'
import { postReturnDestination } from '@/entities/post'
import { clipReturnDestination, clipChannelPath, useClipProject } from '@/entities/clip-project'
import { useWritingTestSource } from '@/entities/writing-test'
import { useEffect, useLayoutEffect, useRef } from 'react'
import { Link, Outlet, useNavigate, useRouter, useRouterState } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { takePendingGift } from '@/features/redeem-voucher'
import {
  Breadcrumb,
  Button,
  ContextualReturn,
  Logo,
  NavigationProvider,
  PromoStage,
  Typography,
  buttonStyles,
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
  CHANNEL_DESTINATIONS,
  isStructuralNavigationEntry,
  isTopLevelDestination,
  navigationPathname,
  navigationChannelFromHref,
  routeLocation,
} from './navigation'
import { NAV_DESKTOP_MEDIA_QUERY, NAV_SCROLL_RESTORE_TIMEOUT_MS } from './navigation-config'

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
  const channel =
    clipQuery.data?.channel ??
    navigationChannelFromHref(location.href) ??
    (storedChannel === 'instagram_clip' || storedChannel === 'naver_clip'
      ? storedChannel
      : undefined) ??
    (origin ? navigationChannelFromHref(origin) : undefined) ??
    clipReturn?.channel
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
            : fromChannel &&
                currentDestination(from, fromChannel) === clipChannelPath(fromChannel) &&
                /^\/video-(templates|guidelines)(?:\/|$)/.test(navigationPathname(to))
              ? clipChannelPath(fromChannel)
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
    current: t(`location.${metadata.current}`),
    ancestors: metadata.ancestors.map((item) => ({
      href: item.href,
      label: t(`location.${item.key}`),
    })),
    returnTo: returnHref && returnName ? { href: returnHref, label: returnName } : undefined,
    Link: NavigationLink,
  }
  return (
    <NavigationProvider value={navigationValue}>
      <div
        data-plans-shell={immersive || undefined}
        className="bg-surface-base text-content-primary relative isolate flex min-h-full flex-col"
      >
        {immersive && <PromoStage viewport />}
        <header
          ref={header}
          className={clsx(
            'min-h-header sticky top-0 z-30 has-[[aria-expanded=true]]:z-40',
            immersive ? 'bg-surface-raised/70 backdrop-blur-xl' : 'bg-surface-base',
          )}
        >
          <div
            className={clsx(
              'min-h-header lg:h-header mx-auto flex w-full max-w-7xl flex-wrap items-center gap-2 px-4 sm:px-6 lg:flex-nowrap lg:px-8',
            )}
          >
            <Link
              to="/"
              aria-label={t('home')}
              className="inline-flex min-h-11 min-w-11 shrink-0 items-center"
            >
              <Logo className="h-6 max-w-full" />
            </Link>
            <nav
              aria-label={t('primary')}
              className="order-last flex w-full gap-2 pb-2 lg:order-none lg:ml-6 lg:w-auto lg:pb-0"
            >
              {CHANNEL_DESTINATIONS.map(({ to, labelKey }) => (
                <Link
                  key={to}
                  to={to}
                  aria-current={current === to ? 'page' : undefined}
                  className={buttonStyles({
                    variant: 'ghost',
                    className: clsx(
                      'flex-1 lg:flex-none',
                      current === to &&
                        'bg-row-bg-active text-link-fg-current underline decoration-2 underline-offset-8',
                    ),
                  })}
                >
                  {t(labelKey)}
                </Link>
              ))}
            </nav>
            {!wide && !setup && (
              <div className="min-w-0 flex-1">
                <Breadcrumb
                  key={`${ownerId}:${location.href}`}
                  ariaLabel={t('locationLabel')}
                  compact
                />
              </div>
            )}
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
          {wide && pathname !== '/' && !setup && (
            <div className="mx-auto w-full max-w-7xl px-4 pb-3 sm:px-6 lg:px-8">
              <Breadcrumb ariaLabel={t('locationLabel')} />
              {origin &&
                origin !== parent?.href &&
                !/^\/(posts|clips)\/[^/]+$/.test(pathname) &&
                !pathname.startsWith('/tests/records/') &&
                pathname !== '/tests' &&
                !(pathname.startsWith('/tests/') && pathname !== '/tests/history') && (
                  <ContextualReturn />
                )}
            </div>
          )}
        </header>
        <div className="flex min-w-0 flex-1 flex-col">
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
      </div>
    </NavigationProvider>
  )
}
