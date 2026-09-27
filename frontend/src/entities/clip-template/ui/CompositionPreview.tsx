import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  CLIP_COMPOSITION_LIMITS,
  CLIP_COMPOSITION_PREVIEW,
  CLIP_DEFAULT_CAPTION_STYLE,
  CLIP_DEFAULT_REGION_PRESETS,
  CLIP_REGIONS,
} from '@/entities/clip-design/@x/clip-template'
import { type ClipRatioId, type ClipRegionPresets } from '@/entities/clip-design/@x/clip-template'
import { Slider, Typography } from '@/shared/ui'
import {
  CompositionProblem,
  type ClipComposition,
  type CompositionTimeline,
} from '../model/composition'
import { sampleClipComposition } from '../lib/composition-sample'
import { CompositionSelect } from './CompositionFields'
import { CompositionDesignFrame } from './CompositionDesignFrame'

const INTRO_IDS = Object.keys(CLIP_REGIONS.intro) as ClipRegionPresets['intro'][]
const OUTRO_IDS = Object.keys(CLIP_REGIONS.outro) as ClipRegionPresets['outro'][]

/** The template's preview (CLIP-42). Its intro and outro selectors are the template's own
 *  starting design (CLIP-166) where the editor hands them in — the preview draws the draft's
 *  selection and each choice edits the draft — and `children` is the rest of that selection,
 *  standing beside them. Without them the selectors only choose what this preview draws in. */
export function CompositionPreview({
  document,
  presets: chosen,
  onPresetsChange,
  captionStyles = [],
  children,
}: {
  document: ClipComposition
  presets?: ClipRegionPresets
  onPresetsChange?: (next: ClipRegionPresets) => void
  /** The styles the sample captions take in turn; none is the default style alone. */
  captionStyles?: readonly string[]
  children?: ReactNode
}) {
  const { t } = useTranslation('clips')
  const [duration, setDuration] = useState<number>(CLIP_COMPOSITION_PREVIEW.durationMs)
  const [time, setTime] = useState(0)
  const [ratio, setRatio] = useState<ClipRatioId>('vertical')
  const [local, setLocal] = useState<ClipRegionPresets>(CLIP_DEFAULT_REGION_PRESETS)
  const presets = chosen ?? local
  const setPresets = onPresetsChange ?? setLocal
  let timeline: CompositionTimeline | undefined, error: CompositionProblem | undefined
  try {
    timeline = sampleClipComposition(
      document,
      duration,
      (label, n) => t('composition.sampleValue', { label, n }),
      {
        caption: (n) => t('composition.sampleCaption', { n }),
        styles: captionStyles.length ? captionStyles : [CLIP_DEFAULT_CAPTION_STYLE],
      },
    )
  } catch (e) {
    if (e instanceof CompositionProblem) error = e
    else throw e
  }
  // The scrubber's end is the clip's last frame, not the instant after it (CLIP-171): the
  // last millisecond is drawn there, inside every interval that runs to the end.
  const instant = Math.min(time, duration - 1)
  const active = timeline?.elements.filter((e) => e.startMs <= instant && instant < e.endMs) ?? []
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
      <CompositionSelect
        label={t('composition.design.intro')}
        value={presets.intro}
        options={INTRO_IDS.map((value) => ({
          value,
          label: t(`composition.design.intro_${value}`),
        }))}
        onChange={(value) => setPresets({ ...presets, intro: value as ClipRegionPresets['intro'] })}
      />
      <CompositionSelect
        label={t('composition.design.outro')}
        value={presets.outro}
        options={OUTRO_IDS.map((value) => ({
          value,
          label: t(`composition.design.outro_${value}`),
        }))}
        onChange={(value) => setPresets({ ...presets, outro: value as ClipRegionPresets['outro'] })}
      />
      {children}
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
          <CompositionDesignFrame
            document={document}
            entries={active}
            ratio={ratio}
            label={t('composition.sampleFrame')}
            sampleAI={t('composition.sampleShortAI')}
            presets={presets}
          />
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
