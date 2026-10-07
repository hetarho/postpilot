import { useCallback, useState, type ComponentProps } from 'react'
import {
  ClipDraftPreview,
  type BrowserCompositionDesign,
  type BrowserLocalOriginal,
} from '@/entities/clip-preview'
import type { ClipLayoutObservations } from '@/entities/clip-observation'
import type { ClipRatioId } from '@/entities/clip-design'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { useClipSpeechCalls } from '@/entities/clip-plan'
import type { ClipSpeechRef } from '@/entities/clip-plan'
import { useClipDraftPreview } from '../model/useClipDraftPreview'

type PreviewProps = Omit<ComponentProps<typeof ClipDraftPreview>, 'preview' | 'plan' | 'timeMs'>

/** The draft preview as a page uses it: the fetching verb around the entity's player. The
 *  position is mirrored here because the overlay is prepared for the elements on screen. */
export function ClipDraftPreviewPanel({
  ownerId,
  projectId,
  projectRevision,
  revision,
  design,
  layoutObservations,
  localSources,
  plan,
  timeMs: controlledTime,
  onTimeChange,
  ...rest
}: PreviewProps & {
  ownerId: string
  projectId: string
  projectRevision?: number
  design?: Partial<BrowserCompositionDesign>
  layoutObservations?: ClipLayoutObservations
  localSources?: readonly BrowserLocalOriginal[]
  revision: number
  plan: ClipEditPlan
  timeMs?: number
  onTimeChange?: (ms: number) => void
}) {
  const [localTime, setLocalTime] = useState(0)
  const speech = useClipSpeechCalls()
  const loadSpeech = useCallback(
    (asset: ClipSpeechRef, signal: AbortSignal) => speech.load(projectId, asset, signal),
    [speech, projectId],
  )
  const timeMs = controlledTime ?? localTime
  const preview = useClipDraftPreview({
    ownerId,
    projectId,
    projectRevision,
    revision,
    plan,
    timeMs,
    ratio: rest.ratio as ClipRatioId,
    sources: rest.sources,
    resolvePlayback: rest.resolvePlayback,
    design,
    layoutObservations,
    localSources,
  })
  // The player uses this callback to register its frame clock. Keep its identity through
  // playhead updates so a new video-frame callback is not torn down on every frame.
  const changeTime = useCallback(
    (ms: number) => {
      setLocalTime(ms)
      onTimeChange?.(ms)
    },
    [onTimeChange],
  )
  return (
    <ClipDraftPreview
      {...rest}
      plan={plan}
      preview={preview}
      localMode
      local={preview.local}
      loadSpeech={loadSpeech}
      timeMs={timeMs}
      onTimeChange={changeTime}
    />
  )
}
