import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { FieldLabel, Listbox, Typography } from '@/shared/ui'
import type { SpeechProfileChoice } from '../model/speech'

export function SpeechProfilePicker({
  profiles,
  selectedId,
  onSelect,
}: {
  profiles: readonly SpeechProfileChoice[]
  selectedId: string | null
  onSelect: (profile: SpeechProfileChoice | null) => void
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const selected = profiles.find((p) => p.id === selectedId)
  return (
    <div>
      <FieldLabel id={`${id}-label`} htmlFor={id}>
        {t('speech.modelLabel')}
      </FieldLabel>
      <Listbox
        id={id}
        aria-labelledby={`${id}-label`}
        value={selectedId ?? ''}
        onChange={(value) => onSelect(profiles.find((p) => p.id === value) ?? null)}
        options={[
          { value: '', label: t('speech.choose') },
          ...profiles.map((p) => ({
            value: p.id,
            disabled: !p.available,
            label: `${p.label} · ${p.designLabel} → ${p.speechLabel} · ${p.grade ? t(`level.${p.grade}`) : t('catalog.levelUnset')}${p.available ? '' : ` · ${!p.entitled ? t('speech.planRequired', { plan: p.requiredPlan }) : t(`speech.reason.${p.unavailableReason}`, { defaultValue: t('speech.unavailable') })}`}`,
          })),
        ]}
      />
      {profiles.length === 0 && <Typography variant="meta">{t('speech.empty')}</Typography>}
      {selected && (
        <Typography variant="meta" role="status">
          {selected.available
            ? t('speech.limits', {
                description: selected.descriptionMax,
                preview: selected.previewMax,
                speech: selected.speechMax,
              })
            : t(`speech.reason.${selected.unavailableReason}`, {
                defaultValue: t('speech.unavailable'),
                plan: selected.requiredPlan,
              })}
        </Typography>
      )}
    </div>
  )
}
