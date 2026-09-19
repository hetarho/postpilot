import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CLIP_TRANSITION } from '@/entities/clip-design'
import {
  CLIP_PLAYBACK_RATES,
  cutOutputMs,
  cutRate,
  type ClipEditCut,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { Button, FieldLabel, FieldMessage, Listbox, Typography } from '@/shared/ui'
import { ClipTimeField } from './ClipTimeField'

export function ClipCutAssemblyControls({
  cut,
  allowedRates,
  playheadMs,
  onChange,
  onSplit,
  native,
}: {
  cut: ClipEditCut
  allowedRates: readonly number[]
  playheadMs: number
  onChange: (edit: TimelineEdit) => void
  onSplit: (sourceMs: number) => Promise<void>
  native: boolean
}) {
  const { t } = useTranslation('clips')
  const [point, setPoint] = useState(() => Math.round((cut.startMs + cut.endMs) / 2))
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  const valid = (ms: number) =>
    Number.isSafeInteger(ms) &&
    ms > cut.startMs &&
    ms < cut.endMs &&
    cutOutputMs({ ...cut, endMs: ms }) > 2 * CLIP_TRANSITION.fade_ms &&
    cutOutputMs({ ...cut, startMs: ms }) > 2 * CLIP_TRANSITION.fade_ms
  const split = (ms: number) => {
    setPending(true)
    setFailed(false)
    void onSplit(ms)
      .catch(() => setFailed(true))
      .finally(() => setPending(false))
  }
  return (
    <fieldset disabled={!!cut.creation || pending} className="min-w-0 space-y-3">
      <div>
        <FieldLabel id={`clip-rate-${cut.id}`}>{t('assembly.rate')}</FieldLabel>
        <Listbox
          aria-labelledby={`clip-rate-${cut.id}`}
          value={cutRate(cut)}
          options={CLIP_PLAYBACK_RATES.map((rate) => ({
            value: rate,
            label: `${rate / 1000}×${allowedRates.includes(rate) ? '' : ` · ${t('assembly.cadenceRefused')}`}`,
            disabled: !allowedRates.includes(rate),
          }))}
          onChange={(ratePermille) => onChange({ type: 'rate', id: cut.id, ratePermille })}
        />
        {CLIP_PLAYBACK_RATES.some((rate) => rate < 1000 && !allowedRates.includes(rate)) && (
          <Typography variant="meta">{t('assembly.slowHelp')}</Typography>
        )}
      </div>
      {native && (
        <>
          <ClipTimeField
            id={`clip-split-${cut.id}`}
            label={t('assembly.splitTime')}
            value={point}
            onChange={setPoint}
            error={!valid(point) ? t('assembly.splitInvalid') : undefined}
          />
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" disabled={!valid(point)} onClick={() => split(point)}>
              {t('assembly.splitExact')}
            </Button>
            <Button
              variant="secondary"
              disabled={!valid(playheadMs)}
              onClick={() => split(playheadMs)}
            >
              {t('assembly.splitPlayhead')}
            </Button>
          </div>
        </>
      )}
      {(cut.creation || failed) && <FieldMessage>{t('assembly.saveFirst')}</FieldMessage>}
    </fieldset>
  )
}
