import { useTranslation } from 'react-i18next'
import { CLIP_HISTORY_FILTERS, type ClipHistoryFilter } from '@/entities/clip-project'
import { ListControls } from '@/shared/ui'
import type { ClipNarrowing } from '../model/narrow'

const STATUSES = ['all', ...CLIP_HISTORY_FILTERS] as const

/** The clip directory's search field and status filter (CLIP-41), over the shared `ListControls`.
 *  Both write straight to the URL through the caller, and both are on the screen at every project
 *  count — a search that appears at some number of projects is a second layout for the same page. */
export function ClipListControls({
  narrowing,
  onChange,
}: {
  narrowing: ClipNarrowing
  onChange: (next: ClipNarrowing) => void
}) {
  const { t } = useTranslation('clips')
  return (
    <ListControls<ClipHistoryFilter | 'all'>
      searchLabel={t('project.search')}
      searchPlaceholder={t('project.searchPlaceholder')}
      query={narrowing.q ?? ''}
      onQueryChange={(q) => onChange({ ...narrowing, q })}
      filter={{
        value: narrowing.status ?? 'all',
        options: STATUSES.map((value) => ({
          value,
          label: value === 'all' ? t('project.filterAll') : t(`state.${value}`),
        })),
        onChange: (value) =>
          onChange({ ...narrowing, status: value === 'all' ? undefined : value }),
        label: t('project.filterLabel'),
      }}
    />
  )
}
