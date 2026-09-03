import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useAdminCatalog, useRefreshCatalog } from '@/entities/model-catalog'
import { Button, Checkbox, FieldLabel, Listbox, Notice, TextField, Typography } from '@/shared/ui'
import {
  NO_FILTERS,
  delistedCount,
  filterEntries,
  providerSlugs,
  sortEntries,
  type CatalogFilters,
} from '../model/catalog-view'
import { CatalogModelList } from './CatalogModelList'

/** The operator's model curation surface: browse what the provider offers, narrow it, and check
 *  the models this installation will let its accounts use.
 *
 *  Every narrowing happens in the browser over the one response. The catalog is a few hundred
 *  rows that arrive together, so a round trip per keystroke would buy latency and nothing else. */
export function ModelCatalogManager() {
  const { t } = useTranslation('models')
  const { catalog, isPending, isError } = useAdminCatalog()
  const refresh = useRefreshCatalog()
  const [filters, setFilters] = useState<CatalogFilters>(NO_FILTERS)
  const controlsId = useId()
  const searchId = `${controlsId}-search`
  const providerId = `${controlsId}-provider`
  const providerLabelId = `${providerId}-label`

  const sorted = useMemo(() => sortEntries(catalog.entries), [catalog.entries])
  const visible = useMemo(() => filterEntries(sorted, filters), [sorted, filters])
  const vendors = useMemo(() => providerSlugs(catalog.entries), [catalog.entries])
  const delisted = useMemo(() => delistedCount(catalog.entries), [catalog.entries])

  const patch = (next: Partial<CatalogFilters>) =>
    setFilters((current) => ({ ...current, ...next }))

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

      <div className="mt-6 grid gap-3">
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
        <div className="flex flex-wrap gap-x-6 gap-y-3">
          {(
            [
              ['visionOnly', t('catalog.filterVision')],
              ['structuredOnly', t('catalog.filterStructured')],
              ['enabledOnly', t('catalog.filterEnabled')],
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
      {delisted > 0 && (
        <Notice tone="warning" role="status" className="mt-4">
          {t('catalog.delistedBanner', { count: delisted })}
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
          {t(catalog.entries.length === 0 ? 'catalog.empty' : 'catalog.noMatches')}
        </Typography>
      )}
      {visible.length > 0 && (
        <>
          <Typography variant="meta" className="mt-6 block">
            {t('catalog.count', { shown: visible.length, total: catalog.entries.length })}
          </Typography>
          <CatalogModelList entries={visible} />
        </>
      )}
    </section>
  )
}
