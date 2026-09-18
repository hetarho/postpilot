import { useId } from 'react'
import { FieldLabel } from '../field/FieldLabel'
import { Listbox, type ListboxOption } from '../listbox/Listbox'
import { TextField } from '../field/TextField'

/** A directory's search field and one filter, above its rows, on ONE row.
 *
 *  Both write straight to whatever the caller keeps them in — the URL, on both lists that use it
 *  (POST-67, CLIP-41) — so there is no local mirror to fall out of step with the address, and no
 *  debounce: the narrowing is a scan over one already-loaded answer, so a delay would only make
 *  the field lag the keystroke.
 *
 *  The filter is a labelled `Listbox`, not a `SegmentedControl`: a row of pills under the search
 *  read as a row of buttons rather than as a filter, and took a row of its own on a phone (owner
 *  decision 2026-09-19). A select-shaped field beside the search says "narrow by this" and shows
 *  the one value that is narrowing.
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
    options: readonly ListboxOption<T>[]
    onChange: (value: T) => void
    /** The filter's visible name (상태): the field's own label, so the control reads as "narrow by
     *  this" and its accessible name is the label followed by the value that is narrowing. */
    label: string
  }
}) {
  const searchId = useId()
  const filterId = useId()
  return (
    <div className="flex items-end gap-2">
      <div className="min-w-0 flex-1">
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
      <div className="shrink-0">
        <FieldLabel id={filterId}>{filter.label}</FieldLabel>
        <Listbox<T>
          aria-labelledby={filterId}
          value={filter.value}
          options={filter.options}
          onChange={filter.onChange}
          className="mt-1 w-32"
        />
      </div>
    </div>
  )
}
