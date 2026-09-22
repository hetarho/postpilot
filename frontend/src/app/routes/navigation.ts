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
 * match, never from a URL prefix (voices belongs to writing, not to /posts).
 *
 * `group` is what makes ONE sidebar possible (owner decision 2026-09-22): the shell reads the
 * group a primary destination opens straight off this list, instead of a second layout telling it
 * from further down the tree. */
export const DESTINATIONS = [
  {
    to: '/posts',
    group: 'writing',
    labelKey: 'posts',
    icon: FileText,
    routeIds: ['/authenticated/writing'],
    masterOnly: false,
  },
  {
    to: '/clips',
    group: 'video',
    labelKey: 'videos',
    icon: Film,
    routeIds: ['/authenticated/video'],
    masterOnly: false,
  },
  {
    to: '/ai-models',
    group: 'models',
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

export type ContentGroup = keyof typeof CONTENT_GROUPS

/** The group the shell is inside, if any: the primary destination's own, so `/voices` is 글's
 *  group without sharing its address. */
export function currentGroup(routeIds: readonly string[]): ContentGroup | undefined {
  return DESTINATIONS.find((destination) =>
    destination.routeIds.some((id) => routeIds.includes(id)),
  )?.group
}

/** Which destination of a group the address is under. A PREFIX match, so `/voices/one/rules` is
 *  still 말투, longest first so `/ai-models/compare` wins over `/ai-models`; the group's home
 *  otherwise. `/ai-models/experiments/$id?from=compare` is the one address that belongs to a
 *  sibling rather than to its own prefix: it was reached from 모델 비교 and returns there. */
export function currentGroupDestination(
  group: ContentGroup,
  pathname: string,
  from?: string,
): (typeof CONTENT_GROUPS)[ContentGroup][number] {
  const destinations: readonly { to: string }[] = CONTENT_GROUPS[group]
  const reached =
    group === 'models' && from === 'compare' && pathname.startsWith('/ai-models/experiments/')
      ? destinations.find((d) => d.to === '/ai-models/compare')
      : undefined
  const under = destinations
    .filter((d) => pathname === d.to || pathname.startsWith(`${d.to}/`))
    .sort((a, b) => b.to.length - a.to.length)[0]
  return (reached ?? under ?? destinations[0]!) as (typeof CONTENT_GROUPS)[ContentGroup][number]
}
