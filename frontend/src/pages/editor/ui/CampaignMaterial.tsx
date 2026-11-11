import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Pencil, X } from 'lucide-react'
import { AdvertiserGuideFields, type AdvertiserGuide } from '@/entities/advertiser-guide'
import type { AppFailure } from '@/shared/api'
import { AppFailureMessage, Button, FieldMessage, TextField, Typography } from '@/shared/ui'
import {
  acceptRequirement,
  editSuggestion,
  suggestRequirements,
  suggestionAlreadyAdded,
  type RequirementSuggestion,
} from '../model/requirements'

/** A post-specific expanded presentation. Clips keep the shared form's compact layout. */
export function CampaignMaterial({
  guide,
  refusal,
  readOnly,
  onChange,
}: {
  guide: AdvertiserGuide
  refusal: AppFailure | undefined
  readOnly: boolean
  onChange: (guide: AdvertiserGuide) => void
}) {
  const { t } = useTranslation('posts')
  const [dismissed, setDismissed] = useState<readonly string[]>([])
  const [editing, setEditing] = useState<RequirementSuggestion>()
  const [editText, setEditText] = useState('')
  const [error, setError] = useState('')
  const suggestions = suggestRequirements(guide.text).filter(
    (item) => !dismissed.includes(item.id) && !suggestionAlreadyAdded(guide, item),
  )
  const label = (item: RequirementSuggestion) =>
    t(`guide.suggestions.${item.kind}`, { value: item.value })
  const accept = (item: RequirementSuggestion) => {
    const next = acceptRequirement(guide, item)
    if (next === guide) {
      setError(
        t(
          suggestionAlreadyAdded(guide, item)
            ? 'advertiserGuide.repeated'
            : 'guide.suggestions.full',
        ),
      )
      return false
    }
    onChange(next)
    setError('')
    return true
  }
  return (
    <div className="mt-6">
      <Typography variant="body" as="p">
        {t('guide.requirementsHelp')}
      </Typography>
      <AdvertiserGuideFields
        value={guide}
        disabled={readOnly}
        expanded
        onChange={onChange}
        afterText={
          suggestions.length > 0 && (
            <section className="mt-5" aria-label={t('guide.suggestions.heading')}>
              <Typography variant="fieldTitle" as="h3">
                {t('guide.suggestions.heading')}
              </Typography>
              <Typography variant="meta" as="p" className="mt-1">
                {t('guide.suggestions.help')}
              </Typography>
              <ul className="mt-3 flex flex-wrap gap-2">
                {suggestions.map((item) => (
                  <li key={item.id} className="bg-field-bg flex max-w-full items-center rounded-lg">
                    <Button
                      variant="ghost"
                      disabled={readOnly}
                      onClick={() => accept(item)}
                      aria-label={t('guide.suggestions.add', { item: label(item) })}
                      className="min-h-11 min-w-0 text-left break-words whitespace-normal"
                    >
                      + {label(item)}
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      disabled={readOnly}
                      aria-label={t('guide.suggestions.edit', { item: label(item) })}
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
                      aria-label={t('guide.suggestions.dismiss', { item: label(item) })}
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
                    aria-label={t('guide.suggestions.editValue', { item: label(editing) })}
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
                      if (!changed) return setError(t('guide.suggestions.invalid'))
                      if (accept(changed)) {
                        setDismissed([...dismissed, editing.id])
                        setEditing(undefined)
                      }
                    }}
                  >
                    {t('guide.suggestions.save')}
                  </Button>
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setEditing(undefined)
                      setError('')
                    }}
                  >
                    {t('guide.suggestions.cancel')}
                  </Button>
                </div>
              )}
              {error && <FieldMessage className="mt-2">{error}</FieldMessage>}
            </section>
          )
        }
      />
      {refusal && (
        <div className="mt-2">
          <AppFailureMessage failure={refusal} />
        </div>
      )}
    </div>
  )
}
