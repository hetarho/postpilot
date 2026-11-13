import { clipChannelPath, type ClipChannel } from '@/entities/clip-project'
import type { navigationI18n } from './navigation-i18n'

/** Guide: five VERBS, not five product areas — what a busy owner came to do. 만들기 asks what to
 *  make; 내 글 holds every post and clip; 내 스타일 is everything the owner has taught the AI;
 *  AI 고르기 picks and compares models; 나 is credits, plan and settings. They are the bottom tab
 *  bar on a phone and a floating pill on the desk, visible at every width (no hamburger). */
export const TAB_DESTINATIONS = [
  { to: '/create', labelKey: 'tabs.create' },
  { to: '/my', labelKey: 'tabs.mine' },
  { to: '/style', labelKey: 'tabs.style' },
  { to: '/ai-models', labelKey: 'tabs.ai' },
  { to: '/settings', labelKey: 'tabs.me' },
] as const
export type PrimaryDestination = (typeof TAB_DESTINATIONS)[number]['to']

/** Guide: a focused flow hides the tab bar, the way a pushed screen does on a phone — one task
 *  per screen, nothing competing with its one action. These are the creation spaces (the editor,
 *  the clip workspace), the template wizards and editors, the spoken-voice wizard, the comparison
 *  flow and setup. Each carries its own way back. */
export function isFocusedFlow(pathname: string): boolean {
  const path = navigationPathname(pathname)
  return (
    /^\/(posts|clips)\/[^/]+$/.test(path) ||
    /^\/(video-)?templates\/[^/]+$/.test(path) ||
    path === '/spoken-voices/new' ||
    path === '/setup' ||
    path === '/tests' ||
    path.startsWith('/tests/records/') ||
    (path.startsWith('/tests/') && path !== '/tests/history')
  )
}

/** The two tabs that hold several sibling pages: 내 글 (all work, or one channel's) and 내 스타일
 *  (말투 · 템플릿 · 지침 · 기억 · 목소리). Their pages share a large title and a chip row; each is
 *  top-level within its hub, with no back link and no recorded return. */
export type Hub = 'mine' | 'style'
export type MineSection = 'all' | 'blog' | 'naverClip' | 'instaClip'
export type StyleSection = 'voices' | 'templates' | 'guidelines' | 'memories' | 'spoken'
export type HubSection =
  { hub: 'mine'; key: MineSection } | { hub: 'style'; key: StyleSection; kind?: 'post' | 'video' }
const SECTION_PATHS: Readonly<Record<string, HubSection>> = {
  '/my': { hub: 'mine', key: 'all' },
  '/naver-blog': { hub: 'mine', key: 'blog' },
  '/naver-clip': { hub: 'mine', key: 'naverClip' },
  '/insta-clip': { hub: 'mine', key: 'instaClip' },
  '/voices': { hub: 'style', key: 'voices' },
  '/templates': { hub: 'style', key: 'templates', kind: 'post' },
  '/video-templates': { hub: 'style', key: 'templates', kind: 'video' },
  '/guidelines': { hub: 'style', key: 'guidelines', kind: 'post' },
  '/video-guidelines': { hub: 'style', key: 'guidelines', kind: 'video' },
  '/memories': { hub: 'style', key: 'memories' },
  '/spoken-voices': { hub: 'style', key: 'spoken' },
}
export function currentSection(pathname: string): HubSection | undefined {
  const path = navigationPathname(pathname)
  return Object.hasOwn(SECTION_PATHS, path) ? SECTION_PATHS[path] : undefined
}
/** The chip row of one hub, in its order. */
export const MINE_SECTIONS: readonly { key: MineSection; href: string }[] = [
  { key: 'all', href: '/my' },
  { key: 'blog', href: '/naver-blog' },
  { key: 'naverClip', href: '/naver-clip' },
  { key: 'instaClip', href: '/insta-clip' },
]
export const STYLE_SECTIONS: readonly { key: StyleSection; href: string }[] = [
  { key: 'voices', href: '/voices' },
  { key: 'templates', href: '/templates' },
  { key: 'guidelines', href: '/guidelines' },
  { key: 'memories', href: '/memories' },
  { key: 'spoken', href: '/spoken-voices' },
]
/** 템플릿 and 지침 each have a 글 side and a 영상 side; the switch between them keeps the clip
 *  channel the page was opened under. */
export function styleKinds(
  section: 'templates' | 'guidelines',
  channel?: ClipChannel,
): { key: 'post' | 'video'; href: string }[] {
  const scope = `?channel=${channel ?? 'naver_clip'}`
  return section === 'templates'
    ? [
        { key: 'post', href: '/templates' },
        { key: 'video', href: `/video-templates${scope}` },
      ]
    : [
        { key: 'post', href: '/guidelines' },
        { key: 'video', href: `/video-guidelines${scope}` },
      ]
}
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
const ME: LocationAncestor = { href: '/settings', key: 'settings' }
const AI_PICK: LocationAncestor = { href: '/ai-models', key: 'models' }
const TESTS: LocationAncestor = { href: '/tests', key: 'tests' }
const SPOKEN_VOICES: LocationAncestor = { href: '/spoken-voices', key: 'spokenVoices' }

/** Which tab a page's root belongs to. */
function tabOf(root: string): PrimaryDestination | undefined {
  const path = navigationPathname(root)
  if (TAB_DESTINATIONS.some((tab) => tab.to === path)) return path as PrimaryDestination
  const section = currentSection(path)
  if (section) return section.hub === 'mine' ? '/my' : '/style'
  return undefined
}

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
  if (path === '/') return { current: 'creation', ancestors: [] }
  if (path === '/create') return { current: 'create', ancestors: [] }
  if (path === '/my') return { current: 'mine', ancestors: [] }
  if (path === '/style') return { current: 'style', ancestors: [] }
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
      ancestors: [AI_PICK, { href: '/tests/history', key: 'testHistory' }],
    }
  if (
    path === '/ai-models/experiments' ||
    path === '/ai-models/leaderboard' ||
    path === '/tests/history'
  )
    return { current: 'testHistory', ancestors: [AI_PICK] }
  if (/^\/posts\/[^/]+$/.test(path)) return { current: 'post', ancestors: [NAVER_BLOG] }
  if (/^\/clips\/[^/]+$/.test(path)) return { current: 'clip', ancestors: [clipHub] }
  if (path === '/tests') return { current: 'tests', ancestors: [AI_PICK] }
  if (/^\/tests\/[^/]+$/.test(path)) return { current: 'test', ancestors: [AI_PICK, TESTS] }
  if (path === '/settings') return { current: 'settings', ancestors: [] }
  if (path === '/voices') return { current: 'voices', ancestors: [] }
  if (path.startsWith('/voices/')) {
    const root = path.split('/').slice(0, 3).join('/')
    const parents: LocationAncestor[] = [
      { href: channel ? `/voices?channel=${channel}` : '/voices', key: 'voices' },
    ]
    const tail = path.split('/')[3]
    return tail
      ? {
          current: tail === 'materials' ? 'materials' : 'checks',
          ancestors: [...parents, { href: root, key: 'voice' }],
        }
      : { current: 'voice', ancestors: parents }
  }
  for (const [root, directory, item, add] of [
    ['/templates', 'templates', 'template', 'newTemplate'],
    ['/video-templates', 'videoTemplates', 'videoTemplate', 'newVideoTemplate'],
  ] as const)
    if (path === root || path.startsWith(root + '/'))
      return path === root
        ? { current: directory, ancestors: [] }
        : {
            current: path === root + '/new' ? add : item,
            ancestors: [
              {
                href:
                  root === '/video-templates' ? `${root}?channel=${channel ?? 'naver_clip'}` : root,
                key: directory,
              },
            ],
          }
  if (path === '/guidelines') return { current: 'guidelines', ancestors: [] }
  if (path === '/memories') return { current: 'memories', ancestors: [] }
  if (path === '/video-guidelines') return { current: 'videoGuidelines', ancestors: [] }
  if (path === '/spoken-voices') return { current: 'spokenVoices', ancestors: [] }
  if (path === '/spoken-voices/new')
    return { current: 'newSpokenVoice', ancestors: [SPOKEN_VOICES] }
  if (path === '/ai-models') return { current: 'models', ancestors: [] }
  for (const [url, key] of [
    ['/account', 'account'],
    ['/billing', 'billing'],
    ['/plans', 'plans'],
    ['/admin', 'admin'],
  ] as const)
    if (path === url) return { current: key, ancestors: [ME] }
  const billingPages = {
    '/billing/checkout': 'checkout',
    '/billing/method/success': 'paymentMethodSuccess',
    '/billing/method/fail': 'paymentMethodFail',
  } as const
  if (path in billingPages)
    return {
      current: billingPages[path as keyof typeof billingPages],
      ancestors: [ME, { href: '/billing', key: 'billing' }],
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
      ancestors: [ME, { href: '/admin', key: 'admin' }],
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
  return { ...location, destination: tabOf(root) }
}
export function currentDestination(pathname: string, channel?: ClipChannel) {
  return routeLocation(pathname, channel).destination
}
export function navigationPathname(href: string): string {
  return href.split(/[?#]/u, 1)[0].replace(/\/+$/u, '') || '/'
}
export function isTopLevelDestination(pathname: string): boolean {
  const path = navigationPathname(pathname)
  return (
    path === '/' ||
    TAB_DESTINATIONS.some((item) => item.to === path) ||
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
