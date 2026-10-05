import { useTranslation } from 'react-i18next'
import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import {
  clipSeconds,
  cutOutputMs,
  outputToSourceMs,
  snapClipTime,
  timelineCuts,
  captionStartCut,
  useClipCaptionPreview,
  useClipCaptionStyleSamples,
  type ClipEditingState,
  type ClipEditCut,
  type ClipEditableText,
  type RetainedClipSource,
} from '@/entities/clip-plan'
import { CLIP_DRAFT_PREVIEW } from '@/entities/clip-design'
import { Button, FieldLabel, Listbox, RangeSlider, Slider, Textarea, Typography } from '@/shared/ui'
import type { useClipCorrection } from '../model/useClipCorrection'
import { ClipTimeField } from './ClipTimeField'
import { ClipTextControls } from './ClipTextControls'
import { ClipCutSourceFrame } from './ClipCutSourceFrame'
import { ClipCutAssemblyControls } from './ClipCutAssemblyControls'
import { ClipCutReading, ClipTextReading } from './ClipItemReading'

type Correction = ReturnType<typeof useClipCorrection>
type CaptionQuery = ReturnType<typeof useClipCaptionPreview>
export interface ClipItemPropertiesProps {
  project: {
    state: ClipEditingState
    captionStyles?: readonly string[]
    notices: readonly ClipNotice[]
    language?: 'ko' | 'en'
  }
  correction: Correction
  footage: {
    localSources: ReadonlyArray<{ fingerprint: string; url: string }>
    resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  }
  selection: {
    cut?: ClipEditCut
    text?: ClipEditableText
    index: number
    source?: RetainedClipSource
    errors?: NonNullable<Correction['validation']>['cuts'][number]
    cutTime?: ReturnType<typeof timelineCuts>[number]
    currentFrame?: number
    captionCut?: ReturnType<typeof captionStartCut>
  }
  captions: {
    captionPreview: CaptionQuery
    fragment?: NonNullable<CaptionQuery['data']>['captions'][number]
    styleSamples: ReturnType<typeof useClipCaptionStyleSamples>
  }
  status: { readOnly: boolean; disabled: boolean }
}

/** One properties body: mounted beside the timeline on desktop, or inside the
 * explicitly opened phone sheet. Save/history/playback stay owned above it. */
export function ClipItemProperties({
  project,
  correction,
  footage,
  selection,
  captions,
  status,
}: ClipItemPropertiesProps) {
  const { t } = useTranslation('clips')
  const { captionStyles, notices, language } = project
  const { localSources, resolvePlayback } = footage
  const { cut, text, index, source, errors, cutTime, currentFrame } = selection
  const { styleSamples } = captions
  const { readOnly, disabled } = status
  const { draft, timeline, dispatch, change } = correction
  const seek = (timeMs: number) => dispatch({ type: 'seek', timeMs })
  return (
    <>
      {readOnly ? (
        <div className="min-w-0 space-y-4">
          {cut && (
            <ClipCutReading
              plan={draft}
              cut={cut}
              filename={source?.filename}
              notices={notices}
              language={language}
            />
          )}
          {text && (
            <ClipTextReading plan={draft} text={text} notices={notices} language={language} />
          )}
        </div>
      ) : (
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
                playheadMs={
                  timelineCuts(draft).filter(
                    (c) => c.startMs <= timeline.timeMs && timeline.timeMs < c.endMs,
                  ).length === 1
                    ? outputToSourceMs(cutTime!, snapClipTime(timeline.timeMs))
                    : NaN
                }
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
          {text?.role === 'caption' && (
            <Typography variant="meta">{t('placement.mainHelp')}</Typography>
          )}
          {text && (
            <ClipTextControls
              plan={draft}
              text={text}
              captionStyles={captionStyles}
              styleSamples={styleSamples.data?.captions}
              samplesUnavailable={styleSamples.isError}
              failure={correction.failure}
              notices={notices}
              language={language}
              change={change}
              regionOverlap={correction.validation?.elements.some(
                (e) => e.id === text.instanceId && e.regionOverlap,
              )}
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
      )}
    </>
  )
}
