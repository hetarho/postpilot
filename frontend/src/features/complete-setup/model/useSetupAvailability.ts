import { useState } from 'react'
import { useVoices } from '@/entities/voice'
import { useTemplates } from '@/entities/template'
import { useClipTemplates } from '@/entities/clip-template'
import { emptySetupProgress } from './setup-machine'
import { readSetupProgress, writeSetupProgress } from './setup-progress'

/** The caller keys its account boundary by ownerId. Disabled reads never look like empty data. */
export function useSetupAvailability(ownerId: string, restart = false, alwaysRead = false) {
  const [progress, setProgress] = useState(() =>
    restart ? emptySetupProgress() : readSetupProgress(ownerId),
  )
  const readOwner = progress.completed && !alwaysRead ? '' : ownerId
  const voices = useVoices(readOwner)
  const posts = useTemplates(readOwner)
  const clips = useClipTemplates(readOwner)
  const status =
    progress.completed && !alwaysRead
      ? 'ready'
      : voices.isError || posts.isError || clips.isError
        ? 'failed'
        : voices.isPending || posts.isPending || clips.isPending
          ? 'checking'
          : 'ready'
  const missingVoice = !voices.active.some((voice) => voice.made)
  const missingPostTemplate = posts.templates.length === 0
  const missingClipTemplate = clips.templates.length === 0
  const needed = !progress.completed && (missingVoice || missingPostTemplate || missingClipTemplate)
  const acknowledge = () => {
    const next = { ...progress, completed: true }
    writeSetupProgress(ownerId, next)
    setProgress(next)
  }
  return {
    status,
    needed,
    progress,
    voices,
    posts,
    clips,
    missingVoice,
    missingPostTemplate,
    missingClipTemplate,
    acknowledge,
    retry: () => {
      voices.refetch()
      posts.refetch()
      void clips.refetch()
    },
  }
}
