import { Bot, FileText, Film, Send } from 'lucide-react'

/** Shared by all three shell shapes. Group activity comes from an actual ancestor
 * match, never from a URL prefix (voices belongs to writing, not to /posts). */
export const DESTINATIONS = [
  {
    to: '/posts',
    labelKey: 'posts',
    icon: FileText,
    routeIds: ['/authenticated/writing'],
    masterOnly: false,
  },
  {
    to: '/clips',
    labelKey: 'videos',
    icon: Film,
    routeIds: ['/authenticated/video'],
    masterOnly: false,
  },
  {
    to: '/ai-models',
    labelKey: 'models',
    icon: Bot,
    routeIds: ['/authenticated/ai-models', '/authenticated/ai-models/experiments/$id'],
    masterOnly: false,
  },
  {
    to: '/publishing-agents',
    labelKey: 'publishingAgents',
    phoneLabelKey: 'publishingShort',
    icon: Send,
    routeIds: ['/authenticated/publishing-agents'],
    masterOnly: true,
  },
] as const

/** Text tabs deliberately retain the shared primitive's horizontal scrolling mode. */
export const CONTENT_GROUPS = {
  writing: [
    { to: '/posts', labelKey: 'posts' },
    { to: '/voices', labelKey: 'voices' },
    { to: '/templates', labelKey: 'templates' },
    { to: '/guidelines', labelKey: 'guidelines' },
  ],
  video: [
    { to: '/clips', labelKey: 'clips' },
    { to: '/video-templates', labelKey: 'videoTemplates' },
  ],
} as const

export function currentDestination(routeIds: readonly string[]) {
  return DESTINATIONS.find((destination) =>
    destination.routeIds.some((id) => routeIds.includes(id)),
  )?.to
}
