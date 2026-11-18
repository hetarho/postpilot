import { useMemo } from 'react'
import { useVoices } from '@/entities/voice'
import { listedLastTemplate, useLastTemplate, useTemplates } from '@/entities/template'
import { useGuidelines } from '@/entities/guideline'
import { useMemories } from '@/entities/memory'
import { sameRef, useModels, useSelections, useStageSelection } from '@/entities/model-catalog'
import {
  aiElement,
  failedElement,
  guidelineElement,
  kitSummary,
  listElement,
  nextKitStep,
  voiceElement,
  type KitElement,
  type KitKey,
} from './kit'

/** The account's writing kit, read from the same queries the kit's own screens read — so the
 *  picture on 홈 and the screen it opens can never disagree. A list that could not be read is
 *  `failed`, never "missing": the picture does not tell an owner with three templates that they
 *  have none because one request failed. */
export function useWritingKit(ownerId: string) {
  const voices = useVoices(ownerId)
  const templates = useTemplates(ownerId)
  const chosenTemplate = listedLastTemplate(useLastTemplate(ownerId), templates.templates)
  const guidelines = useGuidelines(ownerId, 'post')
  const memories = useMemories(ownerId)
  const write = useStageSelection('write')
  // The two reads the write stage's choice is judged from, for their failure and their retry.
  const catalog = useModels()
  const selections = useSelections()
  const writeLabel = write.selected
    ? (write.models.find((model) => sameRef(model.ref, write.selected!))?.label ??
      write.selected.modelId)
    : null
  const defaultsInUse = guidelines.defaults.filter(
    (guideline) => guideline.enabled && !guideline.principle,
  ).length

  const voicesFailed = voices.isError && voices.voices.length === 0
  const templatesFailed = templates.isError && templates.templates.length === 0
  const guidelinesFailed =
    guidelines.isError && guidelines.guidelines.length === 0 && guidelines.defaults.length === 0
  const memoriesFailed = memories.isError && memories.memories.length === 0
  const aiFailed = !write.selected && (catalog.isError || selections.isError)

  const elements = useMemo<Record<KitKey, KitElement>>(
    () => ({
      voice: voicesFailed
        ? failedElement('voice')
        : voiceElement(voices.active, !ownerId || voices.isPending),
      template: templatesFailed
        ? failedElement('template')
        : listElement(
            'template',
            templates.templates,
            !ownerId || templates.isPending,
            chosenTemplate,
          ),
      guideline: guidelinesFailed
        ? failedElement('guideline')
        : guidelineElement(
            guidelines.guidelines.length,
            defaultsInUse,
            !ownerId || guidelines.isPending,
          ),
      ai: aiFailed
        ? failedElement('ai')
        : aiElement(writeLabel, !!write.unavailable, write.isPending),
      memory: memoriesFailed
        ? failedElement('memory')
        : listElement(
            'memory',
            memories.memories.map(() => ({})),
            !ownerId || memories.isPending,
          ),
    }),
    [
      ownerId,
      voicesFailed,
      voices.active,
      voices.isPending,
      templatesFailed,
      templates.templates,
      templates.isPending,
      chosenTemplate,
      guidelinesFailed,
      guidelines.guidelines.length,
      guidelines.isPending,
      defaultsInUse,
      aiFailed,
      writeLabel,
      write.unavailable,
      write.isPending,
      memoriesFailed,
      memories.memories,
      memories.isPending,
    ],
  )
  const ordered = useMemo(
    () => [elements.voice, elements.template, elements.guideline, elements.ai, elements.memory],
    [elements],
  )
  /** Reads again only what failed. */
  const retry = () => {
    if (voicesFailed) voices.refetch()
    if (templatesFailed) templates.refetch()
    if (guidelinesFailed) guidelines.refetch()
    if (memoriesFailed) memories.refetch()
    if (aiFailed) {
      if (catalog.isError) catalog.refetch()
      if (selections.isError) selections.refetch()
    }
  }
  return {
    elements,
    ordered,
    summary: kitSummary(ordered),
    next: nextKitStep(ordered),
    retry,
    retrying:
      (voicesFailed && voices.isFetching) ||
      (templatesFailed && templates.isFetching) ||
      (guidelinesFailed && guidelines.isFetching) ||
      (memoriesFailed && memories.isFetching),
  }
}
