import { createClient, type Transport } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { GuidelineService } from '@/shared/api'
import { activeLocale } from '@/shared/lib'
import type {
  DefaultGuideline,
  DefaultGuidelineEntry,
  Guideline,
  GuidelineKind,
} from '../model/types'
import {
  fromGuidelineKind,
  guidelineKindQueryKey,
  localizeDefaultGuideline,
  toDefaultGuidelineEntry,
  toGuideline,
} from './guideline-queries'

/** What one kind's list read holds: the owner's guidelines of the kind, and its 기본 지침 with this
 *  account's switches (GUIDE-19). */
export interface GuidelineListData {
  guidelines: Guideline[]
  defaults: DefaultGuidelineEntry[]
}

/** The one query behind the list. A read and nothing else: mounting it creates no guideline,
 *  calls no model and starts no job ([I5]).
 *
 *  `staleTime: 0` + `refetchOnMount: 'always'` against the app's 60s default, like the purpose
 *  directory, because each scoped guideline's purpose names are a PROJECTION: renaming a purpose
 *  changes what this list shows without touching any guideline row. */
export function guidelineListQuery(
  transport: Transport,
  ownerId: string,
  kind: GuidelineKind = 'post',
) {
  return {
    queryKey: guidelineKindQueryKey(transport, ownerId, kind),
    // Mapped here, not in the hook: a scope this build cannot read throws, and inside the query it
    // is a read failure the page shows with its retry rather than a crash.
    queryFn: (): Promise<GuidelineListData> =>
      createClient(GuidelineService, transport)
        .listGuidelines({ kind: fromGuidelineKind(kind) })
        .then((response) => ({
          guidelines: response.guidelines.map(toGuideline),
          defaults: response.defaults.map(toDefaultGuidelineEntry),
        })),
    staleTime: 0,
    refetchOnMount: 'always' as const,
  }
}

const NO_GUIDELINES: Guideline[] = []
const NO_DEFAULTS: DefaultGuidelineEntry[] = []

export function useGuidelines(
  ownerId: string,
  kind: GuidelineKind = 'post',
): {
  guidelines: Guideline[]
  /** The 기본 지침 in registry order, in the UI language. */
  defaults: DefaultGuideline[]
  isPending: boolean
  isError: boolean
  isFetching: boolean
  refetch: () => void
} {
  const transport = useTransport()
  // Subscribed so a language switch re-renders the defaults in the other copy.
  useTranslation()
  const query = useQuery({
    ...guidelineListQuery(transport, ownerId, kind),
    enabled: ownerId !== '',
  })
  const locale = activeLocale()
  // The server returns them in injection order; the client never reorders them, so the screen
  // shows exactly the order the writer will be given.
  return {
    guidelines: query.data?.guidelines ?? NO_GUIDELINES,
    defaults: (query.data?.defaults ?? NO_DEFAULTS).map((entry) =>
      localizeDefaultGuideline(entry, locale),
    ),
    isPending: query.isPending,
    isError: query.isError,
    isFetching: query.isFetching,
    refetch: () => void query.refetch(),
  }
}
