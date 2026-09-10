import {
  Bot,
  Clapperboard,
  FileText,
  Film,
  LayoutTemplate,
  ListChecks,
  Scissors,
  Send,
  Speech,
} from 'lucide-react'

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

/** The second level is drawn with the first level's row shape (THEME-38), so a group destination
 *  carries an icon exactly like a primary one. `글` repeats the primary icon deliberately: it is
 *  the same destination seen one level down, not a different place. */
export const CONTENT_GROUPS = {
  writing: [
    { to: '/posts', labelKey: 'posts', icon: FileText },
    { to: '/voices', labelKey: 'voices', icon: Speech },
    { to: '/templates', labelKey: 'templates', icon: LayoutTemplate },
    { to: '/guidelines', labelKey: 'guidelines', icon: ListChecks },
  ],
  video: [
    { to: '/clips', labelKey: 'clips', icon: Scissors },
    { to: '/video-templates', labelKey: 'videoTemplates', icon: Clapperboard },
  ],
} as const

export function currentDestination(routeIds: readonly string[]) {
  return DESTINATIONS.find((destination) =>
    destination.routeIds.some((id) => routeIds.includes(id)),
  )?.to
}
