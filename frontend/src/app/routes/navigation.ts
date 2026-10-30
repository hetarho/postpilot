import { clipChannelPath, type ClipChannel } from '@/entities/clip-project'
import type { navigationI18n } from './navigation-i18n'

export const CHANNEL_DESTINATIONS = [
  { to: '/naver-blog', labelKey: 'blog' },
  { to: '/naver-clip', labelKey: 'clip' },
  { to: '/insta-clip', labelKey: 'instagram' },
] as const
const DESTINATIONS = [...CHANNEL_DESTINATIONS, { to: '/settings' }] as const
/** A channel's four sections (THEME-38), keyed by their list pages. Sections are siblings: each
 *  list page is top-level within its channel, with no back link and no recorded return. */
export type ChannelSection = 'work' | 'templates' | 'voices' | 'guidelines'
const SECTION_PATHS: Readonly<Record<string, ChannelSection>> = {
  '/naver-blog': 'work',
  '/naver-clip': 'work',
  '/insta-clip': 'work',
  '/templates': 'templates',
  '/video-templates': 'templates',
  '/voices': 'voices',
  '/guidelines': 'guidelines',
  '/video-guidelines': 'guidelines',
}
export function currentSection(pathname: string): ChannelSection | undefined {
  const path = navigationPathname(pathname)
  return Object.hasOwn(SECTION_PATHS, path) ? SECTION_PATHS[path] : undefined
}
/** The section tab row of 네이버 블로그 (no channel) or of one clip channel, whose shared 템플릿,
 *  말투 and 지침 carry the channel in their address. */
export function channelSections(channel?: ClipChannel): { key: ChannelSection; href: string }[] {
  if (!channel)
    return [
      { key: 'work', href: '/naver-blog' },
      { key: 'templates', href: '/templates' },
      { key: 'voices', href: '/voices' },
      { key: 'guidelines', href: '/guidelines' },
    ]
  return [
    { key: 'work', href: clipChannelPath(channel) },
    { key: 'templates', href: `/video-templates?channel=${channel}` },
    { key: 'voices', href: `/voices?channel=${channel}` },
    { key: 'guidelines', href: `/video-guidelines?channel=${channel}` },
  ]
}
export type PrimaryDestination = (typeof DESTINATIONS)[number]['to']
export type LocationKey = keyof typeof navigationI18n.ko.location
export interface LocationAncestor {
  href: string
  key: LocationKey
}
export interface RouteLocation {
  current: LocationKey
  ancestors: LocationAncestor[]
  destination?: PrimaryDestination
}
const HOME: LocationAncestor = { href: '/', key: 'creation' }
const NAVER_BLOG: LocationAncestor = { href: '/naver-blog', key: 'naverBlog' }
const MY_SETTINGS: LocationAncestor = { href: '/settings', key: 'settings' }
const PERSONAL: LocationAncestor = { href: '/settings#settings-personal', key: 'personal' }
const SPOKEN: LocationAncestor = { href: '/settings#settings-spoken', key: 'spoken' }
const AI: LocationAncestor = { href: '/settings#settings-ai', key: 'ai' }
const ADVANCED: LocationAncestor = { href: '/settings#settings-advanced', key: 'advanced' }
const TESTS: LocationAncestor = { href: '/tests', key: 'tests' }

export function navigationChannelFromHref(href: string): ClipChannel | undefined {
  const url = new URL(href, 'https://postpilot.invalid')
  if (navigationPathname(url.pathname) === '/insta-clip') return 'instagram_clip'
  if (navigationPathname(url.pathname) === '/naver-clip') return 'naver_clip'
  const channel = url.searchParams.get('channel')
  return channel === 'instagram_clip' || channel === 'naver_clip' ? channel : undefined
}
function structuralLocation(
  path: string,
  channel: ClipChannel | undefined,
): Omit<RouteLocation, 'destination'> {
  const clipHub: LocationAncestor = {
    href: clipChannelPath(channel ?? 'naver_clip'),
    key: channel === 'instagram_clip' ? 'instaClip' : 'naverClip',
  }
  // 말투 is the section every channel shares: a clip channel in its address places it under that
  // channel, and none under 네이버 블로그 (THEME-38).
  const voicesHub = channel ? clipHub : NAVER_BLOG
  if (path === '/') return { current: 'creation', ancestors: [] }
  if (path === '/naver-blog') return { current: 'naverBlog', ancestors: [] }
  if (path === '/naver-clip') return { current: 'naverClip', ancestors: [] }
  if (path === '/insta-clip') return { current: 'instaClip', ancestors: [] }
  if (
    path.startsWith('/tests/records/') ||
    path.startsWith('/posts/experiments/') ||
    path.startsWith('/ai-models/experiments/')
  )
    return {
      current: 'testRecord',
      ancestors: [MY_SETTINGS, ADVANCED, TESTS, { href: '/tests/history', key: 'testHistory' }],
    }
  if (
    path === '/ai-models/experiments' ||
    path === '/ai-models/leaderboard' ||
    path === '/tests/history'
  )
    return { current: 'testHistory', ancestors: [MY_SETTINGS, ADVANCED, TESTS] }
  if (/^\/posts\/[^/]+$/.test(path)) return { current: 'post', ancestors: [NAVER_BLOG] }
  if (/^\/clips\/[^/]+$/.test(path)) return { current: 'clip', ancestors: [clipHub] }
  if (path === '/tests') return { current: 'tests', ancestors: [MY_SETTINGS, ADVANCED] }
  if (/^\/tests\/[^/]+$/.test(path))
    return { current: 'test', ancestors: [MY_SETTINGS, ADVANCED, TESTS] }
  if (path === '/settings') return { current: 'settings', ancestors: [] }
  if (path === '/voices') return { current: 'voices', ancestors: [voicesHub] }
  if (path.startsWith('/voices/')) {
    const root = path.split('/').slice(0, 3).join('/')
    const parents = [
      voicesHub,
      { href: channel ? `/voices?channel=${channel}` : '/voices', key: 'voices' as const },
    ]
    const tail = path.split('/')[3]
    return tail
      ? {
          current: tail === 'materials' ? 'materials' : 'checks',
          ancestors: [...parents, { href: root, key: 'voice' }],
        }
      : { current: 'voice', ancestors: parents }
  }
  for (const [root, directory, item, add, parent] of [
    ['/templates', 'templates', 'template', 'newTemplate', NAVER_BLOG],
    ['/video-templates', 'videoTemplates', 'videoTemplate', 'newVideoTemplate', clipHub],
  ] as const)
    if (path === root || path.startsWith(root + '/'))
      return path === root
        ? { current: directory, ancestors: [parent] }
        : {
            current: path === root + '/new' ? add : item,
            ancestors: [
              parent,
              {
                href: root === '/video-templates' ? `${root}?channel=${channel}` : root,
                key: directory,
              },
            ],
          }
  if (path === '/guidelines') return { current: 'guidelines', ancestors: [NAVER_BLOG] }
  if (path === '/memories') return { current: 'memories', ancestors: [MY_SETTINGS, PERSONAL] }
  if (path === '/video-guidelines') return { current: 'videoGuidelines', ancestors: [clipHub] }
  if (path === '/spoken-voices')
    return { current: 'spokenVoices', ancestors: [MY_SETTINGS, SPOKEN] }
  if (path === '/spoken-voices/new')
    return {
      current: 'newSpokenVoice',
      ancestors: [MY_SETTINGS, SPOKEN, { href: '/spoken-voices', key: 'spokenVoices' }],
    }
  for (const [url, key] of [
    ['/ai-models', 'models'],
    ['/account', 'account'],
    ['/billing', 'billing'],
    ['/plans', 'plans'],
    ['/admin', 'admin'],
  ] as const)
    if (path === url) return { current: key, ancestors: [MY_SETTINGS, AI] }
  const billingPages = {
    '/billing/checkout': 'checkout',
    '/billing/method/success': 'paymentMethodSuccess',
    '/billing/method/fail': 'paymentMethodFail',
  } as const
  if (path in billingPages)
    return {
      current: billingPages[path as keyof typeof billingPages],
      ancestors: [MY_SETTINGS, AI, { href: '/billing', key: 'billing' }],
    }
  const adminPages = {
    '/admin/models': 'adminModels',
    '/admin/estimator': 'adminEstimator',
    '/admin/vouchers': 'adminVouchers',
    '/admin/costs': 'adminCosts',
  } as const
  if (path in adminPages)
    return {
      current: adminPages[path as keyof typeof adminPages],
      ancestors: [MY_SETTINGS, AI, { href: '/admin', key: 'admin' }],
    }
  if (path === '/setup') return { current: 'setup', ancestors: [HOME] }
  return { current: 'notFound', ancestors: [] }
}
/** One hierarchy supplies visible location and automatic entry eligibility. */
export function routeLocation(pathname: string, channel?: ClipChannel): RouteLocation {
  const path = navigationPathname(pathname)
  const canonicalPath = path === '/ai-models/compare' ? '/tests' : path
  const location = structuralLocation(canonicalPath, channel ?? navigationChannelFromHref(pathname))
  const root = location.ancestors[0]?.href ?? canonicalPath
  const destination = DESTINATIONS.find((item) => item.to === root)?.to
  return { ...location, destination }
}
export function currentDestination(pathname: string, channel?: ClipChannel) {
  return routeLocation(pathname, channel).destination
}
export function navigationPathname(href: string): string {
  return href.split(/[?#]/u, 1)[0].replace(/\/+$/u, '') || '/'
}
export function isTopLevelDestination(pathname: string): boolean {
  return (
    navigationPathname(pathname) === '/' ||
    DESTINATIONS.some((item) => item.to === navigationPathname(pathname)) ||
    currentSection(pathname) !== undefined
  )
}
export function isStructuralNavigationEntry(
  target: string,
  entry: string,
  channel?: ClipChannel,
): boolean {
  const targetPath = navigationPathname(target)
  const entryPath = navigationPathname(entry)
  if (isTopLevelDestination(targetPath) || targetPath === entryPath) return false
  const location = routeLocation(
    targetPath,
    channel ?? navigationChannelFromHref(target) ?? navigationChannelFromHref(entry),
  )
  return (
    location.ancestors.some((ancestor) => navigationPathname(ancestor.href) === entryPath) ||
    (location.current === 'test' && entryPath === '/tests/history')
  )
}
