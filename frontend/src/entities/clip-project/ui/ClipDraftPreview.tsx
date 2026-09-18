import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react'
import { useTransport } from '@connectrpc/connect-query'
import { useTranslation } from 'react-i18next'
import { Play, RefreshCw, RotateCcw, Volume2, VolumeX } from 'lucide-react'
import { CLIP_DRAFT_PREVIEW, CLIP_DESIGN, type ClipRatioId } from '@/shared/config'
import { appFailureFromConnect, normalizeAppFailure } from '@/shared/api'
import { AppFailureMessage, Button, Slider, Typography } from '@/shared/ui'
import { clipPreviewRequest } from '../api/preview'
import {
  cutRate,
  outputToSourceMs,
  sourceToOutputMs,
  sourceAudioEnabled,
  type ClipEditPlan,
  type RetainedClipSource,
} from '../model/edit-plan'
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
  reload,
  timeMs,
  playing,
  muted,
  opacity,
  audioGain,
  master,
  canvas,
  onFrame,
  onDisplayedFrame,
  onPlayRefused,
}: {
  item: PreviewCut
  source?: RetainedClipSource
  access: PreviewSourceAccess['resolvePlayback']
  /** Bumped by the preview's refresh: the footage is fetched again, with a fresh URL. */
  reload: number
  timeMs: number
  playing: boolean
  muted: boolean
  opacity: number
  audioGain: number
  master: boolean
  canvas: { width: number; height: number }
  onFrame: (ms: number) => void
  onDisplayedFrame?: (frame: ClipDisplayedFrame) => void
  /** The browser refused to start this footage without a gesture (autoplay policy). */
  onPlayRefused?: () => void
}) {
  const { t } = useTranslation('clips')
  const video = useRef<HTMLVideoElement>(null)
  const [media, setMedia] = useState<{ fingerprint: string; url?: string; error?: string }>({
    fingerprint: '',
  })
  const refreshing = useRef(false)
  const playbackEpoch = useRef(0)
  // The last position this player was TOLD to show, so a paused player is seeked once per new
  // position rather than on every render, and never for a position it already holds.
  const requested = useRef<{ url: string; sourceMs: number }>(undefined)
  const wasPlaying = useRef(false)
  // The cut this player draws, read by the frame loop through a ref: the slot object is rebuilt
  // on every render, and a loop torn down and re-registered per frame drops frames.
  const itemRef = useRef(item)
  useLayoutEffect(() => {
    itemRef.current = item
  })
  const fp = item.cut.fingerprint
  const rate = cutRate(item.cut) / 1000
  const sourceMs = outputToSourceMs(
    item,
    Math.max(item.startMs, Math.min(item.endMs - CLIP_DRAFT_PREVIEW.frameToleranceMs, timeMs)),
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
    void (reload > 0 ? access(fp, true) : access(fp)).then(
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
  }, [fp, access, reload])

  // WHO seeks WHOM. A paused player shows the requested position, so it is seeked whenever that
  // position changes. A playing MASTER is the clock: its own presented frames are what set the
  // time, so seeking it back to the time it reported a render ago only makes it stutter — on a
  // slow frame the drift outran the old two-frame tolerance and the picture hopped backwards
  // every few frames. It is positioned once, when its playback starts. A playing partner (the
  // outgoing side of a fade) follows the master's clock loosely.
  useEffect(() => {
    const el = video.current
    if (!el || !url) return
    const sync = () => {
      el.playbackRate = rate
      if ('preservesPitch' in el) el.preservesPitch = true
      if ('webkitPreservesPitch' in el) el.webkitPreservesPitch = true
      if ('mozPreservesPitch' in el) el.mozPreservesPitch = true
      el.volume = Number.isFinite(item.cut.volumePermille)
        ? Math.max(0, Math.min(1, (item.cut.volumePermille / 1000) * audioGain))
        : 0
      if (!el.readyState) return
      const started = playing && !wasPlaying.current
      wasPlaying.current = playing
      const drift = Math.abs(el.currentTime * 1000 - sourceMs)
      const seek = () => {
        el.currentTime = sourceMs / 1000
        requested.current = { url, sourceMs }
      }
      if (!playing) {
        if (requested.current?.url !== url || requested.current.sourceMs !== sourceMs) seek()
      } else if (!master) {
        if (!el.seeking && drift > CLIP_DRAFT_PREVIEW.frameToleranceMs * 4) seek()
      } else if (started && drift > CLIP_DRAFT_PREVIEW.frameToleranceMs) seek()
    }
    sync()
    el.addEventListener('loadedmetadata', sync)
    return () => {
      el.removeEventListener('loadedmetadata', sync)
    }
  }, [url, sourceMs, playing, master, item.cut.volumePermille, audioGain, rate])

  useEffect(() => {
    const el = video.current
    if (!el || !url) return
    if (playing) {
      const attempt: Promise<void> | undefined = el.play()
      attempt?.catch((error: unknown) => {
        // Interrupted by our own pause() on the next toggle: not a media failure.
        if (error instanceof DOMException && error.name === 'AbortError') return
        // The browser wants a gesture for this footage (sound on, no recent tap): the preview
        // stops where it is instead of dropping the footage as broken.
        if (error instanceof DOMException && error.name === 'NotAllowedError') {
          onPlayRefused?.()
          return
        }
        setMedia({ fingerprint: fp, error: 'playback' })
      })
    } else el.pause()
    return () => {
      el.pause()
    }
  }, [playing, url, fp, onPlayRefused])

  useEffect(() => {
    const el = video.current
    if (!el || !url || !master) return
    let active = true,
      handle = 0
    const precise = typeof el.requestVideoFrameCallback === 'function'
    const frame = (mediaMs: number) => {
      if (!active || el.seeking) return
      const item = itemRef.current
      onDisplayedFrame?.({
        cutId: item.cut.id,
        sourceMs: Math.round(mediaMs),
        outputMs: sourceToOutputMs(item, mediaMs),
        precise,
      })
      if (!playing) return
      // Past the cut's end the frame belongs to the NEXT cut, whatever this source still holds.
      onFrame(
        mediaMs >= item.cut.endMs
          ? item.endMs
          : Math.max(item.startMs, sourceToOutputMs(item, mediaMs)),
      )
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
    // A cut that runs to the end of its source never presents a frame at or past the cut's end:
    // the element stops at EOF with the clock a frame short, and nothing hands over to the next
    // cut. The end of the footage IS the end of the cut.
    const ended = () => {
      if (active && playing) onFrame(itemRef.current.endMs)
    }
    el.addEventListener('seeked', seeked)
    el.addEventListener('ended', ended)
    tick()
    return () => {
      active = false
      if (precise) el.cancelVideoFrameCallback(handle)
      else cancelAnimationFrame(handle)
      el.removeEventListener('seeked', seeked)
      el.removeEventListener('ended', ended)
    }
  }, [url, master, playing, onFrame, onDisplayedFrame])

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
  corner,
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
  /** Controls the caller overlays at the frame's top-right beside the player's own — ②'s info
   *  control (CLIP-148). Rendered outside the clipped frame so a panel it opens is not cut off. */
  corner?: ReactNode
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
  const [retry, setRetry] = useState(0)
  const [reload, setReload] = useState(0)
  const stopPlayback = useCallback(() => setPlaying(false), [])
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
  const atEnd = timeMs >= duration - CLIP_DRAFT_PREVIEW.frameToleranceMs
  // The frame is the player's own control, as a video player's is: one press plays, one pauses,
  // and at the end one press starts over.
  const toggle = () => {
    if (!timeline.length) return
    if (playing) {
      setPlaying(false)
      return
    }
    if (atEnd) changeTime(0)
    setPlaying(true)
  }
  // Fetch the footage and the caption glyphs again, keeping the position: the way out of a
  // player that stopped answering, without leaving the step.
  const refresh = () => {
    setPlaying(false)
    setRetry((value) => value + 1)
    setReload((value) => value + 1)
  }
  return (
    <section aria-label={t('preview.title')} className={compact ? 'contents' : 'space-y-3'}>
      {!compact && <Typography variant="fieldTitle">{t('preview.title')}</Typography>}
      <div
        className={stickyTop === undefined ? 'contents' : 'bg-surface-lowest sticky z-10'}
        style={{ top: stickyTop }}
      >
        {/* The frame clips its footage; the controls stand on a wrapper that does not, so the
            panel the info control opens is not cut off at the frame's edge. */}
        <div
          className="relative mx-auto w-full"
          style={{
            maxWidth: maxHeight ? (maxHeight * canvas.width) / canvas.height : undefined,
          }}
        >
          <div
            data-clip-preview-canvas
            className="bg-media-canvas-bg relative w-full overflow-hidden rounded-md"
            style={{ aspectRatio: `${canvas.width} / ${canvas.height}` }}
          >
            {slots.map((slot) => (
              <PreviewVideo
                key={slot.index % 2}
                item={slot}
                source={sources.find(
                  (s) => s.id === slot.cut.sourceId && s.fingerprint === slot.cut.fingerprint,
                )}
                access={resolvePlayback}
                reload={reload}
                timeMs={timeMs}
                playing={playing && timeMs >= slot.startMs && timeMs < slot.endMs}
                muted={muted || !sourceAudioEnabled(plan, slot.cut)}
                opacity={slot.opacity}
                audioGain={slot.audioGain}
                master={slot.master}
                canvas={canvas}
                onFrame={changeTime}
                onDisplayedFrame={onDisplayedFrame}
                onPlayRefused={stopPlayback}
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
            <button
              type="button"
              aria-label={t(playing ? 'preview.pause' : atEnd ? 'preview.replay' : 'preview.play')}
              disabled={!timeline.length}
              onClick={toggle}
              className="absolute inset-0 z-20 flex items-center justify-center"
            >
              {!playing && timeline.length > 0 && (
                <span
                  aria-hidden="true"
                  className="bg-media-scrim-bg/60 text-media-scrim-fg flex size-14 items-center justify-center rounded-full"
                >
                  {atEnd ? <RotateCcw className="size-7" /> : <Play className="size-7" />}
                </span>
              )}
            </button>
          </div>
          <div className="absolute top-2 right-2 z-30 flex items-center gap-1">
            <Button
              variant="scrim"
              size="icon"
              aria-label={t('preview.originalAudio')}
              aria-pressed={!muted}
              onClick={() => setMuted(!muted)}
            >
              {muted ? (
                <VolumeX aria-hidden="true" className="size-5" />
              ) : (
                <Volume2 aria-hidden="true" className="size-5" />
              )}
            </Button>
            <Button variant="scrim" size="icon" aria-label={t('preview.refresh')} onClick={refresh}>
              <RefreshCw aria-hidden="true" className="size-5" />
            </Button>
            {corner}
          </div>
        </div>
      </div>
      {!compact && (
        <Slider
          ariaLabel={t('preview.outputTime')}
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
      {/* No parity block here: what the draft preview cannot promise about the
          delivered render is reached from the ONE info control on the frame, and
          stands as permanent text nowhere (CLIP-148). The frame's own precision
          travels with each displayed frame, so the control that says it does not
          need this component's state. */}
    </section>
  )
}
