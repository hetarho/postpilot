import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import type { SpeechAccountTariff } from '@/entities/model-catalog'
import { Button, Checkbox, FieldLabel, TextField, Typography } from '@/shared/ui'
import { scaleSpeechDecimal, speechTariffDraft } from '../lib/speech-pricing'

export function SpeechTariffEditor({
  tariff,
  saving,
  unavailable,
  onSave,
}: {
  tariff: SpeechAccountTariff | null
  saving: boolean
  unavailable: boolean
  onSave: (tariff: SpeechAccountTariff) => Promise<unknown>
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const [draft, setDraft] = useState<SpeechAccountTariff>(() => {
    const values = speechTariffDraft(tariff)
    return {
      ...values,
      designUsdPerUnit: scaleSpeechDecimal(values.designUsdPerUnit, 3),
      speechUsdPerUnit: scaleSpeechDecimal(values.speechUsdPerUnit, 3),
    }
  })
  const submit = (event: FormEvent) => {
    event.preventDefault()
    void onSave({
      ...draft,
      designUsdPerUnit: scaleSpeechDecimal(draft.designUsdPerUnit, -3),
      speechUsdPerUnit: scaleSpeechDecimal(draft.speechUsdPerUnit, -3),
    }).catch(() => {})
  }
  return (
    <form onSubmit={submit} className="max-w-measure space-y-3 pt-2">
      <Typography variant="body">{t('speechAdmin.tariffHelp')}</Typography>
      {(['designUsdPerUnit', 'speechUsdPerUnit', 'confirmationUsd', 'source'] as const).map(
        (key) => (
          <div key={key}>
            <FieldLabel htmlFor={`${id}-${key}`}>{t(`speechAdmin.${key}`)}</FieldLabel>
            <TextField
              id={`${id}-${key}`}
              type={key === 'source' ? 'url' : 'text'}
              inputMode={key === 'source' ? undefined : 'decimal'}
              value={draft[key]}
              required
              disabled={saving || unavailable}
              onChange={(event) =>
                setDraft({ ...draft, [key]: event.target.value, complete: false })
              }
            />
          </div>
        ),
      )}
      <label className="flex min-h-11 items-center gap-2">
        <Checkbox
          checked={draft.complete}
          disabled={saving || unavailable}
          onChange={(event) => setDraft({ ...draft, complete: event.target.checked })}
        />
        {t('speechAdmin.tariffComplete')}
      </label>
      <Button
        type="submit"
        pending={saving}
        disabled={
          unavailable ||
          !draft.complete ||
          !draft.designUsdPerUnit ||
          !draft.speechUsdPerUnit ||
          !draft.confirmationUsd ||
          !draft.source
        }
      >
        {t('speechAdmin.saveTariff')}
      </Button>
      {tariff?.checkedAt && (
        <Typography variant="meta">
          {t('speechAdmin.tariffChecked', { at: tariff.checkedAt })}
        </Typography>
      )}
    </form>
  )
}
