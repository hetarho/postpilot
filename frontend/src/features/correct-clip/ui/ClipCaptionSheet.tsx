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
import { Button, Sheet, Typography } from '@/shared/ui'
import { ClipCaptionStage } from './ClipCaptionStage'
import { ClipTextControls } from './ClipTextControls'
import { ClipTextReading } from './ClipItemReading'

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
  validation?: { invalid: boolean; regionOverlap?: boolean }
  failure?: AppFailure
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  readOnly?: boolean
  disabled?: boolean
  change: (edit: TimelineEdit, group?: string) => void
}) {
  const { t } = useTranslation('clips')
  const { t: tCommon } = useTranslation('common')
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
              />
              <Typography variant="meta">{t('placement.mainHelp')}</Typography>
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
          />
        </fieldset>
      )}
    </Sheet>
  )
}
