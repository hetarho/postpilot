import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  type ClipCompositionInputs,
  type ClipSourceAssociation,
  type ClipObservations,
  type TimelineEdit,
} from '@/entities/clip-project'
import { Checkbox, FieldLabel, Listbox, Typography } from '@/shared/ui'

export function ClipAssociationControls({
  inputs,
  associations,
  observations,
  sourceId,
  change,
}: {
  inputs: ClipCompositionInputs
  associations: ClipSourceAssociation[]
  observations?: ClipObservations
  sourceId: string
  change: (edit: TimelineEdit) => void
}) {
  const { t } = useTranslation('clips')
  const items = Object.entries(inputs.items).flatMap(([groupId, items]) =>
    items.map((item) => ({ groupId, ...item })),
  )
  const [chosen, setChosen] = useState('')
  const selected = items.find((item) => `${item.groupId}/${item.id}` === chosen) ?? items[0]
  const source = observations?.sources.find((s) => s.source.id === sourceId)
  if (!items.length) return null
  return (
    <section className="space-y-3" aria-label={t('timeline.associations')}>
      <Typography variant="fieldTitle">{t('timeline.associations')}</Typography>
      <Typography variant="body">{t('timeline.associationHelp')}</Typography>
      <FieldLabel id="clip-association-item-label">{t('timeline.item')}</FieldLabel>
      <Listbox
        aria-labelledby="clip-association-item-label"
        value={selected ? `${selected.groupId}/${selected.id}` : ''}
        onChange={setChosen}
        options={items.map((item) => ({
          value: `${item.groupId}/${item.id}`,
          label: Object.values(item.values).filter(Boolean).join(' · ') || item.id,
        }))}
      />
      {source?.segments.map((range, index) => {
        const association: ClipSourceAssociation = {
          groupId: selected!.groupId,
          itemId: selected!.id,
          sourceId: source.source.id,
          fingerprint: source.source.fingerprint,
          startMs: range.startMs,
          endMs: range.endMs,
        }
        const matches = (a: ClipSourceAssociation) =>
          Object.entries(association).every(
            ([key, value]) => a[key as keyof ClipSourceAssociation] === value,
          )
        return (
          <label key={index} className="flex min-h-11 items-start gap-3 py-2">
            <Checkbox
              checked={associations.some(matches)}
              onChange={(event) =>
                change({
                  type: 'associations',
                  associations: event.target.checked
                    ? [...associations, association]
                    : associations.filter((a) => !matches(a)),
                })
              }
            />
            <span className="min-w-0">
              <Typography variant="body">
                {t('timeline.sourceRange', {
                  start: clipSeconds(range.startMs),
                  end: clipSeconds(range.endMs),
                })}{' '}
                · {range.event}
              </Typography>
            </span>
          </label>
        )
      })}
      {!source?.segments.length && <Typography variant="meta">{t('timeline.noRanges')}</Typography>}
    </section>
  )
}
