import { useSession } from '@/entities/session'
import { useVoiceProfile, useVoices, type Voice } from '@/entities/voice'
import { useTemplates, type Template } from '@/entities/template'
import { useGuidelines } from '@/entities/guideline'
import { useMemories, type Memory } from '@/entities/memory'
import { sameRef, useStageSelection } from '@/entities/model-catalog'
import { useSpokenLibrary } from '@/entities/spoken-voice'
import { kitReadiness, kitSummary, type KitReadiness } from './readiness'

/** Everything home draws, read from the lists the kit's own screens read — no new request and no
 *  number of its own. Mounting it calls no model and starts no job. */
export interface WritingKit {
  kit: KitReadiness[]
  summary: ReturnType<typeof kitSummary>
  template?: { first: Template; count: number }
  voice?: {
    voice: Voice
    sample?: string
    fictional: boolean
    phrases: string[]
  }
  memories: { shown: Memory[]; count: number }
  guidelines: {
    shown: { key: string; text: string; builtIn: boolean }[]
    own: number
    defaults: number
  }
  writer: { pending: boolean; label?: string }
  spokenCount?: number
}

export function useWritingKit(): WritingKit {
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const voices = useVoices(ownerId)
  const templates = useTemplates(ownerId)
  const guidelines = useGuidelines(ownerId, 'post')
  const memories = useMemories(ownerId)
  const write = useStageSelection('write')
  const spoken = useSpokenLibrary(ownerId)
  // The voice a new post is written in: the default when it is made, otherwise the first made one.
  const shownVoice =
    (voices.defaultVoice?.made ? voices.defaultVoice : undefined) ??
    voices.active.find((voice) => voice.made)
  const { profile } = useVoiceProfile(ownerId, shownVoice?.id ?? '')
  const analysis = profile?.analysis
  const defaultsInUse = guidelines.defaults.filter(
    (guideline) => guideline.enabled && !guideline.principle,
  )
  const kit = kitReadiness({
    voices: {
      pending: voices.isPending,
      failed: voices.isError,
      active: voices.active,
    },
    templates: {
      pending: templates.isPending,
      failed: templates.isError,
      count: templates.templates.length,
    },
    guidelines: {
      pending: guidelines.isPending,
      failed: guidelines.isError,
      own: guidelines.guidelines.length,
      defaultsInUse: defaultsInUse.length,
    },
    memories: {
      pending: memories.isPending,
      failed: memories.isError,
      count: memories.memories.length,
    },
  })
  const first = templates.templates[0]
  const writer = write.selected
  return {
    kit,
    summary: kitSummary(kit),
    template: first ? { first, count: templates.templates.length } : undefined,
    voice: shownVoice
      ? {
          voice: shownVoice,
          sample: analysis?.syntheticSample || analysis?.ai?.examples[0]?.sentence,
          fictional: Boolean(analysis?.syntheticSample) || analysis?.origin === 'synthetic',
          phrases: analysis?.ai?.signaturePhrases.slice(0, 3) ?? [],
        }
      : undefined,
    memories: { shown: memories.memories.slice(0, 3), count: memories.memories.length },
    guidelines: {
      // The owner's own rules first — they are what makes the list theirs — then the 기본 지침.
      shown: [
        ...guidelines.guidelines.map((guideline) => ({
          key: guideline.id,
          text: guideline.title || guideline.text,
          builtIn: false,
        })),
        ...defaultsInUse.map((guideline) => ({
          key: guideline.key,
          text: guideline.name,
          builtIn: true,
        })),
      ].slice(0, 3),
      own: guidelines.guidelines.length,
      defaults: defaultsInUse.length,
    },
    writer: {
      pending: write.isPending,
      label: writer ? write.models.find((model) => sameRef(model.ref, writer))?.label : undefined,
    },
    spokenCount: spoken.voicesQuery.isPending ? undefined : spoken.voices.length,
  }
}
