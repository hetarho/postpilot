import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Pencil, X } from 'lucide-react'
import { Button, FieldMessage, TextField, Typography } from '@/shared/ui'
import {
  acceptRequirement,
  editSuggestion,
  suggestRequirements,
  suggestionAlreadyAdded,
  type RequirementSuggestion,
} from '../lib/scan'
import type { AdvertiserGuide, AdvertiserGuideItem } from '../model/types'

/** Chips for what the guide's free text seems to ask, under POST-118's add, edit and dismiss
 *  rules. Nothing becomes a counted item until the owner adds it, and a dismissal lasts only while
 *  this stays mounted. */
export function AdvertiserGuideSuggestions({
  guide,
  items,
  readOnly,
  onChange,
}: {
  guide: AdvertiserGuide
  items?: readonly AdvertiserGuideItem[]
  readOnly: boolean
  onChange: (guide: AdvertiserGuide) => void
}) {
  const { t } = useTranslation('posts')
  const [dismissed, setDismissed] = useState<readonly string[]>([])
  const [editing, setEditing] = useState<RequirementSuggestion>()
  const [editText, setEditText] = useState('')
  const [error, setError] = useState('')
  const suggestions = suggestRequirements(guide.text, items).filter(
    (item) => !dismissed.includes(item.id) && !suggestionAlreadyAdded(guide, item),
  )
  const label = (item: RequirementSuggestion) =>
    t(`advertiserGuide.suggestions.${item.kind}`, { value: item.value })
  const accept = (item: RequirementSuggestion) => {
    const next = acceptRequirement(guide, item)
    if (next === guide) {
      setError(
        t(
          suggestionAlreadyAdded(guide, item)
            ? 'advertiserGuide.repeated'
            : 'advertiserGuide.suggestions.full',
        ),
      )
      return false
    }
    onChange(next)
    setError('')
    return true
  }
  if (!suggestions.length) return null
  return (
    <section className="mt-5" aria-label={t('advertiserGuide.suggestions.heading')}>
      <Typography variant="fieldTitle" as="h3">
        {t('advertiserGuide.suggestions.heading')}
      </Typography>
      <Typography variant="meta" as="p" className="mt-1">
        {t('advertiserGuide.suggestions.help')}
      </Typography>
      <ul className="mt-3 flex flex-wrap gap-2">
        {suggestions.map((item) => (
          <li key={item.id} className="bg-field-bg flex max-w-full items-center rounded-lg">
            <Button
              variant="ghost"
              disabled={readOnly}
              onClick={() => accept(item)}
              aria-label={t('advertiserGuide.suggestions.add', { item: label(item) })}
              className="min-h-11 min-w-0 text-left break-words whitespace-normal"
            >
              + {label(item)}
            </Button>
            <Button
              variant="ghost"
              size="icon"
              disabled={readOnly}
              aria-label={t('advertiserGuide.suggestions.edit', { item: label(item) })}
              onClick={() => {
                setEditing(item)
                setEditText(String(item.value))
                setError('')
              }}
            >
              <Pencil className="size-4" aria-hidden />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              disabled={readOnly}
              aria-label={t('advertiserGuide.suggestions.dismiss', { item: label(item) })}
              onClick={() => {
                setDismissed([...dismissed, item.id])
                if (editing?.id === item.id) setEditing(undefined)
              }}
            >
              <X className="size-4" aria-hidden />
            </Button>
          </li>
        ))}
      </ul>
      {editing && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <TextField
            aria-label={t('advertiserGuide.suggestions.editValue', { item: label(editing) })}
            value={editText}
            disabled={readOnly}
            onChange={(event) => setEditText(event.target.value)}
            inputMode={typeof editing.value === 'number' ? 'numeric' : 'text'}
            className="min-w-0 flex-1"
          />
          <Button
            variant="secondary"
            disabled={readOnly}
            onClick={() => {
              const changed = editSuggestion(editing.kind, editText)
              if (!changed) return setError(t('advertiserGuide.suggestions.invalid'))
              if (accept(changed)) {
                setDismissed([...dismissed, editing.id])
                setEditing(undefined)
              }
            }}
          >
            {t('advertiserGuide.suggestions.save')}
          </Button>
          <Button
            variant="ghost"
            onClick={() => {
              setEditing(undefined)
              setError('')
            }}
          >
            {t('advertiserGuide.suggestions.cancel')}
          </Button>
        </div>
      )}
      {error && <FieldMessage className="mt-2">{error}</FieldMessage>}
    </section>
  )
}
