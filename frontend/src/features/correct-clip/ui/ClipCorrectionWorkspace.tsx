import { type ClipBrowserRenderCapability } from '@/entities/clip-preview'
import type { MyPlan } from '@/entities/plan'
import { ClipNoticeList, type ClipNotice, type ClipRenderKind } from '@/entities/clip-project'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  outputToSourceMs,
  snapClipTime,
  sourceToOutputMs,
  timelineCuts,
  type ClipEditingState,
  captionStartCut,
  type ClipSpokenSegment,
  useClipCaptionPreview,
  useClipCaptionStyleSamples,
} from '@/entities/clip-plan'
import { type ClipDisplayedFrame } from '@/entities/clip-preview'
import { Info, X } from 'lucide-react'
import type { AppFailure } from '@/shared/api'
import { CLIP_DRAFT_PREVIEW, CLIP_TIMELINE } from '@/entities/clip-design'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Dialog,
  FieldMessage,
  Popover,
  Sheet,
  Slider,
  Typography,
  useVisualViewport,
  useMediaQuery,
  MD_MEDIA_QUERY,
} from '@/shared/ui'
import { splitAtOutput } from '../model/timeline-gesture'
import type { useClipCorrection } from '../model/useClipCorrection'
import { ClipRenderAction } from './ClipRenderAction'
import { ClipTimeline } from './ClipTimeline'
import { ClipItemProperties } from './ClipItemProperties'

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
    state: ClipEditingState
    /** The caption styles this project allows (CLIP-142). */
    captionStyles?: readonly string[]
    notices?: readonly ClipNotice[]
    language?: 'ko' | 'en'
  }
  /** The DRAFT side: the correction in progress, its validation and its save state. */
  correction: Correction
  /** The render this step commits, and what the controls may say about it. */
  render: {
    ready: boolean
    pending: boolean
    failure?: AppFailure
    /** Handed over only while there is a browser render to report on. */
    progress?: ReactNode
    lastKind?: ClipRenderKind
    current?: boolean
    capability?: ClipBrowserRenderCapability
    serverWindow?: MyPlan['serverExportWindow']
    serverPlan?: MyPlan['plan']
    start: (kind: ClipRenderKind) => void
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
    /** The dock's composer — the revision request, or its active run — handed the step's actions
     *  (렌더하기 and, once a render exists, 확정하기) to carry at the right of its heading (CLIP-40). */
    revision?: (actions: ReactNode) => ReactNode
    /** Downloading the identified latest successful render, as an icon directly under the video
     *  it downloads (CLIP-149): the dock holds committing controls and a download commits
     *  nothing (CLIP-40, THEME-39). Absent while the plan has no render, and its absence is
     *  silence. */
    downloadAction?: ReactNode
    /** ②'s primary committing control, the one thing of the confirmation the dock carries.
     *  Absent until the project has a render: until then the render IS the step's next action,
     *  and a disabled 확정하기 beside it only said so in smaller type (CLIP-40). */
    finalizeAction?: ReactNode
    referenceAction?: ReactNode
    tools?: ReactNode
    addDubbing?: () => void
    spokenProperties?: (segment: ClipSpokenSegment) => ReactNode
  }
  disabled: boolean
  /** A finalized project reads its plan here instead of editing it (CLIP-160): the sheets state
   *  values, the dock is gone, and nothing asks for an original that finalization deleted. */
  readOnly?: boolean
}) {
  const { id: projectId, state, captionStyles, notices = [], language } = project
  const { localSources } = footage
  const { preview, comparison, revision, downloadAction, finalizeAction, referenceAction } = slots
  const {
    ready: renderReady,
    pending: renderPending,
    failure: renderFailure,
    progress: renderProgress,
    lastKind: lastRenderKind,
    current: currentRender,
    capability: browserCapability,
    start: onRender,
  } = render
  const { t } = useTranslation('clips')
  const { t: tCommon } = useTranslation('common')
  const viewport = useVisualViewport()
  const container = useRef<HTMLElement>(null)
  const previewRoot = useRef<HTMLDivElement>(null)
  const actions = useRef<HTMLDivElement>(null)
  const [pinPreview, setPinPreview] = useState(true)
  const [previewBudget, setPreviewBudget] = useState<number>()
  const measureEditingRoom = useCallback(() => {
    const available = window.visualViewport?.height ?? innerHeight
    const offset = window.visualViewport?.offsetTop ?? 0
    const dockTop =
      actions.current?.firstElementChild?.getBoundingClientRect().top ?? available + offset
    const focused = document.activeElement
    const fieldRoom =
      (focused instanceof HTMLInputElement || focused instanceof HTMLTextAreaElement) &&
      container.current?.contains(focused)
        ? focused.getBoundingClientRect().height + CLIP_TIMELINE.fieldGapPx * 2
        : CLIP_TIMELINE.minimumEditingRoomPx
    const budget = Math.min(available, dockTop - offset) - fieldRoom
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
        window.visualViewport?.offsetTop ?? 0,
        (field.closest('[data-clip-item-properties]')
          ? 0
          : previewRoot.current
              ?.querySelector('[data-clip-preview-canvas]')
              ?.getBoundingClientRect().bottom) ?? 0,
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
  const cut =
    timeline.selection?.kind === 'cut'
      ? draft.cuts.find((c) => c.id === timeline.selection?.id)
      : undefined
  const text =
    timeline.selection?.kind === 'text'
      ? draft.elements?.find((text) => text.instanceId === timeline.selection?.id)
      : undefined
  const desktop = useMediaQuery(MD_MEDIA_QUERY)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const spoken =
    timeline.selection?.kind === 'spoken'
      ? draft.narration?.segments.find((s) => s.id === timeline.selection?.id)
      : undefined
  const selected = !!cut || !!text || !!spoken
  const sheetOpen = !desktop && detailsOpen && selected
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
  // ② draws each caption from the SERVER's own fragment (CDS-83). The query is
  // keyed by what the captions draw, so selecting one or dragging it asks
  // nothing: only their words, styles, sizes and pacing do.
  const captionPreview = useClipCaptionPreview(
    projectId,
    correction.revision,
    draft,
    text?.role === 'caption' && !readOnly,
  )
  const fragment = captionPreview.data?.captions.find((c) => c.instanceId === text?.instanceId)
  // Every approved style, drawn once by the renderer for the caption sheet's picker (CDS-83);
  // the same session-long query ① reads, retried only while the renderer is busy.
  const styleSamples = useClipCaptionStyleSamples(projectId, text?.role === 'caption' && !readOnly)
  // The cut the caption's interval STARTS in: a caption may cross several, and
  // it is placed once, against the frame it opens over (CLIP-143).
  const captionCut = text ? captionStartCut(draft, text) : undefined
  const failure = correction.failure ?? renderFailure
  const itemTitle = cut
    ? `${t('correction.cut', { number: index + 1 })} · ${source?.filename ?? cut.sourceId}`
    : spoken
      ? t('timeline.spokenSegment', {
          number: (draft.narration?.segments.indexOf(spoken) ?? 0) + 1,
        })
      : t('timeline.textControls')
  const itemProperties = spoken ? (
    (slots.spokenProperties?.(spoken) ?? (
      <div className="min-w-0 space-y-2">
        <Typography variant="body">{spoken.text}</Typography>
        <Typography variant="meta">
          {clipSeconds(spoken.startMs)}–{clipSeconds(spoken.endMs)} s
        </Typography>
      </div>
    ))
  ) : (
    <ClipItemProperties
      project={{ state, captionStyles, notices, language }}
      correction={correction}
      footage={footage}
      selection={{ cut, text, index, source, errors, cutTime, currentFrame, captionCut }}
      captions={{ captionPreview, fragment, styleSamples }}
      status={{ readOnly, disabled }}
    />
  )
  const itemActions = readOnly ? undefined : (
    <div className="mt-3 flex flex-wrap items-center justify-end gap-2">
      {/* Deleting the selected item is also closing its sheet: leaving the
                selection behind would have reopened it on the neighbour the
                reducer falls back to. */}
      {cut && (
        <Button
          variant="danger"
          disabled={disabled}
          onClick={() => {
            change({ type: 'remove', id: cut.id })
            dispatch({ type: 'select' })
          }}
        >
          {t('correction.deleteCut')}
        </Button>
      )}
      {text && (
        <Button
          variant="danger"
          disabled={disabled}
          onClick={() => {
            change({ type: 'removeText', id: text.instanceId })
            dispatch({ type: 'select' })
          }}
        >
          {t('timeline.deleteText')}
        </Button>
      )}
    </div>
  )
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
      {/* No heading of its own: the step bar above already names this step, and the preview is
          what the screen is for (owner decision 2026-09-19). The section keeps the name. */}
      <div className="grid min-w-0 gap-4 md:grid-cols-3">
        <div className="min-w-0 space-y-4 md:col-span-2">
          <div ref={previewRoot} className="contents">
            {preview({
              timeMs: timeline.timeMs,
              onTimeChange: seek,
              onDisplayedFrame: setFrame,
              suspended: false,
              maxHeight: Math.min(
                previewBudget ?? Infinity,
                viewport.height *
                  (viewport.height < CLIP_TIMELINE.compactViewportHeight
                    ? CLIP_TIMELINE.compactPreviewFraction
                    : CLIP_TIMELINE.previewViewportFraction),
              ),
              compact: true,
              stickyTop: pinPreview ? viewport.offsetTop : undefined,
              // ONE info control holding what the draft preview cannot promise about the delivered
              // file plus every notice that names no cut and no caption (CLIP-148), on the frame it
              // is about — a video player's corner, not a row of the page.
              corner: (
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
              ),
            })}
          </div>
          {/* Directly under the preview, and nothing else about the clip stands in the
          flow ② edits in (CLIP-148): the download of the render this plan already
          has (CLIP-149) and the source sheet. With a plan and no render there is simply no
          download — a plan awaiting one is ②'s FIRST state (CLIP-56), not a
          missing result. */}
          {/* `mt-4` by hand: the preview above renders through `display: contents` wrappers so its
          frame can be pinned, and a box-less child takes no share of the section's `space-y`. */}
          <div className="mt-4 flex flex-wrap items-center gap-2">
            {downloadAction}
            {referenceAction}
            {slots.tools}
          </div>
          {/* The save state is NOT reported here: CLIP-38 gives it to the page's one
          status region, and a second copy beside the timeline said it twice with
          two different delays. Undo/redo moved to the timeline's own head. */}
          <ClipTimeline
            plan={correction.transientPlan ?? draft}
            onSeek={seek}
            onCommit={change}
            onPreview={correction.setTransientPlan}
            bounds={correction.timelineBounds}
            canSplit={
              timeline.selection?.kind === 'cut' &&
              !!splitAtOutput(draft, timeline.selection.id, timeline.timeMs, 'probe')
            }
            onSplit={
              timeline.selection?.kind === 'cut'
                ? () => {
                    const selected = timelineCuts(draft).find(
                      (c) => c.cut.id === timeline.selection!.id,
                    )
                    if (selected)
                      void correction
                        .splitCut(selected.cut.id, outputToSourceMs(selected, timeline.timeMs))
                        .catch(() => undefined)
                  }
                : undefined
            }
            selection={timeline.selection}
            timeMs={timeline.timeMs}
            history={{
              undo: () => dispatch({ type: 'undo' }),
              redo: () => dispatch({ type: 'redo' }),
              canUndo: !readOnly && !!timeline.past.length,
              canRedo: !readOnly && !!timeline.future.length,
              disabled: disabled || readOnly,
            }}
            onSelect={(selection) => {
              setDetailsOpen(false)
              dispatch({ type: 'select', selection })
            }}
            onAddDubbing={slots.addDubbing}
            readOnly={readOnly}
            disabled={disabled}
            onAddCaption={
              readOnly
                ? undefined
                : (slot) => {
                    // A local identity until the save returns the server-minted one.
                    const id = `new-caption-${Date.now()}`
                    setDetailsOpen(true)
                    change({ type: 'addNarration', id, ...slot })
                    dispatch({ type: 'select', selection: { kind: 'text', id } })
                  }
            }
            localSources={localSources}
            notices={notices}
          />
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
          {!correction.validation?.valid && <FieldMessage>{t('timeline.invalid')}</FieldMessage>}
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
          {correction.validation?.timeline && (
            <FieldMessage>
              {t('correction.timelineError', {
                min: state.minDurationMs,
                max: state.maxDurationMs,
              })}
            </FieldMessage>
          )}
          {comparison}
          {!desktop && selected && (
            <div
              className="flex min-w-0 flex-wrap items-center gap-2"
              aria-label={t('timeline.contextTools')}
            >
              <Typography variant="meta" className="min-w-0 flex-1 truncate">
                {itemTitle}
              </Typography>
              <Button variant="secondary" onClick={() => setDetailsOpen(true)}>
                {t('timeline.details')}
              </Button>
            </div>
          )}
        </div>
        {desktop && (
          <aside
            className="min-w-0 space-y-4"
            aria-label={t('timeline.properties')}
            data-clip-item-properties
          >
            <Typography variant="fieldTitle">
              {selected ? itemTitle : t('timeline.properties')}
            </Typography>
            {selected ? (
              itemProperties
            ) : (
              <Typography variant="body" className="text-content-secondary">
                {t('timeline.selectItem')}
              </Typography>
            )}
            {itemActions}
          </aside>
        )}
      </div>
      <div ref={actions} className="contents">
        {!readOnly && (
          <ActionBar ariaLabel={t('correction.actions')} className="space-y-3">
            {/* Only while there is something to report: an empty first row would still push the
              composer down by the bar's own row gap. */}
            {(renderProgress || failure) && (
              <div className="space-y-2">
                {renderProgress}
                {failure && (
                  <div role="alert" className="mb-3 space-y-2">
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
              </div>
            )}
            {/* The post editor's dock (CLIP-40 →POST-45): the composer's heading row carries the
              step's actions at its right — 렌더하기, the step's own action until the project has
              a render, then secondary beside the primary 확정하기 — and its field and send
              control sit under them. Nothing else is a row of this bar. */}
            {(() => {
              const actions = (
                <>
                  <ClipRenderAction
                    lastKind={lastRenderKind}
                    currentRender={currentRender}
                    browserAvailable={browserCapability?.available ?? false}
                    serverWindow={render.serverWindow}
                    serverPlan={render.serverPlan}
                    browserRefusal={
                      browserCapability && !browserCapability.available
                        ? browserCapability.reason
                        : undefined
                    }
                    pending={renderPending}
                    disabled={
                      disabled ||
                      correction.pending ||
                      !renderReady ||
                      !correction.validation?.valid
                    }
                    variant={finalizeAction ? 'secondary' : 'cta'}
                    onRender={onRender}
                  />
                  {finalizeAction}
                </>
              )
              return revision ? (
                revision(actions)
              ) : (
                <div className="flex flex-wrap items-center justify-end gap-2">{actions}</div>
              )
            })()}
          </ActionBar>
        )}
      </div>
      {/* ONE selection is ONE sheet (CLIP-53): the selected item's own controls
          open over the preview and the timeline, which stay on screen behind
          them, so what is being changed is visible whichever of the two is
          showing. `open` is derived from the selection and never from state of
          its own, which is what makes the timeline's pressed state and this
          sheet the same fact — and closing it clears the selection.
          The body is the sheet's one scroller, so the destructive control in
          its pinned footer stays reachable at 360px with the keyboard open. */}
      <Sheet
        open={sheetOpen}
        labelledBy="clip-item-sheet-title"
        onClose={() => setDetailsOpen(false)}
        header={
          <div className="mb-3 flex items-start justify-between gap-3">
            <Typography variant="fieldTitle" id="clip-item-sheet-title" className="min-w-0">
              {itemTitle}
            </Typography>
            <Button
              variant="ghost"
              size="icon"
              aria-label={tCommon('action.close')}
              onClick={() => setDetailsOpen(false)}
            >
              <X aria-hidden="true" className="size-5" />
            </Button>
          </div>
        }
        footer={itemActions}
      >
        {itemProperties}
      </Sheet>
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
