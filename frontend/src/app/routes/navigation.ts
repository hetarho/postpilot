import { CLIP_LIST_PATH } from '@/entities/clip-project'
import type { navigationI18n } from './navigation-i18n'

/** Guide: four places. 홈 is the owner's writing kit — 말투 · 템플릿 · 지침 · 기억 · 글 쓰는 AI —
 *  and the one 글쓰기; 글 holds every blog post; 영상 holds every video and the kit that makes one;
 *  AI 토너먼트 pits models against each other and keeps the ones in use. Account, plan and settings
 *  live behind the avatar, not in a tab. The bar is the bottom tab bar on a phone and a floating
 *  pill on the desk, visible at every width (no hamburger). */
export const TAB_DESTINATIONS = [
  { to: '/', labelKey: 'tabs.home' },
  { to: '/posts', labelKey: 'tabs.posts' },
  { to: '/clips', labelKey: 'tabs.videos' },
  { to: '/ai-models', labelKey: 'tabs.tournament' },
] as const
export type PrimaryDestination = (typeof TAB_DESTINATIONS)[number]['to']

/** Guide: a focused flow hides the tab bar, the way a pushed screen does on a phone — one task
 *  per screen, nothing competing with its one action. These are the creation spaces (the editor,
 *  the clip workspace), the template screens, the spoken-voice wizard, the comparison flow and
 *  setup. Each carries its own way back. */
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

/** The two tabs that open on a list: 글 (every post) and 영상 (every video, with the three things
 *  that make a video — its templates, its rules and the voice that reads it — as sibling pages).
 *  Their pages share a large title, a line saying what the tab holds and, for 영상, a chip row;
 *  each is top-level within its tab, with no back link and no recorded return. */
export type Hub = 'posts' | 'videos'
export type VideoSection = 'list' | 'templates' | 'guidelines' | 'spoken'
export type HubSection = { hub: 'posts'; key: 'list' } | { hub: 'videos'; key: VideoSection }
const SECTION_PATHS: Readonly<Record<string, HubSection>> = {
  '/posts': { hub: 'posts', key: 'list' },
  '/clips': { hub: 'videos', key: 'list' },
  // The earlier list addresses replace themselves with these two; until they have, the hub chrome
  // stays mounted rather than flashing away and back (THEME-72).
  '/naver-blog': { hub: 'posts', key: 'list' },
  '/naver-clip': { hub: 'videos', key: 'list' },
  '/insta-clip': { hub: 'videos', key: 'list' },
  '/video-templates': { hub: 'videos', key: 'templates' },
  '/video-guidelines': { hub: 'videos', key: 'guidelines' },
  '/spoken-voices': { hub: 'videos', key: 'spoken' },
}
export function currentSection(pathname: string): HubSection | undefined {
  const path = navigationPathname(pathname)
  return Object.hasOwn(SECTION_PATHS, path) ? SECTION_PATHS[path] : undefined
}
/** 영상's chip row, in its order: the videos first, then what makes one. */
export const VIDEO_SECTIONS: readonly { key: VideoSection; href: string }[] = [
  { key: 'list', href: CLIP_LIST_PATH },
  { key: 'templates', href: '/video-templates' },
  { key: 'guidelines', href: '/video-guidelines' },
  { key: 'spoken', href: '/spoken-voices' },
]
/** The writing kit's screens: each is a child of 홈, which draws all of them as one picture, so
 *  each opens with a way back to it and lights the 홈 tab. */
export const KIT_PATHS = ['/voices', '/templates', '/guidelines', '/memories'] as const
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
const HOME: LocationAncestor = { href: '/', key: 'home' }
const POSTS: LocationAncestor = { href: '/posts', key: 'posts' }
const ME: LocationAncestor = { href: '/settings', key: 'settings' }
const AI_PICK: LocationAncestor = { href: '/ai-models', key: 'models' }
const TESTS: LocationAncestor = { href: '/tests', key: 'tests' }
const SPOKEN_VOICES: LocationAncestor = { href: '/spoken-voices', key: 'spokenVoices' }

/** Which tab a page's root belongs to. */
function tabOf(root: string): PrimaryDestination | undefined {
  const path = navigationPathname(root)
  if (TAB_DESTINATIONS.some((tab) => tab.to === path)) return path as PrimaryDestination
  if ((KIT_PATHS as readonly string[]).includes(path)) return '/'
  const section = currentSection(path)
  if (section) return section.hub === 'posts' ? '/posts' : '/clips'
  return undefined
}

function structuralLocation(path: string): Omit<RouteLocation, 'destination'> {
  const clipHub: LocationAncestor = {
    href: CLIP_LIST_PATH,
    key: 'videos',
  }
  if (path === '/') return { current: 'home', ancestors: [] }
  if (path === '/posts') return { current: 'posts', ancestors: [] }
  if (path === CLIP_LIST_PATH) return { current: 'videos', ancestors: [] }
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
  if (/^\/posts\/[^/]+$/.test(path)) return { current: 'post', ancestors: [POSTS] }
  if (/^\/clips\/[^/]+$/.test(path)) return { current: 'clip', ancestors: [clipHub] }
  if (path === '/tests' || path === '/tests/photos')
    return { current: 'tests', ancestors: [AI_PICK] }
  if (/^\/tests\/[^/]+$/.test(path)) return { current: 'test', ancestors: [AI_PICK, TESTS] }
  if (path === '/settings') return { current: 'settings', ancestors: [] }
  if (path === '/voices') return { current: 'voices', ancestors: [HOME] }
  if (path.startsWith('/voices/')) {
    const root = path.split('/').slice(0, 3).join('/')
    const parents: LocationAncestor[] = [{ href: '/voices', key: 'voices' }]
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
        ? { current: directory, ancestors: root === '/templates' ? [HOME] : [] }
        : {
            current: path === root + '/new' ? add : item,
            ancestors: [
              {
                href: root,
                key: directory,
              },
            ],
          }
  if (path === '/guidelines') return { current: 'guidelines', ancestors: [HOME] }
  if (path === '/memories') return { current: 'memories', ancestors: [HOME] }
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
export function routeLocation(pathname: string): RouteLocation {
  const path = navigationPathname(pathname)
  const canonicalPath = path === '/ai-models/compare' ? '/tests' : path
  const location = structuralLocation(canonicalPath)
  const root = location.ancestors[0]?.href ?? canonicalPath
  return { ...location, destination: tabOf(root) }
}
export function currentDestination(pathname: string) {
  return routeLocation(pathname).destination
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
export function isStructuralNavigationEntry(target: string, entry: string): boolean {
  const targetPath = navigationPathname(target)
  const entryPath = navigationPathname(entry)
  if (isTopLevelDestination(targetPath) || targetPath === entryPath) return false
  const location = routeLocation(targetPath)
  return (
    location.ancestors.some((ancestor) => navigationPathname(ancestor.href) === entryPath) ||
    (location.current === 'test' && entryPath === '/tests/history')
  )
}
