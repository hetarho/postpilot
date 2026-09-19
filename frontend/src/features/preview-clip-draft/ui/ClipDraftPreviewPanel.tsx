import { useState, type ComponentProps } from 'react'
import { ClipDraftPreview } from '@/entities/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan'
import { useClipDraftPreview } from '../model/useClipDraftPreview'

type PreviewProps = Omit<ComponentProps<typeof ClipDraftPreview>, 'preview' | 'plan' | 'timeMs'>

/** The draft preview as a page uses it: the fetching verb around the entity's player. The
 *  position is mirrored here because the overlay is prepared for the elements on screen. */
export function ClipDraftPreviewPanel({
  projectId,
  revision,
  plan,
  timeMs: controlledTime,
  onTimeChange,
  ...rest
}: PreviewProps & {
  projectId: string
  revision: number
  plan: ClipEditPlan
  timeMs?: number
  onTimeChange?: (ms: number) => void
}) {
  const [localTime, setLocalTime] = useState(0)
  const timeMs = controlledTime ?? localTime
  const preview = useClipDraftPreview({ projectId, revision, plan, timeMs })
  return (
    <ClipDraftPreview
      {...rest}
      plan={plan}
      preview={preview}
      timeMs={timeMs}
      onTimeChange={(ms) => {
        setLocalTime(ms)
        onTimeChange?.(ms)
      }}
    />
  )
}
