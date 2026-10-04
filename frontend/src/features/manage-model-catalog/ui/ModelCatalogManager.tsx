import { useId, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { LEVELS, useAdminCatalog, useRefreshCatalog } from '@/entities/model-catalog'
import { MODEL_PURPOSES } from '@/entities/model-catalog'
import { type ModelPurpose } from '@/entities/model-catalog'
import {
  Button,
  Checkbox,
  FieldLabel,
  Listbox,
  Notice,
  SegmentedControl,
  TextField,
  Typography,
} from '@/shared/ui'
import {
  CATALOG_SORTS,
  DEFAULT_SORT,
  NO_FILTERS,
  filterEntries,
  isGatedPurpose,
  providerSlugs,
  sortEntries,
  visibleInTab,
  type CatalogFilters,
  type CatalogSort,
} from '../model/catalog-view'
import { CatalogModelList } from './CatalogModelList'
import { CatalogDocumentPanel } from './CatalogDocumentPanel'
import { SpeechCatalogManager } from './SpeechCatalogManager'

/** The operator's model curation surface: browse what the provider offers, narrow it, and check
 *  the models this installation will let its accounts use — PER PURPOSE (MODEL-13). Each tab
 *  registers models for one purpose only, and force-filters its candidates to what that
 *  purpose's capability gate would accept, so the checkbox never offers what the server
 *  refuses.
 *
 *  Every narrowing happens in the browser over the one response. The catalog is a few hundred
 *  rows that arrive together, so a round trip per keystroke would buy latency and nothing else. */
/** Why a tab shows nothing: an empty catalog, an empty forced gate, or the operator's own
 *  narrowing. A2 wants an explanation rather than an error in every one of the three. */
function emptyMessageKey(catalogCount: number, tabCount: number) {
  if (catalogCount === 0) return 'catalog.empty' as const
  if (tabCount > 0) return 'catalog.noMatches' as const
  return 'catalog.emptyForPurpose' as const
}

/** The five purposes, then 추천 조합 (MODEL-28). */
type CatalogTab = ModelPurpose | 'recommendations' | 'speech'

export function ModelCatalogManager({
  recommendations,
}: {
  /** The 추천 조합 tab's content (MODEL-69). It is a slot rather than an import because one
   *  feature may not import another (ARCH-13): the page composes the two. */
  recommendations?: ReactNode
}) {
  const { t } = useTranslation('models')
  const [tab, setTab] = useState<CatalogTab>(MODEL_PURPOSES[0])
  // The purpose is declared before the reads that depend on it: the listing is per purpose,
  // because the effort and the spend signal each row shows belong to the tab being looked at.
  // It keeps the last purpose while 추천 조합 is open, so coming back is not a new read.
  const [purpose, setPurpose] = useState<ModelPurpose>(MODEL_PURPOSES[0])
  const onRecommendations = tab === 'recommendations'
  const chooseTab = (next: CatalogTab) => {
    setTab(next)
    if (next !== 'recommendations' && next !== 'speech') setPurpose(next)
  }
  const { catalog, isPending, isError } = useAdminCatalog(purpose)
  const refresh = useRefreshCatalog(purpose)
  const [filters, setFilters] = useState<CatalogFilters>(NO_FILTERS)
  // Sort is not a filter: filters narrow, sort orders. It lives beside them and, like them,
  // survives a tab switch.
  const [sort, setSort] = useState<CatalogSort>(DEFAULT_SORT)
  // One 일괄 편집 entry for all six tabs, not one per tab: a pasted document names any purpose
  // and the recommendation sets, so a per-tab control would misstate what a paste changes.
  const [documentOpen, setDocumentOpen] = useState(false)
  // Bumped on every open so the panel remounts with empty state instead of an effect
  // clearing the previous session's paste and diff.
  const [documentSession, setDocumentSession] = useState(0)
  const controlsId = useId()
  const searchId = `${controlsId}-search`
  const providerId = `${controlsId}-provider`
  const providerLabelId = `${providerId}-label`
  const sortId = `${controlsId}-sort`
  const sortLabelId = `${sortId}-label`
  const levelId = `${controlsId}-level`
  const levelLabelId = `${levelId}-label`
  const panelId = `${controlsId}-purpose-panel`

  const sorted = useMemo(() => sortEntries(catalog.entries), [catalog.entries])
  // The tab's forced gate runs once per (catalog, purpose); the operator's filters then
  // narrow only this slice, so a keystroke never re-evaluates the gate.
  const tabEntries = useMemo(
    () => sorted.filter((entry) => visibleInTab(entry, purpose)),
    [sorted, purpose],
  )
  // The operator's order runs last, over the filtered slice — so choosing a sort never
  // re-runs the gate or the filters, and every row the list virtualizes is already in place.
  const visible = useMemo(
    () => sortEntries(filterEntries(tabEntries, filters, purpose), sort),
    [tabEntries, filters, purpose, sort],
  )
  const vendors = useMemo(() => providerSlugs(catalog.entries), [catalog.entries])

  const patch = (next: Partial<CatalogFilters>) =>
    setFilters((current) => ({ ...current, ...next }))

  const openDocument = () => {
    setDocumentSession((session) => session + 1)
    setDocumentOpen(true)
  }

  return (
    <section className="mt-8">
      <Typography variant="title">{t('catalog.title')}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('catalog.description')}
      </Typography>
      {/* Stated once, before anything is pressed, rather than repeated on every row: unchecking a
          model clears it out of the selections of everyone who had chosen it. */}
      <Typography variant="body" className="text-content-tertiary max-w-measure mt-1">
        {t('catalog.disableWarning')}
      </Typography>

      <SegmentedControl<CatalogTab>
        value={tab}
        options={[
          ...MODEL_PURPOSES.map((value) => ({
            value,
            label: t(`catalog.purposeTab.${value}`),
          })),
          ...(recommendations
            ? [
                {
                  value: 'recommendations' as const,
                  label: t('catalog.purposeTab.recommendations'),
                },
              ]
            : []),
          { value: 'speech', label: t('speechAdmin.tab') },
        ]}
        onChange={chooseTab}
        ariaLabel={t('catalog.purposeAria')}
        controls={panelId}
        className="mt-6"
      />

      {tab === 'speech' ? (
        <div id={panelId} role="tabpanel">
          <SpeechCatalogManager />
        </div>
      ) : onRecommendations ? (
        // The sets are not a purpose: no capability gate, no search or filters, no provider
        // refresh. What they share with the purposes is the one 일괄 편집 document.
        <div id={panelId} role="tabpanel">
          <div className="mt-6">
            <Button variant="secondary" onClick={openDocument}>
              {t('document.title')}
            </Button>
          </div>
          {recommendations}
        </div>
      ) : (
        <>
          {isGatedPurpose(purpose) && (
            <Typography variant="body" className="text-content-tertiary max-w-measure mt-2">
              {t(`catalog.purposeRequirement.${purpose}`)}
            </Typography>
          )}

          <div id={panelId} role="tabpanel" className="mt-6 grid gap-3">
            <div>
              <FieldLabel htmlFor={searchId}>{t('catalog.search')}</FieldLabel>
              <TextField
                id={searchId}
                type="search"
                value={filters.search}
                onChange={(e) => patch({ search: e.target.value })}
                placeholder={t('catalog.searchPlaceholder')}
                autoCapitalize="none"
                autoCorrect="off"
                enterKeyHint="search"
                className="mt-1"
              />
            </div>
            <div className="min-w-0">
              <FieldLabel id={providerLabelId} htmlFor={providerId}>
                {t('catalog.provider')}
              </FieldLabel>
              <Listbox<string>
                id={providerId}
                value={filters.providerSlug}
                options={[
                  { value: '', label: t('catalog.allProviders') },
                  ...vendors.map((slug) => ({ value: slug, label: slug })),
                ]}
                aria-labelledby={providerLabelId}
                onChange={(providerSlug) => patch({ providerSlug })}
                className="mt-1"
              />
            </div>
            <div className="min-w-0">
              <FieldLabel id={levelLabelId} htmlFor={levelId}>
                {t('catalog.filterLevel')}
              </FieldLabel>
              <Listbox<CatalogFilters['level']>
                id={levelId}
                value={filters.level}
                options={[
                  { value: '', label: t('catalog.allLevels') },
                  ...LEVELS.map((level) => ({ value: level, label: t(`level.${level}`) })),
                  { value: 'unset', label: t('catalog.levelUnset') },
                ]}
                aria-labelledby={levelLabelId}
                onChange={(level) => patch({ level })}
                className="mt-1"
              />
            </div>
            <div className="min-w-0">
              <FieldLabel id={sortLabelId} htmlFor={sortId}>
                {t('catalog.sort')}
              </FieldLabel>
              <Listbox<CatalogSort>
                id={sortId}
                value={sort}
                options={CATALOG_SORTS.map((value) => ({
                  value,
                  label: t(`catalog.sortOption.${value}`),
                }))}
                aria-labelledby={sortLabelId}
                onChange={setSort}
                className="mt-1"
              />
            </div>
            <div className="flex flex-wrap gap-x-6 gap-y-3">
              {(
                [
                  ['visionOnly', t('catalog.filterVision')],
                  ['structuredOnly', t('catalog.filterStructured')],
                  ['registeredOnly', t('catalog.filterEnabled')],
                ] as const
              ).map(([key, label]) => (
                <label key={key} className="flex items-center gap-2">
                  <Checkbox
                    checked={filters[key]}
                    onChange={(e) => patch({ [key]: e.target.checked })}
                  />
                  <Typography variant="label">{label}</Typography>
                </label>
              ))}
            </div>
          </div>

          <div className="mt-6 flex flex-wrap items-center gap-3">
            <Button variant="secondary" onClick={refresh.refresh} pending={refresh.isPending}>
              {t('catalog.refresh')}
            </Button>
            <Button variant="secondary" onClick={openDocument}>
              {t('document.title')}
            </Button>
            <Typography variant="meta" role="status" className="min-w-0 break-words">
              {catalog.fetchedAt
                ? t(catalog.fromCache ? 'catalog.fetchedCached' : 'catalog.fetchedLive', {
                    at: catalog.fetchedAt,
                  })
                : null}
            </Typography>
          </div>

          {catalog.fetchError !== '' && (
            <Notice tone="warning" role="status" className="mt-4">
              {t('catalog.fetchFailed')}
            </Notice>
          )}
          {isError && (
            <Notice tone="danger" role="alert" className="mt-4">
              {t('catalog.loadFailed')}
            </Notice>
          )}

          {!isError && isPending && (
            <Typography variant="body" role="status" className="text-content-tertiary mt-6">
              {t('catalog.loading')}
            </Typography>
          )}
          {!isError && !isPending && visible.length === 0 && (
            <Typography variant="body" className="text-content-tertiary mt-6">
              {t(emptyMessageKey(catalog.entries.length, tabEntries.length))}
            </Typography>
          )}

          {visible.length > 0 && (
            <>
              <Typography variant="meta" className="mt-6 block">
                {t('catalog.count', { shown: visible.length, total: tabEntries.length })}
              </Typography>
              <CatalogModelList entries={visible} purpose={purpose} />
            </>
          )}
        </>
      )}

      <CatalogDocumentPanel
        key={documentSession}
        open={documentOpen}
        onClose={() => setDocumentOpen(false)}
      />
    </section>
  )
}
