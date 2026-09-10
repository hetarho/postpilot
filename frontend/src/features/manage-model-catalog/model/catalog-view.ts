import {
  LEVELS,
  REASONING_EFFORTS,
  type AdminCatalogEntry,
  type ReasoningEffortName,
} from '@/entities/model-catalog'
import { FEATURED_MODEL_PROVIDERS, type ModelPurpose } from '@/shared/config'

/** What the operator has narrowed the catalog to. Every field is a widening default, so the
 *  initial state shows everything the active tab may show. */
export interface CatalogFilters {
  search: string
  /** '' means every vendor. */
  providerSlug: string
  visionOnly: boolean
  structuredOnly: boolean
  registeredOnly: boolean
}

export const NO_FILTERS: CatalogFilters = {
  search: '',
  providerSlug: '',
  visionOnly: false,
  structuredOnly: false,
  registeredOnly: false,
}

/** The purposes whose tab forces a capability gate — the single list the gate, the
 *  requirement note, and any future per-purpose copy key off, so they cannot drift apart. */
export const GATED_PURPOSES = ['photo-analysis', 'image-generation', 'video-generation'] as const

export type GatedPurpose = (typeof GATED_PURPOSES)[number]

export function isGatedPurpose(purpose: ModelPurpose): purpose is GatedPurpose {
  return (GATED_PURPOSES as readonly string[]).includes(purpose)
}

/** The capability gate a purpose tab forces on its candidate list, mirroring the server's
 *  registration gate: an entry the gate refuses is not even listed, so the tab never offers
 *  a checkbox the server would refuse. The style/writing tabs take any text model. */
export function eligibleForPurpose(entry: AdminCatalogEntry, purpose: ModelPurpose): boolean {
  switch (purpose) {
    case 'photo-analysis':
      return entry.vision
    case 'image-generation':
      return entry.imageOutput
    case 'video-generation':
      return entry.videoOutput
    default:
      return true
  }
}

/** What a purpose tab may show at all: everything its gate admits, PLUS anything already
 *  registered to it. A registered model that later LOST the gated capability (a refresh
 *  re-snapshots the flags) must stay visible — this tab's checkbox is the only control that
 *  can deregister it, and hiding it would make the stale registration permanent. */
export function visibleInTab(entry: AdminCatalogEntry, purpose: ModelPurpose): boolean {
  return eligibleForPurpose(entry, purpose) || entry.purposes.includes(purpose)
}

/** Vendors present in this catalog, featured ones first. Derived from the entries rather than
 *  from the config list so a vendor nobody anticipated is still reachable from the filter. */
export function providerSlugs(entries: readonly AdminCatalogEntry[]): string[] {
  const slugs = [...new Set(entries.map((entry) => entry.providerSlug))]
  return slugs.sort(compareProviders)
}

/** How the operator has ordered the list (MODEL-28). `default` is the provider/newest order
 *  below; the price orders key on the output price, because output tokens are what the app's
 *  spend is made of, so one key is enough. */
export type CatalogSort = 'default' | 'level' | 'price-asc' | 'price-desc'

export const CATALOG_SORTS: readonly CatalogSort[] = ['default', 'level', 'price-asc', 'price-desc']

export const DEFAULT_SORT: CatalogSort = 'default'

/** `default`: featured vendors first in their configured order, then everyone else
 *  alphabetically, and inside a vendor the newest model first — a catalog is read to find
 *  what is new. The price orders sort by output price per million, tie-break on input price,
 *  then fall back to the default order; a model with no published price (video, 토큰 단가
 *  미공개) comes last in BOTH directions — "unknown" is neither the cheapest nor the dearest.
 *
 *  `level`: the operator's own grade for THIS tab, 가성비 first, with everything ungraded —
 *  including the models not registered to this purpose at all — after it. The tab is about
 *  this purpose's registrations, so an entry with no registration here has no grade here
 *  either; that tail is the operator's backlog.
 *
 *  Sorting here rather than on the server is deliberate: the whole catalog arrives in one
 *  response, and how to read it is a display preference the browser owns. The price strings
 *  are decimal; `Number` is fine for ORDERING them (no arithmetic, nothing is displayed from
 *  the converted value). */
export function sortEntries(
  entries: readonly AdminCatalogEntry[],
  sort: CatalogSort = DEFAULT_SORT,
): AdminCatalogEntry[] {
  const byDefault = [...entries].sort((a, b) => {
    const byProvider = compareProviders(a.providerSlug, b.providerSlug)
    if (byProvider !== 0) return byProvider
    if (a.sourceCreatedAt !== b.sourceCreatedAt)
      return a.sourceCreatedAt > b.sourceCreatedAt ? -1 : 1
    return a.modelId < b.modelId ? -1 : a.modelId > b.modelId ? 1 : 0
  })
  if (sort === 'default') return byDefault
  if (sort === 'level') {
    // Stable, so the default order survives inside one grade.
    return [...byDefault].sort((a, b) => levelRank(a) - levelRank(b))
  }

  const direction = sort === 'price-asc' ? 1 : -1
  const priced = byDefault.filter((entry) => entry.outputUsdPerMillion !== '')
  const unpriced = byDefault.filter((entry) => entry.outputUsdPerMillion === '')
  // `sort` is stable, so equal prices keep the default order they arrived in.
  priced.sort((a, b) => {
    const byOutput = Number(a.outputUsdPerMillion) - Number(b.outputUsdPerMillion)
    if (byOutput !== 0) return byOutput * direction
    const byInput = Number(a.inputUsdPerMillion) - Number(b.inputUsdPerMillion)
    return Number.isNaN(byInput) ? 0 : byInput * direction
  })
  return [...priced, ...unpriced]
}

/** The operator's own narrowing, applied over a tab's `visibleInTab` slice — the forced
 *  gate has already run, so nothing an operator types can resurface a model the purpose
 *  cannot take. */
export function filterEntries(
  entries: readonly AdminCatalogEntry[],
  filters: CatalogFilters,
  purpose: ModelPurpose,
): AdminCatalogEntry[] {
  const needle = filters.search.trim().toLowerCase()
  return entries.filter((entry) => {
    if (filters.providerSlug && entry.providerSlug !== filters.providerSlug) return false
    if (filters.visionOnly && !entry.vision) return false
    if (filters.structuredOnly && !entry.structuredOutput) return false
    if (filters.registeredOnly && !entry.purposes.includes(purpose)) return false
    if (!needle) return true
    // Both the id and the label, because an operator arrives with either: a model id copied
    // from a provider's page, or the name they read in a comparison.
    return (
      entry.modelId.toLowerCase().includes(needle) || entry.label.toLowerCase().includes(needle)
    )
  })
}

function levelRank(entry: AdminCatalogEntry): number {
  // '' is unset AND is what an entry not registered to this tab carries: both sort last.
  return entry.level === '' ? LEVELS.length : LEVELS.indexOf(entry.level)
}

function compareProviders(a: string, b: string): number {
  const rankA = featuredRank(a)
  const rankB = featuredRank(b)
  if (rankA !== rankB) return rankA - rankB
  return a < b ? -1 : a > b ? 1 : 0
}

function featuredRank(slug: string): number {
  const index = FEATURED_MODEL_PROVIDERS.indexOf(slug)
  return index === -1 ? FEATURED_MODEL_PROVIDERS.length : index
}

/** The effort values the control offers for one model (change 27).
 *
 *  The model's own published list when it has one, and the full eight otherwise — a source
 *  that publishes no list is not a model that refuses every value, so the fallback is the
 *  honest render. Order is the source's descending effort order, kept as published.
 *
 *  `''` (defer to the stage policy) and `unset` are always offered: neither is a claim about
 *  what the model accepts. `none` IS such a claim, so it is withheld when reasoning is
 *  mandatory and when a published list simply does not contain it — the same rule
 *  `SetModelReasoning` enforces server-side.
 *
 *  A drifted override is added back so the control can still SHOW the value it is warning
 *  about; a Listbox whose value is not among its options renders as empty, which would hide
 *  exactly the thing the operator needs to see. */
export function reasoningOptionsFor(entry: AdminCatalogEntry): ReasoningEffortName[] {
  const published = entry.reasoning.efforts
  const base: ReasoningEffortName[] =
    published.length > 0 ? ['', 'unset', ...published] : [...REASONING_EFFORTS]
  const offered = base.filter((effort) => effort !== 'none' || allowsNone(entry))
  if (entry.reasoningEffort !== '' && !offered.includes(entry.reasoningEffort)) {
    offered.push(entry.reasoningEffort)
  }
  return offered
}

function allowsNone(entry: AdminCatalogEntry): boolean {
  if (entry.reasoning.mandatory) return false
  const published = entry.reasoning.efforts
  return published.length === 0 || published.includes('none')
}

/** Whether this model gets an effort control at all. A model the source says does not reason
 *  has no effort to choose, so the control is absent rather than offered and ignored.
 *
 *  Withheld only on a POSITIVE answer — `known` and `reasons: false`. An entry served from
 *  storage says `known: false`, which covers both a row written before this data existed and
 *  every row on a screen whose provider fetch failed; hiding the control there would take away
 *  an override that is still being sent on every call. */
export function offersReasoningControl(entry: AdminCatalogEntry): boolean {
  return !entry.reasoning.known || entry.reasoning.reasons
}
