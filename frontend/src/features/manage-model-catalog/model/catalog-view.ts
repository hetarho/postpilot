import type { AdminCatalogEntry } from '@/entities/model-catalog'
import { FEATURED_MODEL_PROVIDERS } from '@/shared/config'

/** What the operator has narrowed the catalog to. Every field is a widening default, so the
 *  initial state shows everything. */
export interface CatalogFilters {
  search: string
  /** '' means every vendor. */
  providerSlug: string
  visionOnly: boolean
  structuredOnly: boolean
  enabledOnly: boolean
}

export const NO_FILTERS: CatalogFilters = {
  search: '',
  providerSlug: '',
  visionOnly: false,
  structuredOnly: false,
  enabledOnly: false,
}

/** Vendors present in this catalog, featured ones first. Derived from the entries rather than
 *  from the config list so a vendor nobody anticipated is still reachable from the filter. */
export function providerSlugs(entries: readonly AdminCatalogEntry[]): string[] {
  const slugs = [...new Set(entries.map((entry) => entry.providerSlug))]
  return slugs.sort(compareProviders)
}

/** Featured vendors first in their configured order, then everyone else alphabetically, and
 *  inside a vendor the newest model first — a catalog is read to find what is new.
 *
 *  Sorting here rather than on the server is deliberate: the whole catalog arrives in one
 *  response, and which vendors are worth lifting is a display preference the browser owns. */
export function sortEntries(entries: readonly AdminCatalogEntry[]): AdminCatalogEntry[] {
  return [...entries].sort((a, b) => {
    const byProvider = compareProviders(a.providerSlug, b.providerSlug)
    if (byProvider !== 0) return byProvider
    if (a.sourceCreatedAt !== b.sourceCreatedAt)
      return a.sourceCreatedAt > b.sourceCreatedAt ? -1 : 1
    return a.modelId < b.modelId ? -1 : a.modelId > b.modelId ? 1 : 0
  })
}

export function filterEntries(
  entries: readonly AdminCatalogEntry[],
  filters: CatalogFilters,
): AdminCatalogEntry[] {
  const needle = filters.search.trim().toLowerCase()
  return entries.filter((entry) => {
    if (filters.providerSlug && entry.providerSlug !== filters.providerSlug) return false
    if (filters.visionOnly && !entry.vision) return false
    if (filters.structuredOnly && !entry.structuredOutput) return false
    if (filters.enabledOnly && !entry.enabled) return false
    if (!needle) return true
    // Both the id and the label, because an operator arrives with either: a model id copied
    // from a provider's page, or the name they read in a comparison.
    return (
      entry.modelId.toLowerCase().includes(needle) || entry.label.toLowerCase().includes(needle)
    )
  })
}

/** Curated models the provider has stopped offering. They stay selectable-looking to nobody —
 *  users see them disabled — but they need an operator to retire them, so the screen counts
 *  them rather than acting on its own. */
export function delistedCount(entries: readonly AdminCatalogEntry[]): number {
  return entries.filter((entry) => entry.curated && !entry.listed).length
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
