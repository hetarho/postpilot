import { useAuthoringSummaries } from '@/entities/ai-authoring'
import { useGuidelines } from '@/entities/guideline'
import { useMemories } from '@/entities/memory'
import { useTemplates } from '@/entities/template'
import { useVoices } from '@/entities/voice'
import { readWritingKit, readyCount, type KitElement, type KitReading } from './readiness'

/** One element as the kit shows it: its reading, and the owner's own words for it — the voice's
 *  name, a template's name, a rule, a remembered fact — so the kit names what is in it rather
 *  than only how much. */
export interface KitEntry {
  reading: KitReading
  /** The first thing the element holds, in the owner's words; absent while it holds nothing. */
  sample?: string
  /** How many the owner made (made voices, saved templates, own rules, memories). */
  count: number
  /** What stands in while the owner has made none: unsaved drafts, 기본 지침 in use, or the share
   *  a voice still being made has reached. */
  extra: number
  /** The one thing to open to make this element ready, when it is a particular item (the voice
   *  still being made). */
  openId?: string
}

export interface WritingKit {
  entries: Record<KitElement, KitEntry>
  readings: KitReading[]
  ready: number
  pending: boolean
  failed: boolean
  retry: () => void
}

/** Reads the kit from the lists the kit screens already read, so opening a kit screen from here
 *  costs no second request. */
export function useWritingKit(ownerId: string): WritingKit {
  const voices = useVoices(ownerId)
  const templates = useTemplates(ownerId)
  const drafts = useAuthoringSummaries({ ownerId, kind: 'post-template' })
  const guidelines = useGuidelines(ownerId, 'post')
  const memories = useMemories(ownerId)
  const unsavedDrafts = (drafts.data ?? []).filter(
    (row) => !row.targetId && !row.lastPublication,
  ).length
  const defaultsInUse = guidelines.defaults.filter(
    (guideline) => guideline.enabled && !guideline.principle,
  ).length
  const readings = readWritingKit({
    voices: { pending: voices.isPending, failed: voices.isError, voices: voices.active },
    templates: {
      pending: templates.isPending,
      failed: templates.isError,
      saved: templates.templates.length,
      unsavedDrafts,
    },
    guidelines: {
      pending: guidelines.isPending,
      failed: guidelines.isError,
      own: guidelines.guidelines.length,
      defaultsInUse,
    },
    memories: {
      pending: memories.isPending,
      failed: memories.isError,
      count: memories.memories.length,
    },
  })
  const byKey = Object.fromEntries(readings.map((reading) => [reading.key, reading])) as Record<
    KitElement,
    KitReading
  >
  const made = voices.active.filter((voice) => voice.made)
  const shownVoice = voices.defaultVoice && voices.defaultVoice.made ? voices.defaultVoice : made[0]
  const making = voices.active.filter((voice) => !voice.made)
  const firstRule = guidelines.guidelines[0]
  return {
    readings,
    ready: readyCount(readings),
    pending: readings.some((reading) => reading.pending),
    failed: readings.some((reading) => reading.failed),
    retry: () => {
      voices.refetch()
      templates.refetch()
      guidelines.refetch()
      memories.refetch()
      void drafts.refetch()
    },
    entries: {
      voices: {
        reading: byKey.voices,
        sample: shownVoice?.name ?? making[0]?.name,
        count: made.length,
        extra: Math.max(0, ...making.map((voice) => voice.readinessPercent)),
        openId: made.length
          ? undefined
          : [...making].sort((a, b) => b.readinessPercent - a.readinessPercent)[0]?.id,
      },
      templates: {
        reading: byKey.templates,
        sample: templates.templates[0]?.name,
        count: templates.templates.length,
        extra: unsavedDrafts,
      },
      guidelines: {
        reading: byKey.guidelines,
        sample: firstRule ? firstRule.title || firstRule.text : undefined,
        count: guidelines.guidelines.length,
        extra: defaultsInUse,
      },
      memories: {
        reading: byKey.memories,
        sample: memories.memories[0]?.text,
        count: memories.memories.length,
        extra: 0,
      },
    },
  }
}
