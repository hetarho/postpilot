import { useCallback, useEffect, useMemo, useRef, useState, useSyncExternalStore } from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useTranslation } from 'react-i18next'
import { CLIP_DRAFT_PREVIEW, CLIP_DESIGN, type ClipRatioId } from '@/shared/config'
import { appFailureFromConnect, normalizeAppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Checkbox, Slider, Typography } from '@/shared/ui'
import { clipPreviewRequest } from '../api/preview'
import type { ClipEditPlan, RetainedClipSource } from '../model/edit-plan'
import {
  previewCrop,
  previewElementIDs,
  previewFrame,
  previewMotion,
  previewTimeline,
  type PreviewCut,
  type PreviewSourceAccess,
} from '../model/draft-preview'
import { PreviewAssetCache, PreviewPreparation } from '../model/preview-assets'

export interface ClipDisplayedFrame {
  cutId: string
  sourceMs: number
  outputMs: number
  precise: boolean
}

function PreviewVideo({
  item,
  source,
  access,
  timeMs,
  playing,
  muted,
  opacity,
  audioGain,
  master,
  canvas,
  onFrame,
  onPrecision,
  onDisplayedFrame,
}: {
  item: PreviewCut
  source?: RetainedClipSource
  access: PreviewSourceAccess['resolvePlayback']
  timeMs: number
  playing: boolean
  muted: boolean
  opacity: number
  audioGain: number
  master: boolean
  canvas: { width: number; height: number }
  onFrame: (ms: number) => void
  onPrecision: (precise: boolean) => void
  onDisplayedFrame?: (frame: ClipDisplayedFrame) => void
}) {
  const { t } = useTranslation('clips')
  const video = useRef<HTMLVideoElement>(null)
  const [media, setMedia] = useState<{ fingerprint: string; url?: string; error?: string }>({
    fingerprint: '',
  })
  const refreshing = useRef(false)
  const playbackEpoch = useRef(0)
  const fp = item.cut.fingerprint
  const sourceMs = Math.max(
    item.cut.startMs,
    Math.min(
      item.cut.endMs - CLIP_DRAFT_PREVIEW.frameToleranceMs,
      item.cut.startMs + timeMs - item.startMs,
    ),
  )
  const url = media.fingerprint === fp ? media.url : undefined
  const error = media.fingerprint === fp ? media.error : undefined
  const crop = previewCrop(
    source?.width || canvas.width,
    source?.height || canvas.height,
    canvas.width,
    canvas.height,
    item.cut.focal,
  )
  useEffect(() => {
    let active = true
    const epoch = ++playbackEpoch.current
    refreshing.current = false
    void access(fp).then(
      (url) => {
        if (active) setMedia({ fingerprint: fp, url })
      },
      (error: unknown) => {
        if (active) setMedia({ fingerprint: fp, error: appFailureFromConnect(error).reason })
      },
    )
    return () => {
      active = false
      playbackEpoch.current = epoch + 1
    }
  }, [fp, access])

  useEffect(() => {
    const el = video.current
    if (!el || !url) return
    const sync = () => {
      if (
        el.readyState &&
        (!playing ||
          Math.abs(el.currentTime * 1000 - sourceMs) > CLIP_DRAFT_PREVIEW.frameToleranceMs * 2)
      )
        el.currentTime = sourceMs / 1000
      el.volume = Number.isFinite(item.cut.volumePermille)
        ? Math.max(0, Math.min(1, (item.cut.volumePermille / 1000) * audioGain))
        : 0
    }
    sync()
    el.addEventListener('loadedmetadata', sync)
    return () => {
      el.removeEventListener('loadedmetadata', sync)
    }
  }, [url, sourceMs, playing, item.cut.volumePermille, audioGain])

  useEffect(() => {
    const el = video.current
    if (!el || !url) return
    if (playing)
      void el.play().catch(() => {
        setMedia({ fingerprint: fp, error: 'playback' })
      })
    else el.pause()
    return () => {
      el.pause()
    }
  }, [playing, url, fp])

  useEffect(() => {
    const el = video.current
    if (!el || !url || !master) return
    let active = true,
      handle = 0
    const precise = typeof el.requestVideoFrameCallback === 'function'
    const frame = (mediaMs: number) => {
      if (!active) return
      if (!el.seeking)
        onDisplayedFrame?.({
          cutId: item.cut.id,
          sourceMs: Math.round(mediaMs),
          outputMs: item.startMs + mediaMs - item.cut.startMs,
          precise,
        })
      onPrecision(
        precise &&
          Math.abs(mediaMs - (playing ? el.currentTime * 1000 : sourceMs)) <=
            CLIP_DRAFT_PREVIEW.frameToleranceMs,
      )
      if (playing && !el.seeking)
        onFrame(Math.max(item.startMs, item.startMs + mediaMs - item.cut.startMs))
    }
    const tick = () => {
      if (!active) return
      if (precise)
        handle = el.requestVideoFrameCallback((_now, metadata) => {
          frame(metadata.mediaTime * 1000)
          tick()
        })
      else
        handle = requestAnimationFrame(() => {
          frame(el.currentTime * 1000)
          tick()
        })
    }
    const seeked = () => {
      if (!precise) frame(el.currentTime * 1000)
    }
    el.addEventListener('seeked', seeked)
    tick()
    return () => {
      active = false
      if (precise) el.cancelVideoFrameCallback(handle)
      else cancelAnimationFrame(handle)
      el.removeEventListener('seeked', seeked)
    }
  }, [
    url,
    master,
    playing,
    sourceMs,
    item.startMs,
    item.cut.startMs,
    item.cut.id,
    onFrame,
    onPrecision,
    onDisplayedFrame,
  ])

  const failed = async () => {
    const epoch = playbackEpoch.current
    const el = video.current
    if (
      el?.error?.code === MediaError.MEDIA_ERR_SRC_NOT_SUPPORTED ||
      el?.error?.code === MediaError.MEDIA_ERR_DECODE
    ) {
      setMedia({ fingerprint: fp, error: 'codec' })
      return
    }
    if (refreshing.current) {
      setMedia({ fingerprint: fp, error: 'playback' })
      return
    }
    refreshing.current = true
    try {
      const next = await access(fp, true)
      if (epoch === playbackEpoch.current) setMedia({ fingerprint: fp, url: next })
    } catch (error) {
      if (epoch === playbackEpoch.current)
        setMedia({ fingerprint: fp, error: appFailureFromConnect(error).reason })
    }
  }
  return (
    <>
      {url && (
        <video
          ref={video}
          src={url}
          muted={muted}
          playsInline
          preload="auto"
          aria-hidden="true"
          className="absolute max-w-none"
          onError={() => {
            void failed()
          }}
          style={{
            width: `${crop.width}%`,
            height: `${crop.height}%`,
            left: `${crop.left}%`,
            top: `${crop.top}%`,
            opacity,
            zIndex: item.index,
          }}
        />
      )}
      {master && (!url || error) && (
        <Typography
          variant="body"
          role="status"
          className="text-media-scrim-fg absolute inset-x-0 bottom-0 z-10 p-4"
        >
          {error
            ? t(
                error === 'codec'
                  ? 'preview.codec'
                  : error === 'CLIP_SOURCE_EXPIRED'
                    ? 'preview.expired'
                    : error === 'CLIP_SOURCE_MISSING'
                      ? 'preview.missing'
                      : 'preview.mediaFailed',
              )
            : t('preview.loadingMedia')}
        </Typography>
      )}
    </>
  )
}

export function ClipDraftPreview({
  projectId,
  revision,
  plan,
  ratio,
  sources,
  resolvePlayback,
  timeMs: controlledTime,
  onTimeChange,
  onDisplayedFrame,
  maxHeight,
  compact = false,
  stickyTop,
}: {
  projectId: string
  revision: number
  plan: ClipEditPlan
  ratio: string
  sources: readonly RetainedClipSource[]
  resolvePlayback: PreviewSourceAccess['resolvePlayback']
  timeMs?: number
  onTimeChange?: (ms: number) => void
  onDisplayedFrame?: (frame: ClipDisplayedFrame) => void
  maxHeight?: number
  compact?: boolean
  stickyTop?: number
}) {
  const { t } = useTranslation('clips')
  const transport = useTransport()
  const [localTime, setLocalTime] = useState(0)
  const [playing, setPlaying] = useState(false)
  // A timeline selection/scrub is an external seek. Frame-driven updates use
  // changeTime below and already have the same local value, so playback keeps running.
  if (controlledTime !== undefined && !Object.is(controlledTime, localTime)) {
    setLocalTime(controlledTime)
    setPlaying(false)
  }
  const [muted, setMuted] = useState(true)
  const [precise, setPrecise] = useState(false)
  const [retry, setRetry] = useState(0)
  const [preparation] = useState(
    () =>
      new PreviewPreparation(
        new PreviewAssetCache({
          create: (bytes) =>
            URL.createObjectURL(new Blob([new Uint8Array(bytes)], { type: 'image/png' })),
          revoke: (url) => URL.revokeObjectURL(url),
        }),
      ),
  )
  const snapshot = useSyncExternalStore(preparation.subscribe, preparation.getSnapshot)
  const timeline = useMemo(() => previewTimeline(plan), [plan])
  const duration = Math.max(0, timeline.at(-1)?.endMs ?? 0)
  const requestedTime = controlledTime ?? localTime
  const timeMs = Number.isFinite(requestedTime) ? Math.min(duration, Math.max(0, requestedTime)) : 0
  const changeTime = useCallback(
    (ms: number) => {
      const value = Math.max(0, Math.min(duration, ms))
      setLocalTime(value)
      onTimeChange?.(value)
      if (value >= duration - CLIP_DRAFT_PREVIEW.frameToleranceMs) setPlaying(false)
    },
    [duration, onTimeChange],
  )
  const ids = previewElementIDs(plan, timeline, timeMs)
  const idsJSON = JSON.stringify(ids),
    planJSON = JSON.stringify(plan)
  const contentKey = JSON.stringify([projectId, revision, planJSON])
  const key = JSON.stringify([contentKey, idsJSON, retry])
  useEffect(() => {
    let active = true
    void clipPreviewRequest(
      transport,
      projectId,
      revision,
      JSON.parse(planJSON) as ClipEditPlan,
    ).then(
      (request) => {
        if (active)
          preparation.update(
            key,
            request.hash,
            JSON.parse(idsJSON) as string[],
            request.load,
            contentKey,
          )
      },
      (error: unknown) => {
        if (active) preparation.update(key, '', [], () => Promise.reject(error), contentKey)
      },
    )
    return () => {
      active = false
      preparation.stop()
    }
  }, [transport, projectId, revision, planJSON, idsJSON, key, contentKey, preparation])
  useEffect(
    () => () => {
      preparation.dispose()
    },
    [preparation],
  )

  const ready =
    snapshot.contentKey === contentKey && snapshot.canvasWidth > 0 && snapshot.status !== 'failed'
  const failure = snapshot.key === key && snapshot.status === 'failed'
  const preparationFailure =
    snapshot.error instanceof Error && snapshot.error.message === 'CLIP_PREVIEW_TOO_LARGE'
      ? normalizeAppFailure({ reason: snapshot.error.message, params: {} })
      : appFailureFromConnect(snapshot.error)
  const canvas =
    CLIP_DESIGN.ratios[ratio as ClipRatioId]?.canvas ?? CLIP_DESIGN.ratios.vertical.canvas
  const frames = previewFrame(timeline, timeMs)
  const next = timeline[(frames.at(-1)?.index ?? 0) + 1]
  const slots = frames.map((frame) => ({ ...frame, master: frame === frames.at(-1) }))
  if (slots.length < 2 && next)
    slots.push({ ...next, sourceMs: next.cut.startMs, opacity: 0, audioGain: 0, master: false })
  return (
    <section aria-label={t('preview.title')} className={compact ? 'contents' : 'space-y-3'}>
      {!compact && <Typography variant="fieldTitle">{t('preview.title')}</Typography>}
      <div
        className={stickyTop === undefined ? 'contents' : 'bg-surface-lowest sticky z-10'}
        style={{ top: stickyTop }}
      >
        <div
          data-clip-preview-canvas
          className="bg-media-canvas-bg relative mx-auto w-full overflow-hidden rounded-md"
          style={{
            aspectRatio: `${canvas.width} / ${canvas.height}`,
            maxWidth: maxHeight ? (maxHeight * canvas.width) / canvas.height : undefined,
          }}
        >
          {slots.map((slot) => (
            <PreviewVideo
              key={slot.index % 2}
              item={slot}
              source={sources.find(
                (s) => s.id === slot.cut.sourceId && s.fingerprint === slot.cut.fingerprint,
              )}
              access={resolvePlayback}
              timeMs={timeMs}
              playing={playing && timeMs >= slot.startMs && timeMs < slot.endMs}
              muted={muted}
              opacity={slot.opacity}
              audioGain={slot.audioGain}
              master={slot.master}
              canvas={canvas}
              onFrame={changeTime}
              onPrecision={setPrecise}
              onDisplayedFrame={onDisplayedFrame}
            />
          ))}
          {ready &&
            snapshot.assets.map((asset, index) => {
              const motion = previewMotion(asset, timeMs)
              if (!motion.opacity) return null
              return (
                <img
                  key={`${asset.instanceId}-${asset.startMs}-${index}`}
                  src={asset.url}
                  alt=""
                  aria-hidden="true"
                  className="pointer-events-none absolute max-w-none"
                  style={{
                    width: `${(asset.width / snapshot.canvasWidth) * 100}%`,
                    height: `${(asset.height / snapshot.canvasHeight) * 100}%`,
                    left: `${(asset.x / snapshot.canvasWidth) * 100}%`,
                    top: `${((asset.y + motion.dy) / snapshot.canvasHeight) * 100}%`,
                    opacity: motion.opacity,
                    zIndex: timeline.length + asset.layer,
                  }}
                />
              )
            })}
        </div>
      </div>
      <div className={`flex flex-wrap items-center gap-4 ${compact ? 'mt-3' : ''}`}>
        <Button
          variant="secondary"
          disabled={!timeline.length}
          onClick={() => {
            if (timeMs >= duration - CLIP_DRAFT_PREVIEW.frameToleranceMs) changeTime(0)
            setPlaying(!playing)
          }}
        >
          {t(playing ? 'preview.pause' : 'preview.play')}
        </Button>
        <label className="flex items-center gap-3">
          <Checkbox checked={!muted} onChange={(event) => setMuted(!event.target.checked)} />
          <Typography variant="body" as="span">
            {t('preview.originalAudio')}
          </Typography>
        </label>
      </div>
      {!compact && (
        <Slider
          label={t('preview.outputTime')}
          min={0}
          max={Math.max(1, duration)}
          step={CLIP_DRAFT_PREVIEW.seekStepMs}
          value={timeMs}
          valueText={`${(timeMs / 1000).toFixed(3)} / ${(duration / 1000).toFixed(3)} s`}
          onChange={(ms) => {
            setPlaying(false)
            changeTime(ms)
          }}
        />
      )}
      {!timeline.length && (
        <Typography variant="body" role="status" className="text-content-secondary">
          {t('preview.invalidTimeline')}
        </Typography>
      )}
      <Typography
        variant="body"
        role="status"
        className={compact && !failure ? 'sr-only' : 'text-content-secondary'}
      >
        {failure
          ? t('preview.preparationFailed')
          : !ready || snapshot.status === 'updating'
            ? t('preview.updating')
            : t('preview.currentDraft')}
      </Typography>
      {failure && preparationFailure.reason !== 'UNKNOWN_FAILURE' && (
        <AppFailureMessage failure={preparationFailure} />
      )}
      {failure && (
        <Button variant="secondary" onClick={() => setRetry((v) => v + 1)}>
          {t('preview.retry')}
        </Button>
      )}
      <details className={compact ? 'my-3' : undefined}>
        <summary className="text-content-secondary cursor-pointer">
          <Typography as="span" variant="meta">
            {t('preview.parityLabel')}
          </Typography>
        </summary>
        <Typography variant="body" className="text-content-secondary">
          {t('preview.parity')}
        </Typography>
        {!precise && (
          <Typography variant="body" className="text-content-secondary">
            {t('preview.frameApproximate')}
          </Typography>
        )}
      </details>
    </section>
  )
}
