import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  snapClipTime,
  textInterval,
  timelineCuts,
  useClipCaptionPreview,
  cutOutputMs,
  outputToSourceMs,
  sourceToOutputMs,
  type ClipDisplayedFrame,
  type ClipEditingState,
} from '@/entities/clip-project'
import { Info, X } from 'lucide-react'
import type { AppFailure } from '@/shared/api'
import { CLIP_DRAFT_PREVIEW, CLIP_TIMELINE } from '@/shared/config'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  Popover,
  RangeSlider,
  Sheet,
  Slider,
  Textarea,
  Typography,
  useVisualViewport,
} from '@/shared/ui'
import type { useClipCorrection } from '../model/useClipCorrection'
import { ClipTimeline } from './ClipTimeline'
import { ClipTimeField } from './ClipTimeField'
import { ClipTextControls } from './ClipTextControls'
import { ClipCaptionStage } from './ClipCaptionStage'
import { ClipCutSourceFrame } from './ClipCutSourceFrame'
import { ClipCutAssemblyControls } from './ClipCutAssemblyControls'

type Correction = ReturnType<typeof useClipCorrection>
export interface ClipEditorPreviewProps {
  timeMs: number
  onTimeChange: (ms: number) => void
  onDisplayedFrame: (frame: ClipDisplayedFrame) => void
  maxHeight: number
  compact: boolean
  stickyTop?: number
}
export function ClipCorrectionWorkspace({
  projectId,
  captionStyles,
  correction,
  state,
  disabled,
  renderReady,
  renderPending,
  renderFailure,
  onRender,
  sourcePicker,
  preview,
  localSources,
  resolvePlayback,
  comparison,
  revision,
  downloadAction,
  finalizeAction,
  finalizeNotices,
  notices = [],
  language,
}: {
  projectId: string
  /** The caption styles this project allows (CLIP-142). */
  captionStyles?: readonly string[]
  correction: Correction
  state: ClipEditingState
  disabled: boolean
  renderReady: boolean
  renderPending: boolean
  renderFailure?: AppFailure
  onRender: () => void
  sourcePicker: ReactNode
  preview: (props: ClipEditorPreviewProps) => ReactNode
  comparison?: ReactNode
  /** The owner's written revision request and its approval (CLIP-131). It sits in
   *  the PANEL: ②'s dock is full, and a charged action would not belong beside
   *  three credit-free ones in any case (CLIP-40). */
  revision?: ReactNode
  /** Downloading the identified latest successful render, as an icon directly
   *  under the video it downloads (CLIP-149): the dock is one row of committing
   *  controls and a download commits nothing (CLIP-40, THEME-39). Absent while
   *  the plan has no render, and its absence is silence. */
  downloadAction?: ReactNode
  /** ②'s primary committing control, the one thing of the confirmation the dock
   *  carries. */
  finalizeAction?: ReactNode
  /** What confirming does, what the clip delivered and why it is refused. */
  finalizeNotices?: ReactNode
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  /** Resolves an unexpired retained original for a source the session has no
   *  local copy of, so ② can still show the frame a caption sits on. */
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
}) {
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
    const top =
      Math.max(
        window.visualViewport?.offsetTop ?? 0,
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
  const cut =
    timeline.selection?.kind === 'cut'
      ? draft.cuts.find((c) => c.id === timeline.selection?.id)
      : undefined
  const text =
    timeline.selection?.kind === 'text'
      ? draft.elements?.find((text) => text.instanceId === timeline.selection?.id)
      : undefined
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
    text?.role === 'caption',
  )
  const fragment = captionPreview.data?.captions.find((c) => c.instanceId === text?.instanceId)
  // The cut the caption's interval STARTS in: a caption may cross several, and
  // it is placed once, against the frame it opens over (CLIP-143).
  const captionInterval = text ? textInterval(draft, text) : undefined
  const captionCut = captionInterval
    ? (timelineCuts(draft).find(
        (c) => c.startMs <= captionInterval.startMs && captionInterval.startMs < c.endMs,
      ) ?? timelineCuts(draft)[0])
    : undefined
  const failure = correction.failure ?? renderFailure
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
      <Typography variant="title">{t('correction.title')}</Typography>
      <div ref={previewRoot} className="contents">
        {preview({
          timeMs: timeline.timeMs,
          onTimeChange: seek,
          onDisplayedFrame: setFrame,
          maxHeight: Math.min(
            previewBudget ?? Infinity,
            viewport.height *
              (viewport.height < CLIP_TIMELINE.compactViewportHeight
                ? CLIP_TIMELINE.compactPreviewFraction
                : CLIP_TIMELINE.previewViewportFraction),
          ),
          compact: true,
          stickyTop: pinPreview ? viewport.offsetTop : undefined,
        })}
      </div>
      {/* Directly under the preview, and nothing else about the clip stands in the
          flow ② edits in (CLIP-148): the download of the render this plan already
          has (CLIP-149), and one info control holding what the draft preview
          cannot promise about the delivered file plus every notice that names no
          cut and no caption. With a plan and no render there is simply no
          download — a plan awaiting one is ②'s FIRST state (CLIP-56), not a
          missing result. */}
      <div className="flex flex-wrap items-center gap-2">
        {downloadAction}
        <Popover
          label={t('preview.aboutLabel')}
          triggerSize="icon"
          triggerVariant="secondary"
          triggerLabel={<Info aria-hidden="true" className="size-5" />}
          placement="below"
          align="start"
          phone="sheet"
        >
          {() => (
            <div className="space-y-2">
              <Typography variant="body" className="text-content-secondary">
                {t('preview.parity')}
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
      </div>
      {/* The save state is NOT reported here: CLIP-38 gives it to the page's one
          status region, and a second copy beside the timeline said it twice with
          two different delays. Undo/redo moved to the timeline's own head. */}
      <ClipTimeline
        plan={draft}
        selection={timeline.selection}
        timeMs={timeline.timeMs}
        history={{
          undo: () => dispatch({ type: 'undo' }),
          redo: () => dispatch({ type: 'redo' }),
          canUndo: !!timeline.past.length,
          canRedo: !!timeline.future.length,
          disabled,
        }}
        onSelect={(selection) => dispatch({ type: 'select', selection })}
        onAddCaption={(slot) => {
          // A local identity until the save returns the server-minted one.
          const id = `new-caption-${Date.now()}`
          change({ type: 'addNarration', id, ...slot })
          dispatch({ type: 'select', selection: { kind: 'text', id } })
        }}
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
      <Slider
        ariaLabel={t('preview.outputTime')}
        min={0}
        max={Math.max(1, Number.isFinite(draft.durationMs) ? draft.durationMs : 1)}
        step={CLIP_DRAFT_PREVIEW.frameToleranceMs}
        value={timeline.timeMs}
        valueText={`${clipSeconds(timeline.timeMs)} / ${clipSeconds(draft.durationMs)} s`}
        onChange={(ms) => seek(snapClipTime(ms))}
      />
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
              dispatch({ type: 'select', selection: { kind: 'cut', id: draft.cuts[number].id } })
            }
          >
            {t('assembly.cutIssue', { number: number + 1, reason: t(reason) })}
          </Button>
        ) : null
      })}
      {correction.validation?.timeline && (
        <FieldMessage>
          {t('correction.timelineError', { min: state.minDurationMs, max: state.maxDurationMs })}
        </FieldMessage>
      )}
      {revision}
      {comparison}
      {sourcePicker}
      {/* The download moved under the video it downloads (CLIP-149), so what is
          left here is the confirmation's own copy — which T248 takes into the
          finalization dialog. */}
      {finalizeNotices && (
        <section className="mt-10 space-y-3" aria-label={t('finalization.summaryLabel')}>
          {finalizeNotices}
        </section>
      )}
      <div ref={actions} className="contents">
        <ActionBar ariaLabel={t('correction.actions')}>
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
          {/* ONE row: 다시 렌더 and 확정하기 stand side by side, with 저장 joining
              them while the draft is dirty. Everything the confirmation says sits
              in the panel above, so the dock cannot grow over the editor. */}
          <div className="flex flex-wrap items-center justify-end gap-2">
            {/* No 저장: the draft autosaves as ①'s settings do, and 다시 렌더 flushes the
                queue itself, so a dirty draft is not a reason to refuse it (CLIP-39). An
                INVALID draft still is — it is the one thing autosave cannot take. */}
            <Button
              variant="secondary"
              pending={renderPending}
              disabled={
                disabled || correction.pending || !renderReady || !correction.validation?.valid
              }
              onClick={onRender}
            >
              {t('timeline.render')}
            </Button>
            {finalizeAction}
          </div>
        </ActionBar>
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
        open={!!cut || !!text}
        labelledBy="clip-item-sheet-title"
        onClose={() => dispatch({ type: 'select' })}
        header={
          <div className="mb-3 flex items-start justify-between gap-3">
            <Typography variant="fieldTitle" id="clip-item-sheet-title" className="min-w-0">
              {cut
                ? `${t('correction.cut', { number: index + 1 })} · ${source?.filename ?? cut.sourceId}`
                : t('timeline.textControls')}
            </Typography>
            <Button
              variant="ghost"
              size="icon"
              aria-label={tCommon('action.close')}
              onClick={() => dispatch({ type: 'select' })}
            >
              <X aria-hidden="true" className="size-5" />
            </Button>
          </div>
        }
        footer={
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
        }
      >
        <fieldset disabled={disabled} className="min-w-0 space-y-4">
          {cut && (
            <div className="space-y-4">
              {/* The frame this range is trimmed against: the owner's own SOURCE at
                the cut's start, not the composed output, because trimming is
                source-time work (CLIP-53, CLIP-67). */}
              <ClipCutSourceFrame
                cut={cut}
                localSources={localSources}
                resolvePlayback={resolvePlayback}
              />
              <Typography variant="meta">
                {t('timeline.outputRange', {
                  start: clipSeconds(cutTime!.startMs),
                  end: clipSeconds(cutTime!.endMs),
                })}
              </Typography>
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="secondary"
                  disabled={index <= 0}
                  onClick={() => change({ type: 'move', from: index, to: index - 1 })}
                >
                  {t('editor.up')}
                </Button>
                <Button
                  variant="secondary"
                  disabled={index === draft.cuts.length - 1}
                  onClick={() => change({ type: 'move', from: index, to: index + 1 })}
                >
                  {t('editor.down')}
                </Button>
              </div>
              <ClipCutAssemblyControls
                key={cut.id}
                cut={cut}
                allowedRates={source?.allowedRatePermille ?? []}
                playheadMs={outputToSourceMs(cutTime!, snapClipTime(timeline.timeMs))}
                native={!!draft.nativeComposition}
                onChange={change}
                onSplit={(sourceMs) => correction.splitCut(cut.id, sourceMs)}
              />
              <RangeSlider
                disabled={!!cut.creation}
                startLabel={t('timeline.trimStart')}
                endLabel={t('timeline.trimEnd')}
                value={[cut.startMs, cut.endMs]}
                min={0}
                max={source?.durationMs ?? cut.endMs}
                step={CLIP_DRAFT_PREVIEW.frameToleranceMs}
                format={(ms) => `${clipSeconds(ms)} s`}
                onCommit={() => dispatch({ type: 'endTransaction' })}
                onChange={([start, end]) => {
                  const startMs = start === cut.startMs ? start : snapClipTime(start)
                  const endMs = end === cut.endMs ? end : snapClipTime(end)
                  change({ type: 'cut', id: cut.id, patch: { startMs, endMs } }, `trim-${cut.id}`)
                  seek(
                    cutTime!.startMs +
                      (start !== cut.startMs
                        ? 0
                        : Math.max(
                            0,
                            cutOutputMs({ ...cut, startMs, endMs }) -
                              CLIP_DRAFT_PREVIEW.frameToleranceMs,
                          )),
                  )
                }}
              />
              <div className="grid grid-cols-2 gap-3">
                <ClipTimeField
                  id={`clip-cut-${cut.id}-start`}
                  disabled={!!cut.creation}
                  label={t('timeline.sourceStart')}
                  value={cut.startMs}
                  error={errors?.start ? t('timeline.rangeInvalid') : undefined}
                  onChange={(startMs) =>
                    change({ type: 'cut', id: cut.id, patch: { startMs } }, `start-${cut.id}`)
                  }
                />
                <ClipTimeField
                  id={`clip-cut-${cut.id}-end`}
                  disabled={!!cut.creation}
                  label={t('timeline.sourceEnd')}
                  value={cut.endMs}
                  error={errors?.end ? t('timeline.rangeInvalid') : undefined}
                  onChange={(endMs) =>
                    change({ type: 'cut', id: cut.id, patch: { endMs } }, `end-${cut.id}`)
                  }
                />
              </div>
              <div className="flex flex-wrap gap-2">
                <Button
                  variant="secondary"
                  disabled={currentFrame === undefined || !!cut.creation}
                  onClick={() => {
                    change({ type: 'cut', id: cut.id, patch: { startMs: currentFrame! } })
                    seek(cutTime!.startMs)
                  }}
                >
                  {t('timeline.frameStart')}
                </Button>
                <Button
                  variant="secondary"
                  disabled={currentFrame === undefined || !!cut.creation}
                  onClick={() => {
                    change({ type: 'cut', id: cut.id, patch: { endMs: currentFrame! } })
                    seek(
                      cutTime!.startMs +
                        Math.max(
                          0,
                          cutOutputMs({ ...cut, endMs: currentFrame! }) -
                            CLIP_DRAFT_PREVIEW.frameToleranceMs,
                        ),
                    )
                  }}
                >
                  {t('timeline.frameEnd')}
                </Button>
              </div>
              {currentFrame === undefined && (
                <Typography variant="meta">{t('timeline.frameWaiting')}</Typography>
              )}
              <div>
                <FieldLabel id="clip-cut-transition-label">{t('correction.transition')}</FieldLabel>
                <Listbox
                  aria-labelledby="clip-cut-transition-label"
                  value={String(cut.transitionMs)}
                  disabled={index === 0 || !!cut.creation}
                  options={[
                    { value: '0', label: t('correction.transitions.cut') },
                    { value: '200', label: t('correction.transitions.fade', { ms: 200 }) },
                    { value: '300', label: t('timeline.fadeBlack') },
                  ]}
                  onChange={(value) =>
                    change({ type: 'cut', id: cut.id, patch: { transitionMs: Number(value) } })
                  }
                />
              </div>
              <fieldset disabled={!!cut.creation}>
                <Slider
                  label={t('correction.volume')}
                  min={0}
                  max={1000}
                  step={1}
                  value={cut.volumePermille}
                  valueText={`${cut.volumePermille / 10}%`}
                  onChange={(volumePermille) =>
                    change(
                      { type: 'cut', id: cut.id, patch: { volumePermille } },
                      `volume-${cut.id}`,
                    )
                  }
                />
              </fieldset>
              {!draft.nativeComposition &&
                cut.copies.map((copy, i) => (
                  <div key={i} className="space-y-2">
                    <FieldLabel htmlFor={`clip-copy-${i}`}>{t('correction.copy')}</FieldLabel>
                    <Textarea
                      id={`clip-copy-${i}`}
                      autoGrow
                      value={copy.text}
                      onChange={(e) =>
                        change(
                          { type: 'copy', id: cut.id, index: i, patch: { text: e.target.value } },
                          `copy-${cut.id}-${i}`,
                        )
                      }
                    />
                    <div className="grid grid-cols-2 gap-3">
                      <ClipTimeField
                        id={`clip-copy-${i}-start`}
                        label={t('timeline.phraseStart')}
                        value={copy.startMs}
                        onChange={(startMs) =>
                          change({ type: 'copy', id: cut.id, index: i, patch: { startMs } })
                        }
                      />
                      <ClipTimeField
                        id={`clip-copy-${i}-end`}
                        label={t('timeline.phraseEnd')}
                        value={copy.endMs}
                        onChange={(endMs) =>
                          change({ type: 'copy', id: cut.id, index: i, patch: { endMs } })
                        }
                      />
                    </div>
                  </div>
                ))}
              {draft.nativeComposition && (
                <div className="flex flex-wrap gap-2">
                  {draft.elements
                    ?.filter((text) => text.cutId === cut.id)
                    .map((text) => (
                      <Button
                        key={text.instanceId}
                        variant="secondary"
                        onClick={() =>
                          dispatch({
                            type: 'select',
                            selection: { kind: 'text', id: text.instanceId },
                          })
                        }
                      >
                        {text.text || text.elementId}
                      </Button>
                    ))}
                </div>
              )}
            </div>
          )}
          {cut && (
            <ClipNoticeList
              notices={notices.filter((n) => n.cutId === cut.id && !n.elementId)}
              language={language}
            />
          )}
          {text?.cutId && (
            <div className="flex flex-wrap items-center gap-2">
              <Button
                variant="secondary"
                disabled={disabled || draft.cuts.findIndex((c) => c.id === text.cutId) <= 0}
                onClick={() => {
                  const from = draft.cuts.findIndex((c) => c.id === text.cutId)
                  change({ type: 'move', from, to: from - 1 })
                }}
              >
                {t('editor.up')}
              </Button>
              <Button
                variant="secondary"
                disabled={
                  disabled ||
                  draft.cuts.findIndex((c) => c.id === text.cutId) < 0 ||
                  draft.cuts.findIndex((c) => c.id === text.cutId) === draft.cuts.length - 1
                }
                onClick={() => {
                  const from = draft.cuts.findIndex((c) => c.id === text.cutId)
                  change({ type: 'move', from, to: from + 1 })
                }}
              >
                {t('editor.down')}
              </Button>
            </div>
          )}
          {text?.role === 'caption' && captionPreview.data && (
            <ClipCaptionStage
              text={text}
              fragment={fragment}
              canvas={captionPreview.data.canvas}
              safeArea={captionPreview.data.safeArea}
              frameUrl={
                localSources.find((s) => s.fingerprint === captionCut?.cut.fingerprint)?.url
              }
              frameFingerprint={captionCut?.cut.fingerprint}
              frameStartMs={captionCut?.cut.startMs ?? 0}
              resolvePlayback={resolvePlayback}
              notices={notices}
              language={language}
              change={change}
              disabled={disabled}
            />
          )}
          {text && (
            <ClipTextControls
              plan={draft}
              text={text}
              captionStyles={captionStyles}
              notices={notices}
              language={language}
              change={change}
              invalid={
                !!correction.validation?.elements.some(
                  (e) =>
                    e.id === text.instanceId &&
                    Object.entries(e).some(([key, value]) => key !== 'id' && value),
                )
              }
            />
          )}
        </fieldset>
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
