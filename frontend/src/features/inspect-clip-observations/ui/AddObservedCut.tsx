import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { CLIP_TRANSITION } from '@/shared/config'
import {
  clipSeconds,
  type ClipAddCutSelection,
  type ClipEditPlan,
  type ClipSourceObservation,
  type ClipObservedSegment,
} from '@/entities/clip-project'
import { Button, FieldLabel, FieldMessage, RangeSlider, TextField, Typography } from '@/shared/ui'

export function AddObservedCut({
  observation,
  segment,
  plan,
  onAddCut,
}: {
  observation: ClipSourceObservation
  segment: ClipObservedSegment
  plan?: ClipEditPlan
  onAddCut: (selection: ClipAddCutSelection) => Promise<void>
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const [range, setRange] = useState<[number, number]>([segment.startMs, segment.endMs])
  const [pending, setPending] = useState(false)
  const [failed, setFailed] = useState(false)
  const [startMs, endMs] = range
  const valid =
    Number.isSafeInteger(startMs) &&
    Number.isSafeInteger(endMs) &&
    startMs >= segment.startMs &&
    endMs <= segment.endMs &&
    endMs - startMs > 2 * CLIP_TRANSITION.fade_ms
  const overlaps = plan?.cuts.some(
    (c) =>
      c.sourceId === observation.source.id &&
      c.fingerprint === observation.source.fingerprint &&
      c.startMs < endMs &&
      c.endMs > startMs,
  )
  return (
    <fieldset disabled={pending} className="min-w-0 space-y-3">
      <Typography variant="fieldTitle">{t('assembly.addTitle')}</Typography>
      <RangeSlider
        startLabel={t('assembly.addStart')}
        endLabel={t('assembly.addEnd')}
        value={range}
        min={segment.startMs}
        max={segment.endMs}
        step={1}
        format={(ms) => `${clipSeconds(ms)} s`}
        onChange={setRange}
        onCommit={() => {}}
      />
      <div className="grid grid-cols-2 gap-3">
        {(['addStart', 'addEnd'] as const).map((label, index) => (
          <div key={label}>
            <FieldLabel htmlFor={`${id}-${index}`}>{t(`assembly.${label}`)}</FieldLabel>
            <TextField
              id={`${id}-${index}`}
              type="number"
              inputMode="decimal"
              step="0.001"
              value={Number.isFinite(range[index]) ? range[index] / 1000 : ''}
              aria-invalid={!valid || !!overlaps}
              onChange={(event) => {
                const next: [number, number] = [...range]
                next[index] =
                  event.target.value === ''
                    ? NaN
                    : Number((Number(event.target.value) * 1000).toFixed(6))
                setRange(next)
              }}
            />
          </div>
        ))}
      </div>
      {(!valid || overlaps) && (
        <FieldMessage>{t(overlaps ? 'assembly.overlap' : 'timeline.rangeInvalid')}</FieldMessage>
      )}
      {!segment.focal && <FieldMessage>{t('assembly.observationUnavailable')}</FieldMessage>}
      <Button
        variant="secondary"
        disabled={!valid || overlaps || !segment.focal || segment.usability === 'unusable'}
        pending={pending}
        onClick={() => {
          setPending(true)
          setFailed(false)
          void onAddCut({ source: observation.source, segment, startMs, endMs })
            .catch(() => setFailed(true))
            .finally(() => setPending(false))
        }}
      >
        {t('assembly.add')}
      </Button>
      {failed && <FieldMessage>{t('assembly.saveFirst')}</FieldMessage>}
    </fieldset>
  )
}
