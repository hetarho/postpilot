import { useTranslation } from 'react-i18next'
import { useClipRegionPresetSamples } from '@/entities/clip-plan'
import { CLIP_DESIGN, type ClipRegionKind } from '@/entities/clip-design'
import type { ClipRatio } from '@/entities/clip-project'
import type { ClipRegionsEditor } from '../model/useClipRegionsEditor'
import { ClipRegionBlock } from './ClipRegionBlock'

/** One region's block, drawn from the editor the workspace holds. The numbered diagram is the
 *  one ① asks the renderer for — the same query, so it is drawn once (CLIP-165); while it loads,
 *  or when it fails, the block goes without it. */
export function ClipRegionEditor({
  editor,
  kind,
  projectId,
  ratio,
}: {
  editor: ClipRegionsEditor
  kind: ClipRegionKind
  projectId: string
  ratio: ClipRatio
}) {
  const { t } = useTranslation('clips')
  const samples = useClipRegionPresetSamples(projectId, t('composition.design.slotLabel'))
  const region = editor.regions?.[kind]
  if (!region) return null
  const preset = editor.presets[kind]
  return (
    <ClipRegionBlock
      kind={kind}
      region={region}
      capacity={editor.capacity[kind]}
      presetName={
        kind === 'intro'
          ? t(`composition.design.intro_${editor.presets.intro}`)
          : t(`composition.design.outro_${editor.presets.outro}`)
      }
      diagram={samples.data?.[kind].find((sample) => sample.preset === preset)?.svg}
      canvas={samples.data?.canvas ?? CLIP_DESIGN.ratios[ratio].canvas}
      drawable={editor.drawable[kind]}
      errors={editor.errors}
      notices={editor.notices}
      readOnly={editor.readOnly}
      onEnabled={(enabled) => editor.setEnabled(kind, enabled)}
      onSlot={(id, change) => editor.setSlot(kind, id, change)}
      onMove={(from, to) => editor.move(kind, from, to)}
    />
  )
}
