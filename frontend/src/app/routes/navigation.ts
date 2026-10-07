import { House, Archive, Settings2, FlaskConical } from 'lucide-react'
import type { navigationI18n } from './navigation-i18n'

export const DESTINATIONS = [
  { to: '/', labelKey: 'launch', icon: House },
  { to: '/tests', labelKey: 'tests', icon: FlaskConical },
  { to: '/settings', labelKey: 'settings', icon: Settings2 },
  { to: '/library', labelKey: 'history', icon: Archive, secondary: true },
] as const
export type PrimaryDestination = (typeof DESTINATIONS)[number]['to']
export function currentDestination(pathname: string, creationOrigin = false) {
  return routeLocation(pathname, creationOrigin).destination
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
const SETTINGS: LocationAncestor = { href: '/settings', key: 'settings' }
const WRITING: LocationAncestor = { href: '/settings#settings-writing', key: 'writing' }
const VIDEO: LocationAncestor = { href: '/settings#settings-video', key: 'video' }
const AI: LocationAncestor = { href: '/settings#settings-ai', key: 'ai' }
const HISTORY: LocationAncestor = { href: '/library', key: 'history' }
const TESTS: LocationAncestor = { href: '/tests', key: 'tests' }
function structuralLocation(
  pathname: string,
  creationOrigin: boolean,
): Omit<RouteLocation, 'destination'> {
  if (pathname === '/') return { current: 'creation', ancestors: [] }
  if (pathname === '/posts/new' || (creationOrigin && /^\/posts\/[^/]+$/.test(pathname)))
    return { current: 'post', ancestors: [HOME] }
  if (pathname === '/clips/new' || (creationOrigin && /^\/clips\/[^/]+$/.test(pathname)))
    return { current: 'clip', ancestors: [HOME] }
  if (pathname === '/library') return { current: 'history', ancestors: [] }
  if (pathname === '/posts') return { current: 'posts', ancestors: [HISTORY] }
  if (pathname === '/clips') return { current: 'clips', ancestors: [HISTORY] }
  if (
    pathname.startsWith('/tests/records/') ||
    pathname.startsWith('/posts/experiments/') ||
    pathname.startsWith('/ai-models/experiments/')
  )
    return {
      current: 'testRecord',
      ancestors: [TESTS, { href: '/tests/history', key: 'testHistory' }],
    }
  if (pathname === '/ai-models/compare') return { current: 'tests', ancestors: [] }
  if (pathname === '/ai-models/experiments' || pathname === '/ai-models/leaderboard')
    return { current: 'testHistory', ancestors: [TESTS] }
  if (/^\/posts\/[^/]+$/.test(pathname))
    return { current: 'post', ancestors: [HISTORY, { href: '/posts', key: 'posts' }] }
  if (/^\/clips\/[^/]+$/.test(pathname))
    return { current: 'clip', ancestors: [HISTORY, { href: '/clips', key: 'clips' }] }
  if (pathname === '/tests') return { current: 'tests', ancestors: [] }
  if (pathname === '/tests/history') return { current: 'testHistory', ancestors: [TESTS] }
  if (/^\/tests\/[^/]+$/.test(pathname)) return { current: 'test', ancestors: [TESTS] }
  if (pathname === '/settings') return { current: 'settings', ancestors: [] }
  if (pathname === '/voices') return { current: 'voices', ancestors: [SETTINGS, WRITING] }
  if (pathname.startsWith('/voices/')) {
    const root = pathname.split('/').slice(0, 3).join('/')
    const parents = [SETTINGS, WRITING, { href: '/voices', key: 'voices' as const }]
    const tail = pathname.split('/')[3]
    return tail
      ? {
          current: tail === 'materials' ? 'materials' : 'checks',
          ancestors: [...parents, { href: root, key: 'voice' }],
        }
      : { current: 'voice', ancestors: parents }
  }
  for (const [root, directory, item, add, parent] of [
    ['/templates', 'templates', 'template', 'newTemplate', WRITING],
    ['/video-templates', 'videoTemplates', 'videoTemplate', 'newVideoTemplate', VIDEO],
  ] as const)
    if (pathname === root || pathname.startsWith(root + '/')) {
      if (pathname === root) return { current: directory, ancestors: [SETTINGS, parent] }
      return {
        current: pathname === root + '/new' ? add : item,
        ancestors: [SETTINGS, parent, { href: root, key: directory }],
      }
    }
  if (pathname === '/guidelines') return { current: 'guidelines', ancestors: [SETTINGS, WRITING] }
  if (pathname === '/memories') return { current: 'memories', ancestors: [SETTINGS, WRITING] }
  if (pathname === '/video-guidelines')
    return { current: 'videoGuidelines', ancestors: [SETTINGS, VIDEO] }
  if (pathname === '/spoken-voices')
    return { current: 'spokenVoices', ancestors: [SETTINGS, VIDEO] }
  if (pathname === '/spoken-voices/new')
    return {
      current: 'newSpokenVoice',
      ancestors: [SETTINGS, VIDEO, { href: '/spoken-voices', key: 'spokenVoices' }],
    }
  if (pathname === '/ai-models') return { current: 'models', ancestors: [SETTINGS, AI] }
  if (pathname === '/account') return { current: 'account', ancestors: [SETTINGS, AI] }
  if (pathname === '/billing') return { current: 'billing', ancestors: [SETTINGS, AI] }
  const billingPages = {
    '/billing/checkout': 'checkout',
    '/billing/method/success': 'paymentMethodSuccess',
    '/billing/method/fail': 'paymentMethodFail',
  } as const
  if (pathname in billingPages)
    return {
      current: billingPages[pathname as keyof typeof billingPages],
      ancestors: [SETTINGS, AI, { href: '/billing', key: 'billing' }],
    }
  if (pathname === '/plans') return { current: 'plans', ancestors: [SETTINGS, AI] }
  if (pathname === '/admin') return { current: 'admin', ancestors: [SETTINGS, AI] }
  const adminPages = {
    '/admin/models': 'adminModels',
    '/admin/estimator': 'adminEstimator',
    '/admin/vouchers': 'adminVouchers',
    '/admin/costs': 'adminCosts',
  } as const
  if (pathname in adminPages)
    return {
      current: adminPages[pathname as keyof typeof adminPages],
      ancestors: [SETTINGS, AI, { href: '/admin', key: 'admin' }],
    }
  if (pathname === '/setup') return { current: 'setup', ancestors: [HOME] }
  return { current: 'notFound', ancestors: [] }
}

/** One structural hierarchy supplies both chrome and automatic entry eligibility. */
export function routeLocation(pathname: string, creationOrigin = false): RouteLocation {
  const path = navigationPathname(pathname)
  const canonicalPath = path === '/ai-models/compare' ? '/tests' : path
  const location = structuralLocation(canonicalPath, creationOrigin)
  const root = location.ancestors[0]?.href ?? canonicalPath
  const destination = DESTINATIONS.find((item) => item.to === root)?.to
  return { ...location, destination }
}
export function navigationPathname(href: string): string {
  return href.split(/[?#]/u, 1)[0].replace(/\/+$/u, '') || '/'
}
export function isTopLevelDestination(pathname: string): boolean {
  return DESTINATIONS.some((item) => item.to === navigationPathname(pathname))
}
export function isStructuralNavigationEntry(target: string, entry: string): boolean {
  const targetPath = navigationPathname(target)
  const entryPath = navigationPathname(entry)
  if (isTopLevelDestination(targetPath) || targetPath === entryPath) return false
  const location = routeLocation(targetPath)
  return (
    location.ancestors.some((ancestor) => ancestor.href.split('#', 1)[0] === entryPath) ||
    (location.current === 'test' && entryPath === '/tests/history')
  )
}
