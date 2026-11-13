import {
  useClipLocalCaptionPreview,
  useClipLocalStyleSamples,
  type BrowserCompositionDesign,
} from '@/entities/clip-preview'
import type { ClipRatioId } from '@/entities/clip-design'
import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  outputToSourceMs,
  snapClipTime,
  sourceToOutputMs,
  timelineCuts,
  type ClipEditingState,
  captionRefresh,
  narrationSlot,
  minExposureMs,
  textInterval,
  type ClipEditCut,
  type ClipSelection,
  type ClipSpokenSegment,
} from '@/entities/clip-plan'
import { type ClipDisplayedFrame } from '@/entities/clip-preview'
import { Info, Maximize2, Minimize2 } from 'lucide-react'
import type { AppFailure } from '@/shared/api'
import { CLIP_DRAFT_PREVIEW, CLIP_TIMELINE } from '@/entities/clip-design'
import {
  AppFailureMessage,
  Button,
  Dialog,
  FieldMessage,
  Popover,
  Slider,
  StepBar,
  Typography,
  useVisualViewport,
} from '@/shared/ui'
import type { useClipCorrection } from '../model/useClipCorrection'
import { textExcerpt, textIssues } from '../model/caption-issues'
import { ClipSceneBlocks } from './ClipSceneBlocks'
import { ClipCaptionStage } from './ClipCaptionStage'
import { ClipSceneBlockSheet, type ClipCutSound } from './ClipSceneBlockSheet'
import { ClipCaptionSheet } from './ClipCaptionSheet'
import { ClipSegmentSheet } from './ClipSegmentSheet'

type Correction = ReturnType<typeof useClipCorrection>
export interface ClipEditorPreviewProps {
  timeMs: number
  onTimeChange: (ms: number) => void
  onDisplayedFrame: (frame: ClipDisplayedFrame) => void
  maxHeight: number
  compact: boolean
  suspended: boolean
  stickyTop?: number
  /** ②'s info control, overlaid at the frame's top-right beside the player's own (CLIP-148). */
  corner?: ReactNode
  editingOverlay?: ReactNode
  captionPosition?: { instanceId: string; x: number; y: number }
}
/** ②'s screen. It takes six handles rather than the twenty-two scalars the page used to spread
 *  here (ARCH-14): one for the saved project, one for the draft being corrected, one for the
 *  render it commits, one for the footage it draws, one for the slots its owner fills, and the
 *  one flag that disables the lot. */
export function ClipCorrectionWorkspace({
  project,
  correction,
  render,
  footage,
  slots,
  disabled,
  readOnly = false,
}: {
  /** The SAVED side: what the server holds for this project, including the plan ② edits from. */
  project: {
    id: string
    ownerId?: string
    ratio?: ClipRatioId
    design?: Partial<BrowserCompositionDesign>
    state: ClipEditingState
    /** The caption styles this project allows (CLIP-142). */
    captionStyles?: readonly string[]
    notices?: readonly ClipNotice[]
    language?: 'ko' | 'en'
  }
  /** The DRAFT side: the correction in progress, its validation and its save state. */
  correction: Correction
  /** ② renders nothing (CLIP-149, CLIP-153): a generation's failure is all it reports here. */
  render: {
    failure?: AppFailure
  }
  /** The originals ② draws frames from: the local copies, and the way to reach an unexpired
   *  retained one the session has no local copy of. */
  footage: {
    localSources: ReadonlyArray<{ fingerprint: string; url: string }>
    resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  }
  /** What the owner of this workspace fills in: the page owns the composer and this workspace
   *  owns the render control, and neither feature may import the other (ARCH-13). */
  slots: {
    preview: (props: ClipEditorPreviewProps) => ReactNode
    comparison?: ReactNode
    /** The step bar's body (CLIP-40 →THEME-71): the revision request above the row holding the
     *  way back and 확정하기. The owner composes it, since neither feature may import the other
     *  (ARCH-13); this workspace puts its own failure above it. */
    dock?: ReactNode
    referenceAction?: ReactNode
    tools?: ReactNode
    addDubbing?: () => void
    /** The project's intro and outro slots, the scene list's first and last blocks (CLIP-17). */
    intro?: ReactNode
    outro?: ReactNode
    spokenProperties?: (segment: ClipSpokenSegment) => ReactNode
    /** Opens the observed-scene chooser; a cut chosen there lands after the selected block. */
    addScene?: () => void
    /** The original-sound setting of the source a cut plays from (CLIP-18). */
    sourceSound?: (cut: ClipEditCut) => ClipCutSound | undefined
  }
  disabled: boolean
  /** A finalized project reads its plan here instead of editing it (CLIP-160): the sheets state
   *  values, the dock is gone, and nothing asks for an original that finalization deleted. */
  readOnly?: boolean
}) {
  const { id: projectId, state, captionStyles, notices = [], language } = project
  const { preview, comparison, dock, referenceAction } = slots
  const { failure: renderFailure } = render
  const { t } = useTranslation('clips')
  const [captionPosition, setCaptionPosition] = useState<{
    instanceId: string
    x: number
    y: number
  }>()
  const viewport = useVisualViewport()
  const container = useRef<HTMLElement>(null)
  const previewRoot = useRef<HTMLDivElement>(null)
  const chromeProbe = useRef<HTMLSpanElement>(null)
  const actions = useRef<HTMLDivElement>(null)
  const [expandedPreview, setExpandedPreview] = useState(false)
  const [pinPreview, setPinPreview] = useState(true)
  const [previewBudget, setPreviewBudget] = useState<number>()
  const measureEditingRoom = useCallback(() => {
    const available = window.visualViewport?.height ?? innerHeight
    const offset = window.visualViewport?.offsetTop ?? 0
    const chrome = chromeProbe.current?.getBoundingClientRect().height ?? 0
    const dockTop =
      actions.current?.firstElementChild?.getBoundingClientRect().top ?? available + offset
    const focused = document.activeElement
    const fieldRoom =
      (focused instanceof HTMLInputElement || focused instanceof HTMLTextAreaElement) &&
      container.current?.contains(focused)
        ? focused.getBoundingClientRect().height + CLIP_TIMELINE.fieldGapPx * 2
        : CLIP_TIMELINE.minimumEditingRoomPx
    const budget = Math.min(available, dockTop - offset) - chrome - fieldRoom
    setPreviewBudget(budget > 0 ? budget : undefined)
    setPinPreview(budget > 0)
  }, [])
  const revealFocusedField = useCallback(() => {
    const field = document.activeElement
    if (
      !(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement) ||
      !container.current?.contains(field)
    )
      return
    const gap = CLIP_TIMELINE.fieldGapPx
    const dock = actions.current?.firstElementChild
    // The composer is already inside the dock. The item's fields belong above
    // it, but scrolling this field there would chase the sticky bar forever.
    // On keyboard resize, clear the containing section's top by just enough to
    // bring the entire dock back into the visible viewport.
    if (dock?.contains(field)) {
      const bottom =
        (window.visualViewport?.offsetTop ?? 0) + (window.visualViewport?.height ?? innerHeight)
      const overflow = dock.getBoundingClientRect().bottom - bottom
      if (overflow > 0) window.scrollBy({ top: overflow + gap, behavior: 'instant' })
      return
    }
    const top =
      Math.max(
        (window.visualViewport?.offsetTop ?? 0) +
          (chromeProbe.current?.getBoundingClientRect().height ?? 0),
        previewRoot.current?.querySelector('[data-clip-preview-canvas]')?.getBoundingClientRect()
          .bottom ?? 0,
      ) + gap
    const bottom =
      Math.min(
        (window.visualViewport?.offsetTop ?? 0) + (window.visualViewport?.height ?? innerHeight),
        actions.current?.firstElementChild?.getBoundingClientRect().top ?? innerHeight,
      ) - gap
    const rect = field.getBoundingClientRect()
    if (bottom <= top || (rect.top >= top && rect.bottom <= bottom)) return
    const target = top + Math.max(0, (bottom - top - rect.height) / 2)
    window.scrollBy({ top: rect.top - target, behavior: 'instant' })
  }, [])
  useEffect(() => {
    const frame = requestAnimationFrame(() => {
      measureEditingRoom()
      revealFocusedField()
    })
    return () => cancelAnimationFrame(frame)
  }, [
    viewport.height,
    viewport.offsetTop,
    pinPreview,
    previewBudget,
    revealFocusedField,
    measureEditingRoom,
  ])
  useEffect(() => {
    if (typeof ResizeObserver === 'undefined') return
    let frame = 0
    const observer = new ResizeObserver(() => {
      // Only the frame is pinned. Controls and explanations keep the document
      // scroller, leaving room for the focused field above the committing action.
      measureEditingRoom()
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(revealFocusedField)
    })
    for (const node of [container.current, previewRoot.current, actions.current?.firstElementChild])
      if (node) observer.observe(node)
    return () => {
      observer.disconnect()
      cancelAnimationFrame(frame)
    }
  }, [revealFocusedField, measureEditingRoom])
  const [frame, setFrame] = useState<ClipDisplayedFrame>()
  const [confirm, setConfirm] = useState(false)
  const { timeline, draft, dispatch, change } = correction
  const seek = useCallback((timeMs: number) => dispatch({ type: 'seek', timeMs }), [dispatch])
  // The item whose own sheet was last opened (CLIP-55). It outlives `shown` so the sheet can
  // close with its animation and return focus; selection stays when it closes (CLIP-53), and an
  // item deleted meanwhile has no sheet to show.
  const [sheet, setSheet] = useState<ClipSelection>()
  const [shown, setShown] = useState(false)
  const open = (selection: ClipSelection) => {
    dispatch({ type: 'select', selection })
    setSheet(selection)
    setShown(true)
  }
  const closeSheet = useCallback(() => setShown(false), [])
  const cut = sheet?.kind === 'cut' ? draft.cuts.find((c) => c.id === sheet.id) : undefined
  const text =
    sheet?.kind === 'text'
      ? draft.elements?.find((text) => text.instanceId === sheet.id)
      : undefined
  const spoken =
    sheet?.kind === 'spoken' ? draft.narration?.segments.find((s) => s.id === sheet.id) : undefined
  const index = cut ? draft.cuts.indexOf(cut) : -1
  const source = state.sources.find((s) => s.id === cut?.sourceId)
  const errors = correction.validation?.cuts[index]
  const cutTime = cut ? timelineCuts(draft)[index] : undefined
  const currentFrame =
    cut &&
    frame?.precise &&
    frame.cutId === cut.id &&
    frame.sourceMs >= cut.startMs &&
    frame.sourceMs < cut.endMs &&
    Math.abs(sourceToOutputMs(cutTime!, frame.sourceMs) - timeline.timeMs) <=
      CLIP_DRAFT_PREVIEW.frameToleranceMs &&
    Math.abs(frame.outputMs - timeline.timeMs) <= CLIP_DRAFT_PREVIEW.frameToleranceMs
      ? outputToSourceMs(cutTime!, snapClipTime(frame.outputMs))
      : undefined
  const captionPreview = useClipLocalCaptionPreview(
    project.ownerId && project.ratio
      ? {
          ownerId: project.ownerId,
          projectId,
          projectRevision: correction.revision,
          planRevision: correction.revision,
          plan: draft,
          ratio: project.ratio,
          sources: state.sources,
          layoutObservations: state.layoutObservations,
          design: project.design,
        }
      : undefined,
    !!draft.nativeComposition && !readOnly,
    text?.instanceId,
  )
  const styleSamples = useClipLocalStyleSamples(
    project.ratio ?? 'vertical',
    text?.role === 'caption' && !readOnly,
    project.design?.captionPace,
  )
  const failure = correction.failure ?? renderFailure
  const refresh = captionRefresh(draft)
  const captionSlot = draft.nativeComposition ? narrationSlot(draft, timeline.timeMs) : undefined
  // A 9:16 frame narrower than two touch targets spills its corner controls over
  // the editor. When the available budget cannot hold them, let the document
  // scroll instead of pinning an unusably small frame over the focused field.
  const previewFraction =
    viewport.height < CLIP_TIMELINE.compactViewportHeight
      ? expandedPreview
        ? CLIP_TIMELINE.expandedCompactPreviewFraction
        : CLIP_TIMELINE.compactPreviewFraction
      : expandedPreview
        ? CLIP_TIMELINE.expandedPreviewViewportFraction
        : CLIP_TIMELINE.previewViewportFraction
  const desiredPreviewHeight = viewport.height * previewFraction
  const controlsHeight = expandedPreview
    ? CLIP_TIMELINE.minExpandedPreviewControlsHeightPx
    : CLIP_TIMELINE.minPreviewControlsHeightPx
  const canPinPreview =
    pinPreview &&
    (previewBudget ?? Infinity) >= controlsHeight &&
    desiredPreviewHeight >= controlsHeight
  const previewHeight = canPinPreview
    ? Math.min(previewBudget ?? Infinity, desiredPreviewHeight)
    : Math.max(controlsHeight, desiredPreviewHeight)
  return (
    <section
      ref={container}
      onFocusCapture={() =>
        requestAnimationFrame(() => {
          measureEditingRoom()
          revealFocusedField()
        })
      }
      className="min-w-0 space-y-4"
      aria-label={t('correction.title')}
      onBlur={() => dispatch({ type: 'endTransaction' })}
      onKeyDown={(event) => {
        if (disabled) return
        if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'z') {
          event.preventDefault()
          dispatch({ type: event.shiftKey ? 'redo' : 'undo' })
        }
      }}
    >
      {/* Resolve the shared chrome token in CSS pixels for the same clearance as the sticky frame. */}
      <span
        ref={chromeProbe}
        aria-hidden="true"
        className="pointer-events-none invisible absolute"
        style={{ height: 'var(--spacing-chrome, 0px)' }}
      />
      {/* No heading of its own: the step bar above already names this step, and the preview is
          what the screen is for (owner decision 2026-09-19). The section keeps the name. */}
      {/* The preview stands beside the scene blocks from `lg` and stays in view while they scroll;
          below `lg` its compact frame pins under the top chrome above them (CLIP-53, THEME-44). The
          list keeps the document's one scroller (THEME-25). */}
      <div className="grid min-w-0 gap-4 lg:grid-cols-2 lg:items-start">
        <div className="lg:top-chrome min-w-0 space-y-4 lg:sticky">
          <div ref={previewRoot} className="contents">
            {preview({
              captionPosition,
              editingOverlay: !readOnly && captionPreview.data && (
                <>
                  {(draft.elements ?? [])
                    .filter(
                      (text) =>
                        text.role === 'caption' &&
                        textInterval(draft, text).startMs <= timeline.timeMs &&
                        timeline.timeMs < textInterval(draft, text).endMs,
                    )
                    .map((text) => (
                      <ClipCaptionStage
                        key={text.instanceId}
                        overlayOnly
                        text={text}
                        fragment={captionPreview.data?.captions.find(
                          (c) => c.instanceId === text.instanceId,
                        )}
                        canvas={captionPreview.data!.canvas}
                        safeArea={captionPreview.data!.safeArea}
                        frameStartMs={0}
                        selected={
                          timeline.selection?.kind === 'text' &&
                          timeline.selection.id === text.instanceId
                        }
                        onSelect={() =>
                          dispatch({
                            type: 'select',
                            seek: false,
                            selection: { kind: 'text', id: text.instanceId },
                          })
                        }
                        onPositionPreview={setCaptionPosition}
                        change={change}
                        disabled={disabled}
                      />
                    ))}
                </>
              ),
              timeMs: timeline.timeMs,
              onTimeChange: seek,
              onDisplayedFrame: setFrame,
              suspended: false,
              maxHeight: previewHeight,
              compact: true,
              stickyTop: canPinPreview ? viewport.offsetTop : undefined,
              // ONE info control holding what the draft preview cannot promise about the delivered
              // file plus every notice that names no cut and no caption (CLIP-148), on the frame it
              // is about — a video player's corner, not a row of the page.
              corner: (
                <>
                  <Button
                    variant="scrim"
                    size="icon"
                    aria-label={t(expandedPreview ? 'previewSize.compact' : 'previewSize.expand')}
                    aria-pressed={expandedPreview}
                    onClick={() => setExpandedPreview((value) => !value)}
                  >
                    {expandedPreview ? (
                      <Minimize2 aria-hidden="true" className="size-5" />
                    ) : (
                      <Maximize2 aria-hidden="true" className="size-5" />
                    )}
                  </Button>
                  <Popover
                    label={t('preview.aboutLabel')}
                    triggerSize="icon"
                    triggerVariant="scrim"
                    triggerLabel={<Info aria-hidden="true" className="size-5" />}
                    placement="below"
                    align="end"
                    phone="sheet"
                  >
                    {() => (
                      <div className="space-y-2">
                        <Typography variant="body" className="text-content-secondary">
                          {t('preview.parity')}
                        </Typography>
                        {/* The flow view is still frames, never the delivered render (CLIP-176). */}
                        <Typography variant="body" className="text-content-secondary">
                          {t('preview.flowParity')}
                        </Typography>
                        {/* The preview reports the precision of the frame it is showing, so
                      this says the position is approximate only while it is. */}
                        {frame && !frame.precise && (
                          <Typography variant="body" className="text-content-secondary">
                            {t('preview.frameApproximate')}
                          </Typography>
                        )}
                        <ClipNoticeList
                          notices={notices.filter((n) => !n.cutId && !n.elementId)}
                          language={language}
                        />
                      </div>
                    )}
                  </Popover>
                </>
              ),
            })}
          </div>
          {/* Directly under the preview, and nothing else about the clip stands in the flow ② edits
          in (CLIP-148): the source sheet and the tools. ② renders and downloads nothing; both
          stand in ③ (CLIP-149, CLIP-153). `mt-4` by hand: the preview above renders through
          `display: contents` wrappers so its frame can be pinned, and a box-less child takes no
          share of the section's `space-y`. */}
          {(referenceAction || slots.tools) && (
            <div className="mt-4 flex flex-wrap items-center gap-2">
              {referenceAction}
              {slots.tools}
            </div>
          )}
          {/* The scrubber states output time on one row under the frame (CLIP-53). */}
          {!readOnly && (
            <Slider
              ariaLabel={t('preview.outputTime')}
              min={0}
              max={Math.max(1, Number.isFinite(draft.durationMs) ? draft.durationMs : 1)}
              step={CLIP_DRAFT_PREVIEW.frameToleranceMs}
              value={timeline.timeMs}
              valueText={`${clipSeconds(timeline.timeMs)} / ${clipSeconds(draft.durationMs)} s`}
              onChange={(ms) => seek(snapClipTime(ms))}
            />
          )}
        </div>
        <div className="min-w-0 space-y-4">
          {/* The save state is NOT reported here: CLIP-38 gives it to the page's one status
          region. Undo/redo head the list they act on. */}
          <ClipSceneBlocks
            plan={correction.transientPlan ?? draft}
            sources={state.sources}
            selection={timeline.selection}
            timeMs={timeline.timeMs}
            onSelect={(selection) => dispatch({ type: 'select', selection })}
            onOpen={open}
            onMove={correction.moveCut}
            change={change}
            validation={correction.validation}
            footage={footage}
            intro={slots.intro}
            outro={slots.outro}
            notices={notices}
            readOnly={readOnly}
            disabled={disabled}
            history={{
              undo: () => dispatch({ type: 'undo' }),
              redo: () => dispatch({ type: 'redo' }),
              canUndo: !readOnly && !!timeline.past.length,
              canRedo: !readOnly && !!timeline.future.length,
              disabled: disabled || readOnly,
            }}
          />
          {!readOnly && (
            <div className="flex flex-wrap items-center gap-2">
              {/* A new cut starts from an observed scene and lands after the selected block
                  (CLIP-55, CLIP-67). */}
              {slots.addScene && (
                <Button variant="secondary" disabled={disabled} onClick={slots.addScene}>
                  {t('sceneBlocks.addScene')}
                </Button>
              )}
              <Button
                variant="secondary"
                disabled={disabled || !captionSlot}
                onClick={() => {
                  if (!captionSlot) return
                  // A local identity until the save returns the server-minted one.
                  const id = `new-caption-${Date.now()}`
                  change({ type: 'addNarration', id, ...captionSlot })
                  open({ kind: 'text', id })
                }}
              >
                {t('timeline.addCaption')}
              </Button>
              <Button
                variant="secondary"
                disabled={disabled || !slots.addDubbing}
                onClick={slots.addDubbing}
              >
                {t('timeline.addDubbing')}
              </Button>
              {!captionSlot && (
                <Typography variant="meta">{t('timeline.captionUnavailable')}</Typography>
              )}
            </div>
          )}
          {!readOnly && draft.narration && (
            <div className="space-y-2">
              <Button
                variant="secondary"
                disabled={disabled || correction.pending}
                onClick={() => {
                  void correction.refreshCaptions().catch(() => undefined)
                }}
              >
                {t('placement.refresh')}
              </Button>
              {refresh.pending.map((segment, index) => {
                const candidate = narrationSlot(
                  draft,
                  segment.startMs,
                  Math.max(0, segment.endMs - segment.startMs),
                )
                const slot =
                  candidate &&
                  Math.min(candidate.endMs, segment.endMs) - candidate.startMs >= minExposureMs('')
                    ? {
                        startMs: candidate.startMs,
                        endMs: Math.min(candidate.endMs, segment.endMs),
                      }
                    : undefined
                return (
                  <div key={segment.id} className="flex flex-wrap items-center gap-2">
                    <Button
                      variant="secondary"
                      disabled={disabled || !slot || segment.creation}
                      onClick={() => {
                        if (!slot) return
                        const id = `new-script-caption-${Date.now()}`
                        change({ type: 'addScriptCaption', id, segmentId: segment.id, ...slot })
                        open({ kind: 'text', id })
                      }}
                    >
                      {t('placement.addFromScript', { number: index + 1 })}
                    </Button>
                    {!slot && <Typography variant="meta">{t('placement.noRoom')}</Typography>}
                  </div>
                )
              })}
              {refresh.unplaceable.length > 0 && (
                <Typography variant="meta">{t('placement.unplaceableWords')}</Typography>
              )}
              {refresh.orphaned.length > 0 && (
                <Typography variant="meta">{t('placement.orphans')}</Typography>
              )}
            </div>
          )}
          <ClipNoticeList
            notices={notices.filter(
              (n) =>
                (n.cutId && !draft.cuts.some((c) => c.id === n.cutId)) ||
                (n.elementId &&
                  !draft.elements?.some((e) => e.elementId === n.elementId && e.cutId === n.cutId)),
            )}
            language={language}
            cuts={draft.cuts}
            withTargets
          />
          {!correction.validation?.valid && (
            <FieldMessage>
              {t(
                correction.validation?.speechReady === false
                  ? 'dubbing.deliveryPending'
                  : 'timeline.invalid',
              )}
            </FieldMessage>
          )}
          {correction.validation?.cuts.map((error, number) => {
            const reason = error.rate
              ? 'assembly.cadenceRefused'
              : error.overlap
                ? 'assembly.overlap'
                : error.start || error.end || error.duration || error.identity
                  ? 'assembly.invalidRange'
                  : ('evidence' in error && error.evidence) ||
                      failure?.params.cut_id === draft.cuts[number].id
                    ? 'assembly.invalidCreation'
                    : undefined
            return reason ? (
              <Button
                key={draft.cuts[number].id}
                variant="ghost"
                onClick={() =>
                  dispatch({
                    type: 'select',
                    selection: { kind: 'cut', id: draft.cuts[number].id },
                  })
                }
              >
                {t('assembly.cutIssue', { number: number + 1, reason: t(reason) })}
              </Button>
            ) : null
          })}
          {/* Each text that holds the save names itself and what is wrong, and opens its own
              sheet to fix it (CDS-213): the generic line above never says which one it is. */}
          {correction.validation?.elements.map((error) => {
            const reasons = textIssues(error)
            const issueText = draft.elements?.find((e) => e.instanceId === error.id)
            if (!reasons.length || !issueText) return null
            const interval = textInterval(draft, issueText)
            const label =
              issueText.role === 'hook'
                ? t('sceneBlocks.intro')
                : issueText.role === 'ending'
                  ? t('sceneBlocks.outro')
                  : issueText.role === 'badge' || issueText.role === 'disclosure'
                    ? t('sceneBlocks.badge')
                    : issueText.role === 'caption'
                      ? t('sceneBlocks.caption', {
                          start: clipSeconds(interval.startMs),
                          end: clipSeconds(interval.endMs),
                        })
                      : t('assembly.text')
            const excerpt = textExcerpt(issueText)
            const reason = reasons.map((issue) => t(`textIssues.${issue}`)).join(' ')
            return (
              <Button
                key={error.id}
                variant="ghost"
                className="text-left"
                onClick={() => open({ kind: 'text', id: issueText.instanceId })}
              >
                {excerpt
                  ? t('assembly.textIssue', { label, excerpt, reason })
                  : t('assembly.textIssueBare', { label, reason })}
              </Button>
            )
          })}
          {correction.validation?.timeline && (
            <FieldMessage>
              {t('correction.timelineError', {
                min: state.minDurationMs,
                max: state.maxDurationMs,
              })}
            </FieldMessage>
          )}
          {comparison}
        </div>
      </div>
      <div ref={actions} className="contents">
        {!readOnly && (
          <StepBar ariaLabel={t('correction.actions')}>
            {failure && (
              <div role="alert" className="min-w-0 space-y-2">
                <AppFailureMessage failure={failure} />
                {failure.reason === 'CLIP_PLAN_CONFLICT' && (
                  <div className="flex flex-wrap gap-2">
                    <Button
                      variant="ghost"
                      disabled={correction.pending}
                      onClick={() => setConfirm(true)}
                    >
                      {t('correction.reload')}
                    </Button>
                    <Button
                      variant="secondary"
                      disabled={correction.pending}
                      onClick={correction.reapply}
                    >
                      {t('timeline.reapply')}
                    </Button>
                  </div>
                )}
              </div>
            )}
            {dock}
          </StepBar>
        )}
      </div>
      {/* Each item opens its OWN sheet over the preview and the list, which stay on screen behind
          it, so every change shows in the preview while the sheet stays open (CLIP-55). */}
      {cut && (
        <ClipSceneBlockSheet
          open={shown}
          onClose={closeSheet}
          plan={draft}
          cut={cut}
          source={source}
          playheadMs={timeline.timeMs}
          currentFrame={currentFrame}
          errors={errors}
          sound={slots.sourceSound?.(cut)}
          notices={notices}
          language={language}
          footage={footage}
          readOnly={readOnly}
          disabled={disabled}
          change={change}
          seek={seek}
          endTransaction={() => dispatch({ type: 'endTransaction' })}
          split={(sourceMs) => correction.splitCut(cut.id, sourceMs)}
        />
      )}
      {text && (
        <ClipCaptionSheet
          open={shown}
          onClose={closeSheet}
          plan={draft}
          text={text}
          stage={
            captionPreview.data && {
              canvas: captionPreview.data.canvas,
              safeArea: captionPreview.data.safeArea,
              fragment: captionPreview.data.captions.find((c) => c.instanceId === text.instanceId),
            }
          }
          styles={{
            allowed: captionStyles,
            samples: styleSamples.data?.captions,
            unavailable: styleSamples.isError,
          }}
          footage={footage}
          validation={{
            regionOverlap: correction.validation?.elements.some(
              (e) => e.id === text.instanceId && e.regionOverlap,
            ),
            invalid: !!correction.validation?.elements.some(
              (e) =>
                e.id === text.instanceId &&
                Object.entries(e).some(([key, value]) => key !== 'id' && value),
            ),
            issues: textIssues(
              correction.validation?.elements.find((e) => e.id === text.instanceId),
            ),
          }}
          failure={correction.failure}
          notices={notices}
          language={language}
          readOnly={readOnly}
          disabled={disabled}
          change={change}
        />
      )}
      {spoken && draft.narration && (
        <ClipSegmentSheet
          open={shown}
          onClose={closeSheet}
          narration={draft.narration}
          segment={spoken}
          durationMs={draft.durationMs}
        >
          {readOnly ? undefined : slots.spokenProperties?.(spoken)}
        </ClipSegmentSheet>
      )}
      <Dialog
        open={confirm}
        title={t('correction.leaveTitle')}
        confirmLabel={t('correction.reload')}
        onClose={() => setConfirm(false)}
        onConfirm={() => {
          correction.reload()
          setConfirm(false)
        }}
      >
        {t('correction.leaveBody')}
      </Dialog>
    </section>
  )
}
