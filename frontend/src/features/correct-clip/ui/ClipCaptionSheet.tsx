import { useLayoutEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import type { AppFailure } from '@/shared/api'
import { type ClipNotice } from '@/entities/clip-project'
import {
  captionStartCut,
  outputToSourceMs,
  textInterval,
  type ClipCanvasBox,
  type ClipCaptionFragment,
  type ClipEditPlan,
  type ClipEditableText,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { CLIP_TIMELINE } from '@/entities/clip-design'
import { Button, Sheet, Typography, useVisualViewport } from '@/shared/ui'
import { ClipCaptionStage } from './ClipCaptionStage'
import { ClipTextControls } from './ClipTextControls'
import { ClipTextReading } from './ClipItemReading'
import type { ClipTextIssue } from '../model/caption-issues'

/** A caption's own sheet (CLIP-143, CLIP-191): its interval, phrases, position, size and style
 *  sample, placed over a representative frame from the cut its interval starts in. A badge or a
 *  narration caption opens the same sheet without the frame stage when it has none to place. */
export function ClipCaptionSheet({
  open,
  onClose,
  plan,
  text,
  stage,
  styles,
  footage,
  validation,
  failure,
  notices = [],
  language,
  readOnly = false,
  disabled = false,
  change,
}: {
  open: boolean
  onClose: () => void
  plan: ClipEditPlan
  text: ClipEditableText
  /** The renderer's drawing of this caption and the canvas it sits on, once it has arrived. */
  stage?: { canvas: ClipCanvasBox; safeArea: ClipCanvasBox; fragment?: ClipCaptionFragment }
  styles: {
    allowed?: readonly string[]
    samples?: readonly ClipCaptionFragment[]
    unavailable?: boolean
  }
  footage: {
    localSources: ReadonlyArray<{ fingerprint: string; url: string }>
    resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  }
  validation?: { invalid: boolean; regionOverlap?: boolean; issues?: readonly ClipTextIssue[] }
  failure?: AppFailure
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  readOnly?: boolean
  disabled?: boolean
  change: (edit: TimelineEdit, group?: string) => void
}) {
  const { t } = useTranslation('clips')
  const { t: tCommon } = useTranslation('common')
  const viewport = useVisualViewport()
  const headerRef = useRef<HTMLDivElement>(null)
  useLayoutEffect(() => {
    const panel = headerRef.current?.closest<HTMLElement>('[role=dialog]')
    const overlay = panel?.parentElement
    if (!open || !panel || !overlay) return
    overlay.style.top = `${viewport.offsetTop}px`
    overlay.style.bottom = `${Math.max(0, window.innerHeight - viewport.offsetTop - viewport.height)}px`
    panel.style.maxHeight = `${viewport.height}px`
    const body = panel.querySelector<HTMLElement>('.overflow-y-auto')
    let frame = 0
    let bounded:
      | {
          field: HTMLTextAreaElement
          maxHeight: string
          overflowY: string
          overscrollBehaviorY: string
        }
      | undefined
    const restoreField = () => {
      if (!bounded) return
      const { field, maxHeight, overflowY, overscrollBehaviorY } = bounded
      field.style.maxHeight = maxHeight
      field.style.overflowY = overflowY
      field.style.overscrollBehaviorY = overscrollBehaviorY
      bounded = undefined
    }
    const revealFocusedField = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        const field = document.activeElement
        if (
          !body ||
          !(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement) ||
          !body.contains(field)
        )
          return
        const bodyBox = body.getBoundingClientRect()
        const top = Math.max(bodyBox.top, viewport.offsetTop) + CLIP_TIMELINE.fieldGapPx
        const bottom =
          Math.min(bodyBox.bottom, viewport.offsetTop + viewport.height) - CLIP_TIMELINE.fieldGapPx
        const available = Math.max(0, bottom - top)
        if (
          field instanceof HTMLTextAreaElement &&
          available > 0 &&
          field.getBoundingClientRect().height > available
        ) {
          if (bounded?.field !== field) {
            restoreField()
            bounded = {
              field,
              maxHeight: field.style.maxHeight,
              overflowY: field.style.overflowY,
              overscrollBehaviorY: field.style.overscrollBehaviorY,
            }
          }
          // Keep the entire active field inside the sheet scrollport. Native
          // textarea scrolling keeps its caret visible without guessing line positions.
          field.style.maxHeight = `${available}px`
          field.style.overflowY = 'auto'
          field.style.overscrollBehaviorY = 'contain'
        }
        const fieldBox = field.getBoundingClientRect()
        if (fieldBox.bottom > bottom) body.scrollTop += fieldBox.bottom - bottom
        else if (fieldBox.top < top) body.scrollTop += fieldBox.top - top
      })
    }
    // The sheet owns its scrollport; scrolling the document would leave a focused
    // field behind the pinned frame when the software keyboard changes its height.
    panel.addEventListener('focusin', revealFocusedField)
    panel.addEventListener('focusout', restoreField)
    revealFocusedField()
    return () => {
      cancelAnimationFrame(frame)
      panel.removeEventListener('focusin', revealFocusedField)
      panel.removeEventListener('focusout', restoreField)
      restoreField()
      overlay.style.top = ''
      overlay.style.bottom = ''
      panel.style.maxHeight = ''
    }
  }, [open, viewport.height, viewport.offsetTop])
  const titleId = `clip-text-sheet-${text.instanceId}`
  // The cut the caption's interval STARTS in: a caption may cross several, and it is placed once,
  // against the frame it opens over (CLIP-143).
  const startCut = captionStartCut(plan, text)
  const frameFingerprint = startCut?.cut.fingerprint
  return (
    <Sheet
      open={open}
      labelledBy={titleId}
      onClose={onClose}
      header={
        <div ref={headerRef}>
          <div className="mb-3 flex items-start justify-between gap-3">
            <Typography variant="fieldTitle" id={titleId} className="min-w-0">
              {t('timeline.textControls')}
            </Typography>
            <Button
              variant="ghost"
              size="icon"
              aria-label={tCommon('action.close')}
              onClick={onClose}
            >
              <X aria-hidden="true" className="size-5" />
            </Button>
          </div>
          {!readOnly && text.role === 'caption' && stage && startCut && (
            <ClipCaptionStage
              text={text}
              fragment={stage.fragment}
              canvas={stage.canvas}
              safeArea={stage.safeArea}
              frameUrl={footage.localSources.find((s) => s.fingerprint === frameFingerprint)?.url}
              frameFingerprint={frameFingerprint}
              frameStartMs={outputToSourceMs(startCut, textInterval(plan, text).startMs)}
              resolvePlayback={footage.resolvePlayback}
              notices={notices}
              language={language}
              change={change}
              disabled={disabled}
              compact
              maxFrameHeight={
                viewport.height *
                (viewport.height < CLIP_TIMELINE.compactViewportHeight
                  ? CLIP_TIMELINE.compactPreviewFraction
                  : CLIP_TIMELINE.previewViewportFraction)
              }
            />
          )}
        </div>
      }
      footer={
        readOnly ? undefined : (
          <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
            <Button
              variant="danger"
              disabled={disabled}
              onClick={() => {
                onClose()
                change({ type: 'removeText', id: text.instanceId })
              }}
            >
              {t('timeline.deleteText')}
            </Button>
          </div>
        )
      }
    >
      {readOnly ? (
        <ClipTextReading plan={plan} text={text} notices={notices} language={language} />
      ) : (
        <fieldset disabled={disabled} className="min-w-0 space-y-4">
          {text.role === 'caption' && stage && startCut && (
            <>
              <Typography variant="meta">{t('placement.mainHelp')}</Typography>
              {stage.fragment?.representativeFrame && (
                <Typography variant="meta">{t('placement.representative')}</Typography>
              )}
              {!stage.fragment && <Typography variant="meta">{t('placement.drawing')}</Typography>}
            </>
          )}
          <ClipTextControls
            plan={plan}
            text={text}
            captionStyles={styles.allowed}
            styleSamples={styles.samples}
            samplesUnavailable={styles.unavailable}
            failure={failure}
            notices={notices}
            language={language}
            change={change}
            regionOverlap={validation?.regionOverlap}
            invalid={!!validation?.invalid}
            issues={validation?.issues}
          />
        </fieldset>
      )}
    </Sheet>
  )
}
