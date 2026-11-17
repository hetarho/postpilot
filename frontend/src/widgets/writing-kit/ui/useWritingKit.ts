import { useAuthoringSummaries } from '@/entities/ai-authoring'
import { useGuidelineCandidates, useGuidelines } from '@/entities/guideline'
import { useMemories } from '@/entities/memory'
import { useStageSelection } from '@/entities/model-catalog'
import { useSpokenLibrary } from '@/entities/spoken-voice'
import { useTemplates } from '@/entities/template'
import { useVoices } from '@/entities/voice'
import { kitParts, kitSummary, type KitInput, type KitPartKey } from '../model/readiness'

/** Each part's input, read with the same queries the part's own screen reads, so opening one after
 *  the home costs no second request. Reads only: nothing here creates, starts or charges. */
function useVoiceInput(ownerId: string): KitInput['voices'] {
  const voices = useVoices(ownerId)
  return { isPending: voices.isPending, isError: voices.isError, active: voices.active }
}
function useTemplateInput(ownerId: string): KitInput['templates'] {
  const templates = useTemplates(ownerId)
  const work = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  return {
    isPending: templates.isPending,
    isError: templates.isError,
    count: templates.templates.length,
    drafts: work.data?.filter((row) => !row.targetId && !row.lastPublication).length ?? 0,
  }
}
function useGuidelineInput(ownerId: string, suggestions = 0): KitInput['guidelines'] {
  const guidelines = useGuidelines(ownerId, 'post')
  return {
    isPending: guidelines.isPending,
    isError: guidelines.isError,
    own: guidelines.guidelines.length,
    defaultsInUse: guidelines.defaults.filter(
      (guideline) => guideline.enabled && !guideline.principle,
    ).length,
    suggestions,
  }
}
function useMemoryInput(ownerId: string): KitInput['memories'] {
  const memories = useMemories(ownerId)
  return {
    isPending: memories.isPending,
    isError: memories.isError,
    count: memories.memories.length,
  }
}
function useModelInput(): KitInput['model'] {
  const write = useStageSelection('write')
  const selectedLabel = write.selected
    ? (write.models.find(
        (model) =>
          model.ref.providerId === write.selected?.providerId &&
          model.ref.modelId === write.selected?.modelId,
      )?.label ?? write.selected.modelId)
    : null
  return { isPending: write.isPending, selectedLabel, unusable: !!write.unavailable }
}
function useSpokenInput(ownerId: string): KitInput['spoken'] {
  const spoken = useSpokenLibrary(ownerId)
  return {
    isPending: spoken.voicesQuery.isPending,
    isError: spoken.voicesQuery.isError,
    count: spoken.voices.length,
    drafts: spoken.drafts.filter((draft) => !draft.confirmedVoiceId).length,
  }
}

const SETTLED_EMPTY: KitInput = {
  voices: { isPending: false, isError: false, active: [] },
  templates: { isPending: false, isError: false, count: 0, drafts: 0 },
  guidelines: { isPending: false, isError: false, own: 0, defaultsInUse: 0, suggestions: 0 },
  memories: { isPending: false, isError: false, count: 0 },
  model: { isPending: false, selectedLabel: null, unusable: false },
  spoken: { isPending: false, isError: false, count: 0, drafts: 0 },
}

/** Every part of the kit, folded into parts and a summary — the home's graphic. */
export function useWritingKit(ownerId: string) {
  const suggestions = useGuidelineCandidates(ownerId, 'post')
  const parts = kitParts({
    voices: useVoiceInput(ownerId),
    templates: useTemplateInput(ownerId),
    guidelines: useGuidelineInput(ownerId, suggestions.candidates.length),
    memories: useMemoryInput(ownerId),
    model: useModelInput(),
    spoken: useSpokenInput(ownerId),
  })
  return { parts, summary: kitSummary(parts) }
}

function useVoicePartInput(ownerId: string): Partial<KitInput> {
  return { voices: useVoiceInput(ownerId) }
}
function useTemplatePartInput(ownerId: string): Partial<KitInput> {
  return { templates: useTemplateInput(ownerId) }
}
function useGuidelinePartInput(ownerId: string): Partial<KitInput> {
  return { guidelines: useGuidelineInput(ownerId) }
}
function useMemoryPartInput(ownerId: string): Partial<KitInput> {
  return { memories: useMemoryInput(ownerId) }
}
function useModelPartInput(): Partial<KitInput> {
  return { model: useModelInput() }
}
function useSpokenPartInput(ownerId: string): Partial<KitInput> {
  return { spoken: useSpokenInput(ownerId) }
}

/** One part's input hook, by part — each reads only that part's lists, for a kit screen's head. */
export const KIT_PART_INPUTS: Record<KitPartKey, (ownerId: string) => Partial<KitInput>> = {
  voice: useVoicePartInput,
  template: useTemplatePartInput,
  guideline: useGuidelinePartInput,
  memory: useMemoryPartInput,
  model: useModelPartInput,
  spoken: useSpokenPartInput,
}

/** The part `key` from one part's input, every other part settled empty. */
export function kitPartFrom(key: KitPartKey, input: Partial<KitInput>) {
  return kitParts({ ...SETTLED_EMPTY, ...input })[key]
}
