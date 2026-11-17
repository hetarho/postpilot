import { CLIP_LIST_PATH } from '@/entities/clip-project'
import type { navigationI18n } from './navigation-i18n'

/** Guide: four places, each a thing the owner holds — 홈 is the writing kit and the one 글쓰기,
 *  글 every post, 영상 every video, AI 토너먼트 the models that compete to write them. Account,
 *  plan, credits, theme and language live behind the avatar at the top right, not in a tab. They
 *  are the bottom tab bar on a phone and a floating pill on the desk, visible at every width. */
export const TAB_DESTINATIONS = [
  { to: '/', labelKey: 'tabs.home' },
  { to: '/posts', labelKey: 'tabs.posts' },
  { to: '/clips', labelKey: 'tabs.videos' },
  { to: '/ai-models', labelKey: 'tabs.ai' },
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

/** The writing kit: what the owner prepares before writing (말투 · 템플릿 · 지침 · 기억 · 목소리).
 *  Each is its own screen under 홈 — the readiness graphic is the way in — and carries one way
 *  back to it. 템플릿 and 지침 each have a 글 side and a 영상 side. */
export type KitSection = 'voices' | 'templates' | 'guidelines' | 'memories' | 'spoken'
export interface KitPage {
  key: KitSection
  kind?: 'post' | 'video'
}
const KIT_PATHS: Readonly<Record<string, KitPage>> = {
  '/voices': { key: 'voices' },
  '/templates': { key: 'templates', kind: 'post' },
  '/video-templates': { key: 'templates', kind: 'video' },
  '/guidelines': { key: 'guidelines', kind: 'post' },
  '/video-guidelines': { key: 'guidelines', kind: 'video' },
  '/memories': { key: 'memories' },
  '/spoken-voices': { key: 'spoken' },
}
export function currentKitPage(pathname: string): KitPage | undefined {
  const path = navigationPathname(pathname)
  return Object.hasOwn(KIT_PATHS, path) ? KIT_PATHS[path] : undefined
}
/** 템플릿 and 지침 each have a 글 side and a 영상 side; a video has no channel, so the switch
 *  links carry none (THEME-38). */
export function styleKinds(
  section: 'templates' | 'guidelines',
): { key: 'post' | 'video'; href: string }[] {
  return section === 'templates'
    ? [
        { key: 'post', href: '/templates' },
        { key: 'video', href: '/video-templates' },
      ]
    : [
        { key: 'post', href: '/guidelines' },
        { key: 'video', href: '/video-guidelines' },
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
  return undefined
}

function structuralLocation(path: string): Omit<RouteLocation, 'destination'> {
  const clipHub: LocationAncestor = {
    href: CLIP_LIST_PATH,
    key: 'videos',
  }
  if (path === '/') return { current: 'home', ancestors: [] }
  if (path === '/create') return { current: 'create', ancestors: [] }
  if (path === '/posts' || path === '/naver-blog' || path === '/my')
    return { current: 'posts', ancestors: [] }
  if (path === '/naver-clip' || path === '/insta-clip') return { current: 'videos', ancestors: [] }
  if (path === '/style') return { current: 'home', ancestors: [] }
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
    const parents: LocationAncestor[] = [HOME, { href: '/voices', key: 'voices' }]
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
        ? { current: directory, ancestors: [HOME] }
        : {
            current: path === root + '/new' ? add : item,
            ancestors: [
              HOME,
              {
                href: root,
                key: directory,
              },
            ],
          }
  if (path === '/guidelines') return { current: 'guidelines', ancestors: [HOME] }
  if (path === '/memories') return { current: 'memories', ancestors: [HOME] }
  if (path === '/video-guidelines') return { current: 'videoGuidelines', ancestors: [HOME] }
  if (path === '/spoken-voices') return { current: 'spokenVoices', ancestors: [HOME] }
  if (path === '/spoken-voices/new')
    return { current: 'newSpokenVoice', ancestors: [HOME, SPOKEN_VOICES] }
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
  return path === '/' || path === '/settings' || TAB_DESTINATIONS.some((item) => item.to === path)
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
