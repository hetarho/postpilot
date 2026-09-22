import {
  Bot,
  SlidersHorizontal,
  GitCompareArrows,
  History,
  Trophy,
  Brain,
  Clapperboard,
  FileText,
  Film,
  LayoutTemplate,
  ListChecks,
  Scissors,
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
    routeIds: ['/authenticated/models'],
    masterOnly: false,
  },
] as const

/** The second level is drawn with the first level's row shape (THEME-38), so a group destination
 *  carries an icon exactly like a primary one. The group's HOME repeats the primary icon
 *  deliberately — it is the same destination seen one level down, not a different place — and is
 *  named for what it lists (내 글 · 내 영상), which is also the name the phone's group row shows by
 *  default (owner decision 2026-09-19). */
export const CONTENT_GROUP_LABELS = {
  writing: 'writingGroup',
  video: 'videoGroup',
  models: 'modelsGroup',
} as const

export const CONTENT_GROUPS = {
  writing: [
    { to: '/posts', labelKey: 'myPosts', icon: FileText },
    { to: '/voices', labelKey: 'voices', icon: Speech },
    { to: '/templates', labelKey: 'templates', icon: LayoutTemplate },
    { to: '/guidelines', labelKey: 'guidelines', icon: ListChecks },
    { to: '/memories', labelKey: 'memories', icon: Brain },
  ],
  models: [
    { to: '/ai-models', labelKey: 'modelSettings', icon: SlidersHorizontal },
    { to: '/ai-models/compare', labelKey: 'modelComparison', icon: GitCompareArrows },
    { to: '/ai-models/experiments', labelKey: 'modelHistory', icon: History },
    { to: '/ai-models/leaderboard', labelKey: 'modelLeaderboard', icon: Trophy },
  ],
  video: [
    { to: '/clips', labelKey: 'myVideos', icon: Scissors },
    { to: '/video-templates', labelKey: 'videoTemplates', icon: Clapperboard },
  ],
} as const

export function currentDestination(routeIds: readonly string[]) {
  return DESTINATIONS.find((destination) =>
    destination.routeIds.some((id) => routeIds.includes(id)),
  )?.to
}
