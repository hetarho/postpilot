import { useTranslation } from 'react-i18next'
import type { PostStatus } from '@/entities/post'
import { ListControls } from '@/shared/ui'
import type { PostNarrowing } from '../model/narrow'

const STATUSES: readonly (PostStatus | 'all')[] = ['all', 'draft', 'review', 'finalized']

/** The post list's search field and status filter (POST-65, POST-66), over the shared
 *  `ListControls`. Both write straight to the URL through the caller, which is the single source
 *  for what they show (POST-67).
 *
 *  They are on the screen at every post count (POST-68): a search that appears at some number of
 *  posts is a second layout for the same page. */
export function PostListControls({
  narrowing,
  onChange,
}: {
  narrowing: PostNarrowing
  onChange: (next: PostNarrowing) => void
}) {
  const { t } = useTranslation('posts')
  return (
    <ListControls<PostStatus | 'all'>
      searchLabel={t('list.search')}
      searchPlaceholder={t('list.searchPlaceholder')}
      query={narrowing.q ?? ''}
      onQueryChange={(q) => onChange({ ...narrowing, q })}
      filter={{
        value: narrowing.status ?? 'all',
        options: STATUSES.map((value) => ({ value, label: t(`list.filter.${value}`) })),
        onChange: (value) =>
          onChange({ ...narrowing, status: value === 'all' ? undefined : value }),
        ariaLabel: t('list.filter.aria'),
      }}
    />
  )
}
