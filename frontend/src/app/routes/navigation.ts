import { House, Archive, Settings2 } from 'lucide-react'

export const DESTINATIONS = [
  { to: '/', labelKey: 'launch', icon: House },
  { to: '/library', labelKey: 'library', icon: Archive },
  { to: '/settings', labelKey: 'settings', icon: Settings2 },
] as const

export function currentDestination(pathname: string) {
  if (pathname === '/' || pathname === '/posts/new' || pathname === '/clips/new') return '/'
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
