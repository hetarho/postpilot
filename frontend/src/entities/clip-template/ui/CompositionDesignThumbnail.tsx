import { useTranslation } from 'react-i18next'
import { CLIP_COMPOSITION_PREVIEW, clipRegionSlots } from '@/entities/clip-design/@x/clip-template'
import { type ClipRegionPresets } from '@/entities/clip-design/@x/clip-template'
import { parseClipComposition } from '../lib/composition-parse'
import { sampleClipComposition } from '../lib/composition-sample'
import { CompositionDesignFrame } from './CompositionDesignFrame'

/** The body one preset is drawn from: an entry of the role being shown, with as
 *  many lines as that preset has slots, and a caption beside it for the caption
 *  sample. A template names no preset (CLIP-14), so the id comes from the
 *  surface asking for the picture. */
function sampleBody(kind: 'intro' | 'outro' | 'caption', preset: string) {
  const slots = kind === 'caption' ? 0 : clipRegionSlots(kind, preset).length
  const role = kind === 'intro' ? 'hook' : 'ending'
  const rows = Array.from({ length: slots }, () => '<row kind="ai"/>').join('')
  const region = slots ? `<text id="sample" kind="ai" role="${role}">${rows}</text>` : ''
  const caption =
    kind === 'caption' ? '<text id="sample-caption" kind="ai" role="caption">Scene</text>' : ''
  return `<clip version="1">${region}${caption}</clip>`
}

/** One preset drawn with the same frame the editor and the render draw with, so
 *  every surface that offers a preset offers the same picture of it (CDS-83). */
export function CompositionDesignThumbnail({
  kind,
  value,
}: {
  kind: 'intro' | 'outro' | 'caption'
  value: string
}) {
  const { t } = useTranslation('clips')
  const document = parseClipComposition(sampleBody(kind, value))
  // The representative drawing puts generated sample words in every slot.
  document.elements = document.elements.map((e) => ({
    ...e,
    kind: 'ai',
    rows: e.rows.map((r) => ({
      ...r,
      kind: 'ai',
      parts: [{ literal: t('composition.sampleShortAI'), field: '' }],
    })),
  }))
  const timeline = sampleClipComposition(document, CLIP_COMPOSITION_PREVIEW.durationMs, () =>
    t('composition.sampleShortAI'),
  )
  const presets: ClipRegionPresets = {
    intro: kind === 'intro' ? (value as 'a' | 'b') : 'b',
    outro: kind === 'outro' ? (value as 'b' | 'e') : 'e',
  }
  return (
    <div className="mx-auto mb-2 w-24" aria-hidden="true">
      <CompositionDesignFrame
        document={document}
        entries={timeline.elements}
        ratio="vertical"
        label={t(`composition.design.${kind}`)}
        sampleAI={t('composition.sampleShortAI')}
        presets={presets}
      />
    </div>
  )
}
