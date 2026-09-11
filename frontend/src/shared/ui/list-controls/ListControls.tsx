import { useId } from 'react'
import { FieldLabel } from '../field/FieldLabel'
import { SegmentedControl, type SegmentedOption } from '../segmented-control/SegmentedControl'
import { TextField } from '../field/TextField'

/** A directory's search field and one filter, above its rows.
 *
 *  Both write straight to whatever the caller keeps them in — the URL, on both lists that use it
 *  (POST-67, CLIP-41) — so there is no local mirror to fall out of step with the address, and no
 *  debounce: the narrowing is a scan over one already-loaded answer, so a delay would only make
 *  the field lag the keystroke.
 *
 *  A primitive rather than a copy per list: the post directory and the clip directory reach for
 *  the same shape, and a second slice needing it is what §1.1 says makes it one (THEME-10). It
 *  knows no product noun — the caller supplies its own copy and its own filter values. */
export function ListControls<T extends string>({
  searchLabel,
  searchPlaceholder,
  query,
  onQueryChange,
  filter,
}: {
  searchLabel: string
  searchPlaceholder: string
  query: string
  onQueryChange: (value: string) => void
  filter: {
    value: T
    options: readonly SegmentedOption<T>[]
    onChange: (value: T) => void
    ariaLabel: string
  }
}) {
  const searchId = useId()
  return (
    <div className="grid gap-3">
      <div>
        <FieldLabel htmlFor={searchId}>{searchLabel}</FieldLabel>
        <TextField
          id={searchId}
          type="search"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder={searchPlaceholder}
          autoCapitalize="none"
          autoCorrect="off"
          enterKeyHint="search"
          className="mt-1"
        />
      </div>
      <SegmentedControl<T>
        value={filter.value}
        options={filter.options}
        onChange={filter.onChange}
        ariaLabel={filter.ariaLabel}
      />
    </div>
  )
}
