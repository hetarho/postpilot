import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { SpeechOperationPrice, SpeechPriceComponent } from '@/entities/model-catalog'
import { Button, Checkbox, FieldLabel, Listbox, TextField, Typography } from '@/shared/ui'
import { SPEECH_PRICE_COMPONENTS_MAX, SPEECH_UNITS } from '../config/speech'

export function SpeechPriceEditor({
  price,
  onChange,
}: {
  price: SpeechOperationPrice
  onChange: (p: SpeechOperationPrice) => void
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const patchCharge = (index: number, patch: Partial<SpeechPriceComponent>) =>
    onChange({
      ...price,
      charges: price.charges.map((c, i) => (i === index ? { ...c, ...patch } : c)),
    })
  return (
    <section className="space-y-3">
      <Typography variant="fieldTitle">
        {t(`speechAdmin.operation.${price.operation}`, { defaultValue: price.operation })}
      </Typography>
      {(['source', 'boundsSource', 'checkedAt'] as const).map((key) => (
        <div key={key}>
          <FieldLabel htmlFor={`${id}-${key}`}>{t(`speechAdmin.${key}`)}</FieldLabel>
          <TextField
            id={`${id}-${key}`}
            value={price[key]}
            onChange={(e) => onChange({ ...price, [key]: e.target.value })}
            type={key === 'checkedAt' ? 'text' : 'url'}
          />
        </div>
      ))}
      {price.charges.map((c, index) => (
        <div key={index} className="bg-surface-raised space-y-3 rounded-md p-4">
          <FieldLabel id={`${id}-${index}-unit-label`}>{t('speechAdmin.unit')}</FieldLabel>
          <Listbox
            aria-labelledby={`${id}-${index}-unit-label`}
            value={c.unit}
            onChange={(unit) => patchCharge(index, { unit })}
            options={[
              { value: '', label: t('speechAdmin.chooseUnit') },
              ...SPEECH_UNITS.map((unit) => ({
                value: unit,
                label: t(`speechAdmin.units.${unit}`),
              })),
            ]}
          />
          {(['usdPerUnit', 'multiplier', 'maximumUnits'] as const).map((key) => (
            <div key={key}>
              <FieldLabel htmlFor={`${id}-${index}-${key}`}>{t(`speechAdmin.${key}`)}</FieldLabel>
              <TextField
                id={`${id}-${index}-${key}`}
                value={c[key]}
                inputMode="decimal"
                onChange={(e) => patchCharge(index, { [key]: e.target.value })}
              />
            </div>
          ))}
          <Button
            variant="ghost"
            onClick={() =>
              onChange({ ...price, charges: price.charges.filter((_, i) => i !== index) })
            }
          >
            {t('speechAdmin.removeCharge')}
          </Button>
        </div>
      ))}
      <Button
        variant="secondary"
        disabled={price.charges.length >= SPEECH_PRICE_COMPONENTS_MAX}
        onClick={() =>
          onChange({
            ...price,
            charges: [
              ...price.charges,
              { unit: '', usdPerUnit: '', multiplier: '', maximumUnits: '' },
            ],
          })
        }
      >
        {t('speechAdmin.addCharge')}
      </Button>
      <label className="flex items-center gap-2">
        <Checkbox
          checked={price.complete}
          onChange={(e) => onChange({ ...price, complete: e.target.checked })}
        />
        {t('speechAdmin.complete')}
      </label>
    </section>
  )
}
