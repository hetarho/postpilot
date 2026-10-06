import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { LEVELS, type AdminSpeechProfile, type SpeechRegistration } from '@/entities/model-catalog'
import {
  Button,
  Checkbox,
  Disclosure,
  FieldLabel,
  Listbox,
  TextField,
  Typography,
} from '@/shared/ui'
import { friendlySpeechModel } from '../lib/speech-pricing'
import { SpeechCostSummary } from './SpeechCostSummary'
import { SPEECH_ADJUSTMENT_MAX, SPEECH_ADJUSTMENT_MIN } from '../config/speech'

export function SpeechCombinationRow({
  profile: p,
  styleSupported,
  designName: suppliedDesignName,
  speechName: suppliedSpeechName,
  metadataAvailable,
  saving,
  reason,
  onSave,
  onQualify,
}: {
  profile: AdminSpeechProfile
  styleSupported: boolean
  designName?: string
  speechName?: string
  metadataAvailable: boolean
  saving: boolean
  reason?: string
  onSave: (registration: SpeechRegistration) => Promise<unknown>
  onQualify: (profile: AdminSpeechProfile) => void
}) {
  const { t } = useTranslation('models')
  const id = useId()
  const [settings, setSettings] = useState(p.binding.settings)
  const registered = p.id !== ''
  const parts = p.label.includes('→') ? p.label.split('→').map((part) => part.trim()) : []
  const designName =
    friendlySpeechModel(p.binding.designModel.modelId, suppliedDesignName ?? parts[0]) ??
    t('speechAdmin.designModel')
  const speechName =
    friendlySpeechModel(p.binding.speechModel.modelId, suppliedSpeechName ?? parts[1]) ??
    t('speechAdmin.speechModel')
  const title = p.label.includes('→')
    ? t('speechAdmin.combinationTitle', { design: designName, speech: speechName })
    : p.label
  const registration: SpeechRegistration = {
    profileId: p.id,
    expectedRevision: p.revision,
    designModel: p.binding.designModel,
    speechModel: p.binding.speechModel,
    enabled: p.enabled,
    grade: p.grade,
  }
  const save = (patch: Partial<SpeechRegistration>) =>
    void onSave({ ...registration, ...patch }).catch(() => {})
  const submitSettings = (event: FormEvent) => {
    event.preventDefault()
    save({
      adjustments: {
        stability: settings.stability,
        similarity: settings.similarityBoost,
        style: styleSupported ? settings.style : 0,
      },
    })
  }
  const priceReady = p.prices.length === 3 && p.prices.every((price) => price.complete)
  return (
    <li className="space-y-3 py-4">
      <div className="flex items-start justify-between gap-3">
        <Typography variant="fieldTitle">{title}</Typography>
        <label className="flex min-h-11 shrink-0 items-center gap-2">
          <Checkbox
            checked={p.enabled}
            disabled={saving || (!registered && !metadataAvailable)}
            aria-label={t('speechAdmin.useLabel', { name: title })}
            onChange={(event) => save({ enabled: event.target.checked })}
          />
          {t('catalog.use')}
        </label>
      </div>
      <div className="space-y-2">
        <Typography variant="body">{t('speechAdmin.designRole', { model: designName })}</Typography>
        <Typography variant="meta">{t('speechAdmin.designRoleHelp')}</Typography>
        <Typography variant="body">{t('speechAdmin.speechRole', { model: speechName })}</Typography>
        <Typography variant="meta">{t('speechAdmin.speechRoleHelp')}</Typography>
      </div>
      <SpeechCostSummary profile={p} />
      <Disclosure title={t('speechAdmin.modelDetails')} size="row" headingLevel={3}>
        <Typography variant="meta" mono className="break-all">
          {t('speechAdmin.designModel')}: {p.binding.designModel.modelId}
        </Typography>
        <Typography variant="meta" mono className="break-all">
          {t('speechAdmin.speechModel')}: {p.binding.speechModel.modelId}
        </Typography>
      </Disclosure>
      {registered && (
        <>
          <FieldLabel id={`${id}-grade-label`}>{t('speechAdmin.grade')}</FieldLabel>
          <Listbox
            aria-labelledby={`${id}-grade-label`}
            value={p.grade}
            disabled={saving}
            options={[
              { value: '' as const, label: t('catalog.levelUnset') },
              ...LEVELS.map((value) => ({ value, label: t(`level.${value}`) })),
            ]}
            onChange={(grade) => save({ grade })}
          />
          {!p.grade && (
            <Typography variant="meta" role="status">
              {t('catalog.levelMissing')}
            </Typography>
          )}
          <Typography variant="meta">
            {t('speech.limits', {
              description: p.binding.descriptionMax,
              preview: p.binding.previewMax,
              speech: p.binding.speechMax,
            })}
          </Typography>
          <Typography variant="meta">
            {t(p.voiceReady ? 'speechAdmin.voiceReady' : 'speechAdmin.voicePending')} ·{' '}
            {t(p.exportReady ? 'speechAdmin.exportReady' : 'speechAdmin.exportPending')}
          </Typography>
          {!priceReady && (
            <Typography variant="meta" role="status">
              {t('speechAdmin.pricePending')}
            </Typography>
          )}
          {reason && (
            <Typography variant="meta">
              {t(`speech.reason.${reason}`, { defaultValue: t('speech.unavailable') })}
            </Typography>
          )}
          <Disclosure title={t('speechAdmin.settings')} size="row" headingLevel={3}>
            <form onSubmit={submitSettings} className="max-w-measure space-y-3 pt-2">
              <Typography variant="meta">{t('speechAdmin.settingsHelp')}</Typography>
              {(
                [
                  'stability',
                  'similarityBoost',
                  ...(styleSupported ? (['style'] as const) : []),
                ] as const
              ).map((key) => (
                <div key={key}>
                  <FieldLabel htmlFor={`${id}-${key}`}>{t(`speechAdmin.${key}`)}</FieldLabel>
                  <TextField
                    id={`${id}-${key}`}
                    type="number"
                    min={SPEECH_ADJUSTMENT_MIN}
                    max={SPEECH_ADJUSTMENT_MAX}
                    step="any"
                    required
                    value={settings[key]}
                    disabled={saving || !metadataAvailable}
                    onChange={(event) =>
                      setSettings({ ...settings, [key]: event.target.valueAsNumber })
                    }
                  />
                </div>
              ))}
              <Button type="submit" pending={saving} disabled={!metadataAvailable}>
                {t('speechAdmin.saveSettings')}
              </Button>
            </form>
          </Disclosure>
          <Button
            variant="ghost"
            disabled={saving || !p.enabled || !p.grade || !priceReady || !metadataAvailable}
            onClick={() => onQualify(p)}
          >
            {t('speechAdmin.qualify')}
          </Button>
        </>
      )}
    </li>
  )
}
