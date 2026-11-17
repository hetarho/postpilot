import { useMemo } from 'react'
import { useVoices } from '@/entities/voice'
import { useTemplates } from '@/entities/template'
import { useGuidelines } from '@/entities/guideline'
import { useMemories } from '@/entities/memory'
import { sameRef, useStageSelection } from '@/entities/model-catalog'
import {
  aiElement,
  guidelineElement,
  kitSummary,
  listElement,
  nextKitStep,
  voiceElement,
  type KitElement,
  type KitKey,
} from './kit'

/** The account's writing kit, read from the same queries the kit's own screens read — so the
 *  picture on 홈 and the screen it opens can never disagree. */
export function useWritingKit(ownerId: string) {
  const voices = useVoices(ownerId)
  const templates = useTemplates(ownerId)
  const guidelines = useGuidelines(ownerId, 'post')
  const memories = useMemories(ownerId)
  const write = useStageSelection('write')
  const writeLabel = write.selected
    ? (write.models.find((model) => sameRef(model.ref, write.selected!))?.label ??
      write.selected.modelId)
    : null
  const defaultsInUse = guidelines.defaults.filter(
    (guideline) => guideline.enabled && !guideline.principle,
  ).length

  const elements = useMemo<Record<KitKey, KitElement>>(
    () => ({
      voice: voiceElement(voices.active, !ownerId || voices.isPending),
      template: listElement('template', templates.templates, !ownerId || templates.isPending),
      guideline: guidelineElement(
        guidelines.guidelines.length,
        defaultsInUse,
        !ownerId || guidelines.isPending,
      ),
      ai: aiElement(writeLabel, !!write.unavailable, write.isPending),
      memory: listElement(
        'memory',
        memories.memories.map(() => ({})),
        !ownerId || memories.isPending,
      ),
    }),
    [
      ownerId,
      voices.active,
      voices.isPending,
      templates.templates,
      templates.isPending,
      guidelines.guidelines.length,
      guidelines.isPending,
      defaultsInUse,
      writeLabel,
      write.unavailable,
      write.isPending,
      memories.memories,
      memories.isPending,
    ],
  )
  const ordered = useMemo(
    () => [elements.voice, elements.template, elements.guideline, elements.ai, elements.memory],
    [elements],
  )
  return {
    elements,
    ordered,
    summary: kitSummary(ordered),
    next: nextKitStep(ordered),
  }
}
