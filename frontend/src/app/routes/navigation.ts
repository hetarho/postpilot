import { House, Archive, Settings2, FlaskConical } from 'lucide-react'
import type { navigationI18n } from './navigation-i18n'

export const DESTINATIONS = [
  { to: '/', labelKey: 'launch', icon: House },
  { to: '/tests', labelKey: 'tests', icon: FlaskConical },
  { to: '/settings', labelKey: 'settings', icon: Settings2 },
  { to: '/library', labelKey: 'history', icon: Archive, secondary: true },
] as const
export function currentDestination(pathname: string, creationOrigin = false) {
  if (
    pathname === '/' ||
    pathname === '/posts/new' ||
    pathname === '/clips/new' ||
    (creationOrigin && /^\/(posts|clips)\/[^/]+$/.test(pathname))
  )
    return '/'
  if (
    pathname === '/tests' ||
    pathname.startsWith('/tests/') ||
    pathname.startsWith('/ai-models/experiments') ||
    pathname === '/ai-models/compare' ||
    pathname === '/ai-models/leaderboard' ||
    pathname.startsWith('/posts/experiments/')
  )
    return '/tests'
  if (
    pathname === '/library' ||
    pathname === '/posts' ||
    pathname.startsWith('/posts/') ||
    pathname === '/clips' ||
    pathname.startsWith('/clips/')
  )
    return '/library'
  return '/settings'
}
export type LocationKey = keyof typeof navigationI18n.ko.location
export interface LocationAncestor {
  href: string
  key: LocationKey
}
export interface RouteLocation {
  current: LocationKey
  ancestors: LocationAncestor[]
}
const HOME: LocationAncestor = { href: '/', key: 'creation' }
const SETTINGS: LocationAncestor = { href: '/settings', key: 'settings' }
const WRITING: LocationAncestor = { href: '/settings#settings-writing', key: 'writing' }
const VIDEO: LocationAncestor = { href: '/settings#settings-video', key: 'video' }
const AI: LocationAncestor = { href: '/settings#settings-ai', key: 'ai' }
const HISTORY: LocationAncestor = { href: '/library', key: 'history' }
const TESTS: LocationAncestor = { href: '/tests', key: 'tests' }
export function routeLocation(pathname: string, creationOrigin = false): RouteLocation {
  if (pathname === '/') return { current: 'creation', ancestors: [] }
  if (pathname === '/posts/new' || (creationOrigin && /^\/posts\/[^/]+$/.test(pathname)))
    return { current: 'post', ancestors: [HOME] }
  if (pathname === '/clips/new' || (creationOrigin && /^\/clips\/[^/]+$/.test(pathname)))
    return { current: 'clip', ancestors: [HOME] }
  if (pathname === '/library') return { current: 'history', ancestors: [HOME] }
  if (pathname === '/posts') return { current: 'posts', ancestors: [HISTORY] }
  if (pathname === '/clips') return { current: 'clips', ancestors: [HISTORY] }
  if (pathname.startsWith('/posts/experiments/') || pathname.startsWith('/ai-models/experiments/'))
    return { current: 'test', ancestors: [TESTS, { href: '/tests/history', key: 'testHistory' }] }
  if (pathname.startsWith('/posts/'))
    return { current: 'post', ancestors: [HISTORY, { href: '/posts', key: 'posts' }] }
  if (pathname.startsWith('/clips/'))
    return { current: 'clip', ancestors: [HISTORY, { href: '/clips', key: 'clips' }] }
  if (pathname === '/tests') return { current: 'tests', ancestors: [HOME] }
  if (pathname === '/tests/history') return { current: 'testHistory', ancestors: [TESTS] }
  if (pathname.startsWith('/tests/'))
    return { current: 'test', ancestors: [TESTS, { href: '/tests/history', key: 'testHistory' }] }
  if (pathname === '/settings') return { current: 'settings', ancestors: [HOME] }
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
  if (pathname.startsWith('/ai-models')) return { current: 'models', ancestors: [SETTINGS, AI] }
  if (pathname.startsWith('/account')) return { current: 'account', ancestors: [SETTINGS, AI] }
  if (pathname.startsWith('/billing')) return { current: 'billing', ancestors: [SETTINGS, AI] }
  if (pathname === '/plans') return { current: 'plans', ancestors: [SETTINGS, AI] }
  if (pathname.startsWith('/admin')) return { current: 'admin', ancestors: [SETTINGS, AI] }
  if (pathname === '/setup') return { current: 'setup', ancestors: [HOME] }
  if (pathname.startsWith('/gift/')) return { current: 'gift', ancestors: [SETTINGS, AI] }
  return { current: 'settings', ancestors: [HOME] }
}
