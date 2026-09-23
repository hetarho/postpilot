import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  BLOG_FIELD_IDS,
  blogFieldLabelKey,
  type BlogFieldId,
} from '@/entities/blog-field/@x/guideline'
import { Checkbox, FieldLabel } from '@/shared/ui'

/** The product's 분야 as a checkbox list, in catalogue order — the template list's shape, since a
 *  multiple-choice native select is close to unusable on a phone. It emits the next set in
 *  catalogue order whatever order the boxes were pressed in, so the wire never carries a press
 *  order. The scope control and the preset row's 적용할 분야 both use it. */
export function GuidelineFieldPicker({
  value,
  onChange,
  legend,
  legendVisible = false,
  disabled = false,
  className,
}: {
  value: readonly BlogFieldId[]
  onChange: (next: BlogFieldId[]) => void
  legend: string
  /** Hidden by default: the scope control's own help line already names the list. */
  legendVisible?: boolean
  disabled?: boolean
  className?: string
}) {
  const { t } = useTranslation('posts')
  const id = useId()

  const toggle = (field: BlogFieldId) => {
    const checked = value.includes(field)
    onChange(
      BLOG_FIELD_IDS.filter((candidate) =>
        candidate === field ? !checked : value.includes(candidate),
      ),
    )
  }

  return (
    <fieldset className={className} disabled={disabled}>
      <legend className={legendVisible ? undefined : 'sr-only'}>{legend}</legend>
      <ul className="space-y-1">
        {BLOG_FIELD_IDS.map((field) => (
          <li key={field} className="flex items-center gap-3">
            <Checkbox
              id={`${id}-${field}`}
              checked={value.includes(field)}
              onChange={() => toggle(field)}
            />
            <FieldLabel htmlFor={`${id}-${field}`}>{t(blogFieldLabelKey(field))}</FieldLabel>
          </li>
        ))}
      </ul>
    </fieldset>
  )
}
