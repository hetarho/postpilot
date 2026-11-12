import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import {
  clipSeconds,
  spokenState,
  type ClipNarration,
  type ClipSpokenSegment,
} from '@/entities/clip-plan'
import { Button, Sheet, Typography } from '@/shared/ui'

/** A spoken segment's own sheet (CLIP-55, DUB-20): the segment's script and its speech state.
 *  The body is the owner's script editor; typing only marks the changed speech stale and nothing
 *  here or in undo/redo synthesizes it. Read-only, or with no editor, it states the script. */
export function ClipSegmentSheet({
  open,
  onClose,
  narration,
  segment,
  durationMs,
  children,
}: {
  open: boolean
  onClose: () => void
  narration: ClipNarration
  segment: ClipSpokenSegment
  durationMs: number
  /** The script editor, absent while the plan is only read. */
  children?: ReactNode
}) {
  const { t } = useTranslation('clips')
  const { t: tCommon } = useTranslation('common')
  const titleId = `clip-spoken-sheet-${segment.id}`
  const number = narration.segments.indexOf(segment) + 1
  const state = spokenState(narration, segment, durationMs)
  return (
    <Sheet
      open={open}
      labelledBy={titleId}
      onClose={onClose}
      header={
        <div className="mb-3 flex items-start justify-between gap-3">
          <div className="min-w-0 space-y-1">
            <Typography variant="fieldTitle" id={titleId}>
              {t('timeline.spokenSegment', { number })}
            </Typography>
            <Typography variant="meta" as="p" data-spoken-state={state}>
              {t(`timeline.speechState.${state}`)}
            </Typography>
          </div>
          <Button
            variant="ghost"
            size="icon"
            aria-label={tCommon('action.close')}
            onClick={onClose}
          >
            <X aria-hidden="true" className="size-5" />
          </Button>
        </div>
      }
    >
      {children ?? (
        <div className="min-w-0 space-y-2">
          <Typography variant="body" className="break-words whitespace-pre-wrap">
            {segment.text}
          </Typography>
          <Typography variant="meta">
            {clipSeconds(segment.startMs)}–{clipSeconds(segment.endMs)} s
          </Typography>
        </div>
      )}
    </Sheet>
  )
}
