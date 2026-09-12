import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  CLIP_COMPOSITION_LIMITS,
  CLIP_COMPOSITION_PREVIEW,
  CLIP_DESIGN,
  type ClipRatioId,
} from '@/shared/config'
import { Slider, Typography } from '@/shared/ui'
import {
  CompositionProblem,
  type ClipComposition,
  type CompositionTimeline,
} from '../model/composition'
import { sampleClipComposition } from '../lib/composition-sample'
import { CompositionSelect } from './CompositionFields'

export function CompositionPreview({ document }: { document: ClipComposition }) {
  const { t } = useTranslation('clips')
  const [duration, setDuration] = useState<number>(CLIP_COMPOSITION_PREVIEW.durationMs)
  const [time, setTime] = useState(0)
  const [ratio, setRatio] = useState<ClipRatioId>('vertical')
  let timeline: CompositionTimeline | undefined, error: CompositionProblem | undefined
  try {
    timeline = sampleClipComposition(document, duration, (label, n) =>
      t('composition.sampleValue', { label, n }),
    )
  } catch (e) {
    if (e instanceof CompositionProblem) error = e
    else throw e
  }
  const shape = CLIP_DESIGN.ratios[ratio]
  const active = timeline?.elements.filter((e) => e.startMs <= time && time < e.endMs) ?? []
  return (
    <section className="min-w-0 space-y-4" aria-label={t('composition.preview')}>
      <Typography variant="fieldTitle" as="h2">
        {t('composition.preview')}
      </Typography>
      <Typography variant="body" className="text-content-secondary">
        {t('composition.previewHelp')}
      </Typography>
      <CompositionSelect
        label={t('project.ratio')}
        value={ratio}
        options={(['vertical', 'horizontal', 'square'] as const).map((value) => ({
          value,
          label: t(`ratio.${value}`),
        }))}
        onChange={(value) => setRatio(value as ClipRatioId)}
      />
      <Slider
        label={t('composition.sampleDuration')}
        min={CLIP_COMPOSITION_PREVIEW.minDurationMs}
        max={CLIP_COMPOSITION_LIMITS.maxDurationMs}
        step={CLIP_COMPOSITION_PREVIEW.stepMs}
        value={duration}
        valueText={t('composition.seconds', { value: duration / 1000 })}
        onChange={(value) => {
          setDuration(value)
          setTime(Math.min(time, value))
        }}
      />
      <Slider
        label={t('composition.sampleTime')}
        min={0}
        max={duration}
        step={CLIP_COMPOSITION_PREVIEW.stepMs}
        value={time}
        valueText={t('composition.seconds', { value: time / 1000 })}
        onChange={setTime}
      />
      {error ? (
        <Typography variant="body" role="status">
          {t('composition.previewError', { line: error.line, element: error.elementId })}
        </Typography>
      ) : (
        <>
          <svg
            viewBox={`0 0 ${shape.canvas.width} ${shape.canvas.height}`}
            className="mx-auto max-h-80 w-full"
            role="img"
            aria-label={t('composition.sampleFrame')}
          >
            <rect
              width={shape.canvas.width}
              height={shape.canvas.height}
              className="fill-surface-recessed"
            />
            <rect {...shape.safe} className="fill-surface-raised" />
            <text
              x={shape.canvas.width / 2}
              y={shape.canvas.height / 2}
              textAnchor="middle"
              fontSize={CLIP_DESIGN.type.caption.size}
              className="fill-content-secondary"
            >
              {t('composition.sampleOnly')}
            </text>
            {active.map((entry, i) => (
              <text
                key={entry.instanceId}
                x={shape.anchor.center}
                y={shape.anchor.top + (i + 1) * CLIP_DESIGN.type.caption.size}
                textAnchor="middle"
                fontSize={CLIP_DESIGN.type.caption.size}
                className="fill-content-primary"
              >
                {t(`composition.role.${entry.element.role}`)}
              </text>
            ))}
          </svg>
          <ul className="space-y-2">
            {active.map((entry) => (
              <li key={entry.instanceId} className="break-words">
                <Typography variant="meta">
                  {t(`composition.role.${entry.element.role}`)} · {entry.startMs / 1000}–
                  {entry.endMs / 1000}s
                </Typography>
                <Typography variant="body">
                  {entry.element.kind === 'ai'
                    ? t('composition.sampleAI')
                    : entry.rows.length
                      ? entry.rows.map((r) => r.text).join(' / ')
                      : entry.text}
                </Typography>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  )
}
