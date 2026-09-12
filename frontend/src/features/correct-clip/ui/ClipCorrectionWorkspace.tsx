import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  clipSeconds,
  snapClipTime,
  timelineCuts,
  type ClipDisplayedFrame,
  type ClipEditingState,
  type ClipCompositionInputs,
  type ClipObservations,
} from '@/entities/clip-project'
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
  RangeSlider,
  Slider,
  Textarea,
  Typography,
  useVisualViewport,
} from '@/shared/ui'
import type { useClipCorrection } from '../model/useClipCorrection'
import { ClipTimeline } from './ClipTimeline'
import { ClipTimeField } from './ClipTimeField'
import { ClipTextControls } from './ClipTextControls'
import { ClipAssociationControls } from './ClipAssociationControls'

type Correction = ReturnType<typeof useClipCorrection>
export interface ClipEditorPreviewProps {
  timeMs: number
  onTimeChange: (ms: number) => void
  onDisplayedFrame: (frame: ClipDisplayedFrame) => void
  maxHeight: number
  compact: boolean
}
export function ClipCorrectionWorkspace({
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
  comparison,
  downloadAction,
  finalizeAction,
  inputs,
  observations,
}: {
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
  downloadAction?: ReactNode
  finalizeAction?: ReactNode
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  inputs?: ClipCompositionInputs
  observations?: ClipObservations
}) {
  const { t } = useTranslation('clips')
  const viewport = useVisualViewport()
  const container = useRef<HTMLElement>(null)
  const previewRoot = useRef<HTMLDivElement>(null)
  const actions = useRef<HTMLDivElement>(null)
  const [pinPreview, setPinPreview] = useState(true)
  const measureEditingRoom = useCallback(() => {
    const available = window.visualViewport?.height ?? innerHeight
    const previewHeight = previewRoot.current?.getBoundingClientRect().height ?? 0
    const dockHeight = actions.current?.firstElementChild?.getBoundingClientRect().height ?? 0
    setPinPreview(available - previewHeight - dockHeight >= CLIP_TIMELINE.minimumEditingRoomPx)
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
        previewRoot.current?.getBoundingClientRect().bottom ?? 0,
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
  }, [viewport.height, viewport.offsetTop, pinPreview, revealFocusedField, measureEditingRoom])
  useEffect(() => {
    if (typeof ResizeObserver === 'undefined') return
    let frame = 0
    const observer = new ResizeObserver(() => {
      // An expired-source explanation or the confirmation notice can occupy more space than
      // the preview itself. Let that preview scroll when pinning it would hide every field.
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
    Math.abs(frame.sourceMs - (cut.startMs + timeline.timeMs - cutTime!.startMs)) <=
      CLIP_DRAFT_PREVIEW.frameToleranceMs &&
    Math.abs(frame.outputMs - timeline.timeMs) <= CLIP_DRAFT_PREVIEW.frameToleranceMs
      ? frame.sourceMs
      : undefined
  const failure = correction.failure ?? renderFailure
  return (
    <section
      ref={container}
      onFocusCapture={() => requestAnimationFrame(revealFocusedField)}
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
      <div
        ref={previewRoot}
        className={`bg-surface-lowest z-10 space-y-2 py-2 ${pinPreview ? 'sticky' : ''}`}
        style={{ top: viewport.offsetTop }}
      >
        {preview({
          timeMs: timeline.timeMs,
          onTimeChange: seek,
          onDisplayedFrame: setFrame,
          maxHeight:
            viewport.height *
            (viewport.height < CLIP_TIMELINE.compactViewportHeight
              ? CLIP_TIMELINE.compactPreviewFraction
              : CLIP_TIMELINE.previewViewportFraction),
          compact: true,
        })}
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <Button
          variant="secondary"
          disabled={disabled || !timeline.past.length}
          onClick={() => dispatch({ type: 'undo' })}
        >
          {t('timeline.undo')}
        </Button>
        <Button
          variant="secondary"
          disabled={disabled || !timeline.future.length}
          onClick={() => dispatch({ type: 'redo' })}
        >
          {t('timeline.redo')}
        </Button>
        <Typography variant="meta" role="status">
          {t(
            correction.saving
              ? 'timeline.saving'
              : correction.dirty
                ? 'timeline.unsaved'
                : 'timeline.saved',
          )}
        </Typography>
        {text?.cutId && (
          <>
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
          </>
        )}
      </div>
      <ClipTimeline
        plan={draft}
        selection={timeline.selection}
        timeMs={timeline.timeMs}
        onSelect={(selection) => dispatch({ type: 'select', selection })}
        localSources={localSources}
      />
      <Slider
        label={t('preview.outputTime')}
        min={0}
        max={Math.max(1, Number.isFinite(draft.durationMs) ? draft.durationMs : 1)}
        step={CLIP_DRAFT_PREVIEW.frameToleranceMs}
        value={timeline.timeMs}
        valueText={`${clipSeconds(timeline.timeMs)} / ${clipSeconds(draft.durationMs)} s`}
        onChange={(ms) => seek(snapClipTime(ms))}
      />
      {!correction.validation?.valid && <FieldMessage>{t('timeline.invalid')}</FieldMessage>}
      {correction.validation?.timeline && (
        <FieldMessage>
          {t('correction.timelineError', { min: state.minDurationMs, max: state.maxDurationMs })}
        </FieldMessage>
      )}
      <fieldset disabled={disabled} className="min-w-0 space-y-4">
        {cut && (
          <div className="space-y-4" aria-label={t('correction.cut', { number: index + 1 })}>
            <Typography variant="fieldTitle">
              {t('correction.cut', { number: index + 1 })} · {source?.filename ?? cut.sourceId}
            </Typography>
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
              <Button variant="danger" onClick={() => change({ type: 'remove', id: cut.id })}>
                {t('correction.deleteCut')}
              </Button>
            </div>
            <RangeSlider
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
                      : Math.max(0, endMs - startMs - CLIP_DRAFT_PREVIEW.frameToleranceMs)),
                )
              }}
            />
            <div className="grid grid-cols-2 gap-3">
              <ClipTimeField
                id={`clip-cut-${cut.id}-start`}
                label={t('timeline.sourceStart')}
                value={cut.startMs}
                error={errors?.start ? t('timeline.rangeInvalid') : undefined}
                onChange={(startMs) =>
                  change({ type: 'cut', id: cut.id, patch: { startMs } }, `start-${cut.id}`)
                }
              />
              <ClipTimeField
                id={`clip-cut-${cut.id}-end`}
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
                disabled={currentFrame === undefined}
                onClick={() => {
                  change({ type: 'cut', id: cut.id, patch: { startMs: currentFrame! } })
                  seek(cutTime!.startMs)
                }}
              >
                {t('timeline.frameStart')}
              </Button>
              <Button
                variant="secondary"
                disabled={currentFrame === undefined}
                onClick={() => {
                  change({ type: 'cut', id: cut.id, patch: { endMs: currentFrame! } })
                  seek(
                    cutTime!.startMs +
                      Math.max(
                        0,
                        currentFrame! - cut.startMs - CLIP_DRAFT_PREVIEW.frameToleranceMs,
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
                disabled={index === 0}
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
            <Slider
              label={t('correction.volume')}
              min={0}
              max={1000}
              step={1}
              value={cut.volumePermille}
              valueText={`${cut.volumePermille / 10}%`}
              onChange={(volumePermille) =>
                change({ type: 'cut', id: cut.id, patch: { volumePermille } }, `volume-${cut.id}`)
              }
            />
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
            {inputs && (
              <ClipAssociationControls
                inputs={inputs}
                associations={draft.associations ?? []}
                observations={observations}
                sourceId={cut.sourceId}
                change={change}
              />
            )}
          </div>
        )}
        {text && (
          <ClipTextControls
            plan={draft}
            text={text}
            styles={state.copyStyles}
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
      {comparison}
      {sourcePicker}
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
          <div className="flex flex-wrap justify-end gap-2">
            {correction.dirty && (
              <Button
                variant="secondary"
                pending={correction.pending}
                disabled={disabled || !correction.validation?.saveable}
                onClick={() => void correction.save()}
              >
                {t('timeline.save')}
              </Button>
            )}
            {downloadAction}
            <Button
              variant="secondary"
              pending={renderPending}
              disabled={
                disabled ||
                correction.pending ||
                correction.dirty ||
                !renderReady ||
                !correction.validation?.valid
              }
              onClick={onRender}
            >
              {t('timeline.render')}
            </Button>
          </div>
          {finalizeAction}
        </ActionBar>
      </div>
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
