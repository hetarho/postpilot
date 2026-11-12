import { useTranslation } from 'react-i18next'
import { X } from 'lucide-react'
import { ClipNoticeList, type ClipNotice } from '@/entities/clip-project'
import {
  clipSeconds,
  clipSourceSound,
  cutOutputMs,
  outputToSourceMs,
  snapClipTime,
  timelineCuts,
  type ClipEditPlan,
  type ClipEditCut,
  type RetainedClipSource,
  type TimelineEdit,
} from '@/entities/clip-plan'
import { CLIP_DRAFT_PREVIEW } from '@/entities/clip-design'
import {
  Button,
  FieldLabel,
  Listbox,
  RangeSlider,
  Sheet,
  Slider,
  Switch,
  Textarea,
  Typography,
} from '@/shared/ui'
import { ClipTimeField } from './ClipTimeField'
import { ClipCutSourceFrame } from './ClipCutSourceFrame'
import { ClipCutAssemblyControls } from './ClipCutAssemblyControls'
import { ClipCutReading } from './ClipItemReading'

/** One source's original-sound setting as ② may change it (CLIP-18): absent `change` reads it. */
export interface ClipCutSound {
  value: boolean
  change?: (enabled: boolean) => void
}

/** A scene block's own sheet (CLIP-55): trimming the cut's source range with precise source-time
 *  entry, splitting at the preview's playhead, the fixed rate its source admits and the source's
 *  sound. Every change lands in the draft the preview draws, so the sheet stays open over it. */
export function ClipSceneBlockSheet({
  open,
  onClose,
  plan,
  cut,
  source,
  playheadMs,
  currentFrame,
  errors,
  sound,
  notices = [],
  language,
  footage,
  readOnly = false,
  disabled = false,
  change,
  seek,
  endTransaction,
  split,
}: {
  open: boolean
  onClose: () => void
  plan: ClipEditPlan
  cut: ClipEditCut
  source?: Pick<RetainedClipSource, 'filename' | 'durationMs' | 'allowedRatePermille'>
  /** The preview's output time. */
  playheadMs: number
  /** The source time of the precise frame the preview shows inside this cut, if it shows one. */
  currentFrame?: number
  errors?: { start?: boolean; end?: boolean }
  sound?: ClipCutSound
  notices?: readonly ClipNotice[]
  language?: 'ko' | 'en'
  footage: {
    localSources: ReadonlyArray<{ fingerprint: string; url: string }>
    resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  }
  readOnly?: boolean
  disabled?: boolean
  change: (edit: TimelineEdit, group?: string) => void
  seek: (outputMs: number) => void
  endTransaction: () => void
  split: (sourceMs: number) => Promise<void>
}) {
  const { t } = useTranslation('clips')
  const { t: tCommon } = useTranslation('common')
  const index = plan.cuts.findIndex((c) => c.id === cut.id)
  const cuts = timelineCuts(plan)
  const cutTime = cuts[index]
  const titleId = `clip-cut-sheet-${cut.id}`
  const active = cuts.filter((c) => c.startMs <= playheadMs && playheadMs < c.endMs)
  const splitAt =
    active.length === 1 && active[0].cut.id === cut.id
      ? outputToSourceMs(cutTime, snapClipTime(playheadMs))
      : NaN
  const soundOn = sound?.value ?? clipSourceSound(plan, cut)
  return (
    <Sheet
      open={open}
      labelledBy={titleId}
      onClose={onClose}
      header={
        <div className="mb-3 flex items-start justify-between gap-3">
          <Typography variant="fieldTitle" id={titleId} className="min-w-0 break-words">
            {`${t('correction.cut', { number: index + 1 })} · ${source?.filename ?? cut.sourceId}`}
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
            {/* Deleting the cut is also closing its sheet: the selection would otherwise fall to a
                neighbour the owner never opened. */}
            <Button
              variant="danger"
              disabled={disabled}
              onClick={() => {
                onClose()
                change({ type: 'remove', id: cut.id })
              }}
            >
              {t('correction.deleteCut')}
            </Button>
          </div>
        )
      }
    >
      {readOnly ? (
        <ClipCutReading
          plan={plan}
          cut={cut}
          filename={source?.filename}
          notices={notices}
          language={language}
        />
      ) : (
        <fieldset disabled={disabled} className="min-w-0 space-y-4">
          {/* The frame this range is trimmed against: the owner's own SOURCE at the cut's start,
              because trimming is source-time work (CLIP-53, CLIP-67). */}
          <ClipCutSourceFrame
            cut={cut}
            localSources={footage.localSources}
            resolvePlayback={footage.resolvePlayback}
          />
          <Typography variant="meta">
            {t('timeline.outputRange', {
              start: clipSeconds(cutTime.startMs),
              end: clipSeconds(cutTime.endMs),
            })}
          </Typography>
          <RangeSlider
            disabled={!!cut.creation}
            startLabel={t('timeline.trimStart')}
            endLabel={t('timeline.trimEnd')}
            value={[cut.startMs, cut.endMs]}
            min={0}
            max={source?.durationMs ?? cut.endMs}
            step={CLIP_DRAFT_PREVIEW.frameToleranceMs}
            format={(ms) => `${clipSeconds(ms)} s`}
            onCommit={endTransaction}
            onChange={([start, end]) => {
              const startMs = start === cut.startMs ? start : snapClipTime(start)
              const endMs = end === cut.endMs ? end : snapClipTime(end)
              change({ type: 'cut', id: cut.id, patch: { startMs, endMs } }, `trim-${cut.id}`)
              seek(
                cutTime.startMs +
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
                seek(cutTime.startMs)
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
                  cutTime.startMs +
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
          <ClipCutAssemblyControls
            key={cut.id}
            cut={cut}
            allowedRates={source?.allowedRatePermille ?? []}
            playheadMs={splitAt}
            native={!!plan.nativeComposition}
            onChange={change}
            onSplit={split}
          />
          {/* One setting per SOURCE (CLIP-18): every cut of this source plays or drops its sound. */}
          <div className="flex min-w-0 items-start gap-3">
            <Switch
              id={`clip-cut-sound-${cut.id}`}
              checked={soundOn}
              disabled={!sound?.change}
              aria-describedby={`clip-cut-sound-${cut.id}-help`}
              onChange={(event) => sound?.change?.(event.target.checked)}
            />
            <div className="min-w-0 space-y-1">
              <FieldLabel htmlFor={`clip-cut-sound-${cut.id}`}>
                {t('sceneBlocks.sourceSound')}
              </FieldLabel>
              <Typography variant="meta" as="p" id={`clip-cut-sound-${cut.id}-help`}>
                {t('sceneBlocks.sourceSoundHelp')}
              </Typography>
            </div>
          </div>
          <div>
            <FieldLabel id={`clip-cut-transition-${cut.id}`}>
              {t('correction.transition')}
            </FieldLabel>
            <Listbox
              aria-labelledby={`clip-cut-transition-${cut.id}`}
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
                change({ type: 'cut', id: cut.id, patch: { volumePermille } }, `volume-${cut.id}`)
              }
            />
          </fieldset>
          {!plan.nativeComposition &&
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
          <ClipNoticeList
            notices={notices.filter((n) => n.cutId === cut.id && !n.elementId)}
            language={language}
          />
        </fieldset>
      )}
    </Sheet>
  )
}
