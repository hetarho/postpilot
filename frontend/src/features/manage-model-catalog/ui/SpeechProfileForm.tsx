import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import type {
  AdminSpeechProfile,
  SpeechCandidate,
  SpeechOperationPrice,
} from '@/entities/model-catalog'
import { LEVELS, refKey } from '@/entities/model-catalog'
import { Button, Checkbox, FieldLabel, Listbox, TextField, Typography } from '@/shared/ui'
import {
  SPEECH_BINDING_DEFAULTS,
  SPEECH_OPERATIONS,
  SPEECH_PROFILE_LABEL_MAX,
} from '../config/speech'
import { SpeechPriceEditor } from './SpeechPriceEditor'

function newProfile(): AdminSpeechProfile {
  return {
    id: '',
    revision: 0n,
    label: '',
    grade: '',
    enabled: true,
    voiceReady: false,
    exportReady: false,
    prices: [],
    binding: {
      ...SPEECH_BINDING_DEFAULTS,
      settings: { ...SPEECH_BINDING_DEFAULTS.settings },
      designModel: { providerId: '', modelId: '' },
      speechModel: { providerId: '', modelId: '' },
    },
  }
}

export function SpeechProfileForm({
  profile,
  candidates,
  saving,
  onSave,
  onCancel,
}: {
  profile: AdminSpeechProfile | null
  candidates: readonly SpeechCandidate[]
  saving: boolean
  onSave: (p: AdminSpeechProfile) => Promise<void>
  onCancel: () => void
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const [draft, setDraft] = useState<AdminSpeechProfile>(profile ?? newProfile)
  const [prices, setPrices] = useState<SpeechOperationPrice[]>(() =>
    SPEECH_OPERATIONS.map(
      (operation) =>
        profile?.prices.find((p) => p.operation === operation) ?? {
          operation,
          charges: [],
          source: '',
          boundsSource: '',
          checkedAt: '',
          complete: false,
        },
    ),
  )
  const bind = (patch: Partial<AdminSpeechProfile['binding']>) =>
    setDraft((p) => ({ ...p, binding: { ...p.binding, ...patch } }))
  const submit = async (e: FormEvent) => {
    e.preventDefault()
    await onSave({ ...draft, prices: prices.filter((p) => p.charges.length > 0 || p.complete) })
  }
  const complete =
    draft.label.trim() !== '' &&
    draft.grade !== '' &&
    draft.binding.designModel.modelId !== '' &&
    draft.binding.speechModel.modelId !== ''
  return (
    <form onSubmit={(e) => void submit(e)} className="max-w-measure mt-6 space-y-5">
      <Typography variant="fieldTitle">
        {t(profile ? 'speechAdmin.edit' : 'speechAdmin.new')}
      </Typography>
      <FieldLabel htmlFor={`${id}-name`}>{t('speechAdmin.name')}</FieldLabel>
      <TextField
        id={`${id}-name`}
        value={draft.label}
        maxLength={SPEECH_PROFILE_LABEL_MAX}
        onChange={(e) => setDraft({ ...draft, label: e.target.value })}
        required
      />
      {(['designModel', 'speechModel'] as const).map((key) => {
        const models = candidates.filter((m) => (key === 'designModel' ? m.design : m.synthesis))
        const current = draft.binding[key]
        return (
          <div key={key}>
            <FieldLabel id={`${id}-${key}-label`}>{t(`speechAdmin.${key}`)}</FieldLabel>
            <Listbox
              aria-labelledby={`${id}-${key}-label`}
              value={current.modelId ? refKey(current) : ''}
              options={[
                { value: '', label: t('speechAdmin.chooseModel') },
                ...models.map((m) => ({
                  value: refKey(m.ref),
                  label: m.label,
                  disabled:
                    key === 'speechModel' &&
                    (!m.korean ||
                      m.requiresAlpha ||
                      m.maxText <= 0 ||
                      (draft.binding.designModel.providerId !== '' &&
                        draft.binding.designModel.providerId !== m.ref.providerId)),
                })),
                ...(current.modelId && !models.some((m) => refKey(m.ref) === refKey(current))
                  ? [{ value: refKey(current), label: current.modelId, disabled: true }]
                  : []),
              ]}
              onChange={(value) =>
                bind({
                  [key]: models.find((m) => refKey(m.ref) === value)?.ref ?? {
                    providerId: '',
                    modelId: '',
                  },
                })
              }
            />
          </div>
        )
      })}
      <Typography variant="meta">{t('speechAdmin.koreanOnly')}</Typography>
      <FieldLabel id={`${id}-grade-label`}>{t('speechAdmin.grade')}</FieldLabel>
      <Listbox
        aria-labelledby={`${id}-grade-label`}
        value={draft.grade}
        onChange={(grade) => setDraft({ ...draft, grade })}
        options={[
          { value: '' as const, label: t('speechAdmin.chooseGrade') },
          ...LEVELS.map((value) => ({ value, label: t(`level.${value}`) })),
        ]}
      />
      <label className="flex items-center gap-2">
        <Checkbox
          checked={draft.enabled}
          onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })}
        />
        {t('speechAdmin.enabled')}
      </label>
      <Typography variant="fieldTitle">{t('speechAdmin.settings')}</Typography>
      {(['stability', 'similarityBoost', 'style'] as const).map((key) => (
        <div key={key}>
          <FieldLabel htmlFor={`${id}-${key}`}>{t(`speechAdmin.${key}`)}</FieldLabel>
          <TextField
            type="number"
            min={0}
            max={1}
            step="any"
            required
            id={`${id}-${key}`}
            value={draft.binding.settings[key]}
            onChange={(e) =>
              bind({ settings: { ...draft.binding.settings, [key]: e.target.valueAsNumber } })
            }
          />
        </div>
      ))}
      <label className="flex items-center gap-2">
        <Checkbox
          checked={draft.binding.settings.speakerBoost}
          onChange={(e) =>
            bind({ settings: { ...draft.binding.settings, speakerBoost: e.target.checked } })
          }
        />
        {t('speechAdmin.speakerBoost')}
      </label>
      {(['descriptionMax', 'previewMax', 'speechMax'] as const).map((key) => (
        <div key={key}>
          <FieldLabel htmlFor={`${id}-${key}`}>{t(`speechAdmin.${key}`)}</FieldLabel>
          <TextField
            type="number"
            min={1}
            required
            id={`${id}-${key}`}
            value={draft.binding[key]}
            onChange={(e) => bind({ [key]: e.target.valueAsNumber })}
          />
        </div>
      ))}
      <Typography variant="fieldTitle">{t('speechAdmin.pricing')}</Typography>
      <Typography variant="body">{t('speechAdmin.pricingHelp')}</Typography>
      {prices.map((price, index) => (
        <SpeechPriceEditor
          key={price.operation}
          price={price}
          onChange={(next) => setPrices(prices.map((p, i) => (i === index ? next : p)))}
        />
      ))}
      <div className="flex flex-wrap gap-3">
        <Button type="submit" pending={saving} disabled={!complete}>
          {t('speechAdmin.save')}
        </Button>
        <Button variant="ghost" onClick={onCancel} disabled={saving}>
          {t('speechAdmin.cancel')}
        </Button>
      </div>
    </form>
  )
}
