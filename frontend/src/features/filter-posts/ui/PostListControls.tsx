import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { PostStatus } from '@/entities/post'
import { FieldLabel, SegmentedControl, TextField } from '@/shared/ui'
import type { PostNarrowing } from '../model/narrow'

const STATUSES: readonly (PostStatus | 'all')[] = ['all', 'draft', 'review', 'finalized']

/** The list's search field and status filter (POST-65, POST-66). Both write straight to the URL
 *  through the caller, which is the single source for what they show (POST-67) — there is no
 *  local mirror to fall out of step with the address, and no debounce: the narrowing is a scan
 *  over one already-loaded answer, so a delay would only make the field lag the keystroke.
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
  const searchId = useId()

  return (
    <div className="grid gap-3">
      <div>
        <FieldLabel htmlFor={searchId}>{t('list.search')}</FieldLabel>
        <TextField
          id={searchId}
          type="search"
          value={narrowing.q ?? ''}
          onChange={(event) => onChange({ ...narrowing, q: event.target.value })}
          placeholder={t('list.searchPlaceholder')}
          autoCapitalize="none"
          autoCorrect="off"
          enterKeyHint="search"
          className="mt-1"
        />
      </div>
      <SegmentedControl<PostStatus | 'all'>
        value={narrowing.status ?? 'all'}
        options={STATUSES.map((value) => ({ value, label: t(`list.filter.${value}`) }))}
        onChange={(value) =>
          onChange({ ...narrowing, status: value === 'all' ? undefined : value })
        }
        ariaLabel={t('list.filter.aria')}
      />
    </div>
  )
}
