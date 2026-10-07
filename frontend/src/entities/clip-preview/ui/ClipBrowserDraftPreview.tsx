import { useCallback, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Film, Images, Play, RefreshCw, RotateCcw, Volume2, VolumeX } from 'lucide-react'
import {
  CLIP_BROWSER_RENDER,
  CLIP_DRAFT_PREVIEW,
  CLIP_DESIGN,
  type ClipRatioId,
} from '@/entities/clip-design/@x/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
import { Button, Slider, Typography } from '@/shared/ui'
import { BrowserPreviewWorker } from '../lib/create-preview-worker'
import { BrowserCompositionPlayback } from '../model/composition-playback'
import { browserAudioPlan } from '../model/browser-audio-plan'
import { flowCut, previewTimeline, type ClipPreviewOverlay } from '../model/draft-preview'
import type { ClipLocalCompositionRuntime } from '../model/local-preview'
import type { SpeechAudioLoader } from '../model/speech-playback'
import type { ClipDisplayedFrame } from './ClipDraftPreview'
import { CLIP_LOCAL_PREVIEW } from '../config/local-preview'
import type { BrowserPreviewFrameRequest } from '../model/preview-worker-protocol'

export interface ClipBrowserDraftPreviewProps {
  preview: ClipPreviewOverlay
  local?: ClipLocalCompositionRuntime
  plan: ClipEditPlan
  ratio: string
  timeMs?: number
  onTimeChange?: (ms: number) => void
  onDisplayedFrame?: (frame: ClipDisplayedFrame) => void
  maxHeight?: number
  compact?: boolean
  suspended?: boolean
  stickyTop?: number
  corner?: ReactNode
  editingOverlay?: ReactNode
  captionPosition?: { instanceId: string; x: number; y: number }
  loadSpeech?: SpeechAudioLoader
}
/** React owns controls. Worker frames and one output clock run independently of React rendering. */
export function ClipBrowserDraftPreview(props: ClipBrowserDraftPreviewProps) {
  const { t } = useTranslation('clips')
  const {
    preview,
    local,
    plan,
    ratio,
    maxHeight,
    compact = false,
    suspended = false,
    stickyTop,
    corner,
    editingOverlay,
    captionPosition,
  } = props
  const canvas =
    CLIP_DESIGN.ratios[ratio as ClipRatioId]?.canvas ?? CLIP_DESIGN.ratios.vertical.canvas
  const timeline = previewTimeline(plan),
    duration = local
      ? (local.snapshot.frameCount * 1000) / CLIP_BROWSER_RENDER.frameRate
      : Math.max(0, timeline.at(-1)?.endMs ?? 0)
  const surface = useRef<HTMLCanvasElement>(null),
    session = useRef<BrowserPreviewWorker | undefined>(undefined),
    transport = useRef<BrowserCompositionPlayback | undefined>(undefined)
  const latest = useRef(props)
  const [localTime, setLocalTime] = useState(0),
    [playingState, setPlaying] = useState(false),
    [muted, setMuted] = useState(!plan.narration?.enabled),
    [flow, setFlow] = useState(false),
    [readyKey, setReadyKey] = useState(''),
    [preparingState, setPreparing] = useState(false),
    [error, setError] = useState<string>(),
    [audioFailure, setAudioFailure] = useState<{ key: string; error: string }>(),
    [neutral, setNeutral] = useState(false),
    [refresh, setRefresh] = useState(0)
  const fingerprint = local?.snapshot.snapshotFingerprint
  const runtimeKey = `${fingerprint ?? ''}/${refresh}`
  const [playKey, setPlayKey] = useState(''),
    [prepareKey, setPrepareKey] = useState('')
  const playing = playingState && playKey === runtimeKey && !flow && !suspended
  const preparing = preparingState && prepareKey === runtimeKey && !flow && !suspended
  const ready = !!fingerprint && readyKey === runtimeKey
  const audioError = audioFailure?.key === runtimeKey ? audioFailure.error : undefined
  const mutedRef = useRef(muted)
  const flowRef = useRef(flow)
  const playingRef = useRef(playing)
  useLayoutEffect(() => {
    latest.current = props
    mutedRef.current = muted
    flowRef.current = flow
    playingRef.current = playing
  })
  const localClock = useRef(localTime),
    controlled = useRef(props.timeMs)
  const audioIdentity = useRef('')
  const frameOwner = useRef(new AbortController()),
    drawEpoch = useRef(0),
    readyRef = useRef(false),
    pending = useRef<BrowserPreviewFrameRequest | undefined>(undefined),
    busy = useRef(false),
    lastRequested = useRef('')
  const timeMs = Math.max(0, Math.min(duration, props.timeMs ?? localTime))
  const changeTime = useCallback(
    (time: number) => {
      const value = Math.max(0, Math.min(duration, time))
      localClock.current = value
      setLocalTime(value)
      latest.current.onTimeChange?.(value)
    },
    [duration],
  )
  const invalidate = useCallback(() => {
    drawEpoch.current++
    frameOwner.current.abort()
    frameOwner.current = new AbortController()
    pending.current = undefined
    lastRequested.current = ''
    session.current?.cancel()
  }, [])
  const draw = useRef<(time: number) => void>(() => {})
  const dispatch = useRef<(request: BrowserPreviewFrameRequest) => void>(() => {})
  useLayoutEffect(() => {
    dispatch.current = (request) => {
      if (
        !session.current ||
        !readyRef.current ||
        latest.current.suspended ||
        !latest.current.local
      )
        return
      if (busy.current) {
        pending.current = request
        return
      }
      const own = drawEpoch.current,
        worker = session.current,
        snapshot = latest.current.local.snapshot,
        signal = frameOwner.current.signal
      busy.current = true
      void worker
        .render(request, signal)
        .then((result) => {
          try {
            if (
              signal.aborted ||
              own !== drawEpoch.current ||
              snapshot.snapshotFingerprint !== latest.current.local?.snapshot.snapshotFingerprint ||
              !surface.current ||
              request.flow !== flowRef.current ||
              JSON.stringify(request.captionPosition) !==
                JSON.stringify(latest.current.captionPosition)
            )
              return
            const context = surface.current.getContext('2d', { alpha: false })
            if (!context) throw new Error('CLIP_CANVAS_UNAVAILABLE')
            context.drawImage(result.bitmap, 0, 0)
            surface.current.dataset.clipCaptionPosition = JSON.stringify(
              result.captionPosition ?? null,
            )
            surface.current.dataset.clipLocalFrame = String(result.frame)
            surface.current.dataset.clipLocalFingerprint = result.fingerprint
            surface.current.dataset.clipLocalFlow = String(result.flow)
            surface.current.dataset.clipLiveFrames = String(result.resources.liveFrames)
            surface.current.dataset.clipBackgroundMeasured = String(result.backgroundMeasured)
            setNeutral(result.flow && !result.displayed.length)
            setError(undefined)
            if (!result.flow) {
              const displayed = result.displayed.at(-1)
              if (displayed) latest.current.onDisplayedFrame?.({ ...displayed, precise: true })
            }
          } finally {
            result.bitmap.close()
          }
        })
        .catch((failure: unknown) => {
          if (!signal.aborted && own === drawEpoch.current) {
            lastRequested.current = ''
            setError(failure instanceof Error ? failure.message : 'CLIP_PREVIEW_FAILED')
            const context = surface.current?.getContext('2d')
            if (context && surface.current)
              context.clearRect(0, 0, surface.current.width, surface.current.height)
          }
        })
        .finally(() => {
          busy.current = false
          const next = pending.current
          pending.current = undefined
          if (next) dispatch.current(next)
        })
    }
    draw.current = (time) => {
      const runtime = latest.current.local
      if (!runtime || !readyRef.current || latest.current.suspended) return
      const frame = Math.min(
        runtime.snapshot.frameCount - 1,
        Math.max(0, Math.floor((time * CLIP_BROWSER_RENDER.frameRate) / 1000)),
      )
      const request = {
          frame,
          ...(flowRef.current ? { timeMs: time } : {}),
          flow: flowRef.current,
          captionPosition: latest.current.captionPosition,
        },
        key = JSON.stringify(request)
      if (lastRequested.current === key) return
      lastRequested.current = key
      dispatch.current(request)
    }
  })
  const stop = useCallback(() => {
    if (transport.current?.running) changeTime(transport.current.timeMs)
    transport.current?.pause()
    setPlaying(false)
    setPreparing(false)
    invalidate()
  }, [changeTime, invalidate])
  useLayoutEffect(() => {
    try {
      session.current = new BrowserPreviewWorker((id, fingerprint, signal) =>
        latest.current.local
          ? latest.current.local.source(id, fingerprint, signal)
          : Promise.reject(new Error('CLIP_SOURCE_UNAVAILABLE')),
      )
    } catch {
      queueMicrotask(() => setError('CLIP_PREVIEW_WORKER_UNAVAILABLE'))
    }
    return () => {
      invalidate()
      session.current?.dispose()
      session.current = undefined
    }
  }, [invalidate])
  useLayoutEffect(() => {
    const runtime = latest.current.local,
      worker = session.current,
      controller = new AbortController()
    readyRef.current = false
    invalidate()
    transport.current?.pause()
    if (!runtime || !worker) return
    const signature = JSON.stringify([
      runtime.snapshot.ownerId,
      runtime.snapshot.projectId,
      runtime.snapshot.versions,
      runtime.snapshot.sources,
      browserAudioPlan(runtime.snapshot.plan as ClipEditPlan),
    ])
    if (!transport.current || signature !== audioIdentity.current) {
      transport.current?.dispose()
      transport.current = new BrowserCompositionPlayback(
        runtime.snapshot.plan as ClipEditPlan,
        (fingerprint, signal) =>
          latest.current.local
            ? latest.current.local.audioSource(fingerprint, signal)
            : Promise.reject(new Error('CLIP_SOURCE_UNAVAILABLE')),
        (speech, signal) =>
          latest.current.loadSpeech
            ? latest.current.loadSpeech(speech, signal)
            : Promise.reject(new Error('CLIP_SPEECH_UNAVAILABLE')),
      )
      audioIdentity.current = signature
    }
    transport.current.setMuted(mutedRef.current)
    void worker.initialize(runtime.snapshot, controller.signal).then(
      () => {
        if (!controller.signal.aborted) {
          readyRef.current = true
          setReadyKey(runtimeKey)
          draw.current(latest.current.timeMs ?? localClock.current)
        }
      },
      (failure: unknown) => {
        if (!controller.signal.aborted)
          setError(failure instanceof Error ? failure.message : 'CLIP_PREVIEW_FAILED')
      },
    )
    return () => {
      controller.abort()
      invalidate()
      transport.current?.pause()
      readyRef.current = false
    }
    // The snapshot binds the exact draft. Playhead changes never rebuild the renderer.
  }, [runtimeKey, invalidate])
  useEffect(
    () => () => {
      transport.current?.dispose()
      transport.current = undefined
      audioIdentity.current = ''
    },
    [],
  )
  useEffect(() => transport.current?.setMuted(muted), [muted])
  useLayoutEffect(() => {
    if (props.timeMs !== controlled.current) {
      controlled.current = props.timeMs
      if (props.timeMs !== undefined && Math.abs(props.timeMs - localClock.current) > 0.001) {
        transport.current?.pause()
        setPlaying(false)
        setPreparing(false)
        invalidate()
        localClock.current = props.timeMs
        setLocalTime(props.timeMs)
      }
    }
    if (!playingRef.current) draw.current(props.timeMs ?? localClock.current)
  }, [props.timeMs, invalidate, fingerprint, ready])
  useEffect(() => {
    if (suspended) stop()
    else draw.current(latest.current.timeMs ?? localClock.current)
  }, [suspended, stop])
  useEffect(() => {
    draw.current(latest.current.timeMs ?? localClock.current)
  }, [flow, captionPosition, ready])
  useEffect(() => {
    if (!playing || flow || suspended) return
    let raf = 0,
      lastDraw = -Infinity
    const tick = (now: number) => {
      const playback = transport.current
      if (!playback?.running) {
        setPlaying(false)
        return
      }
      const time = playback.timeMs
      if (now - lastDraw >= 1000 / CLIP_LOCAL_PREVIEW.cadence || time >= duration) {
        lastDraw = now
        draw.current(time)
        changeTime(time)
      }
      if (time >= duration) {
        playback.pause()
        setPlaying(false)
        return
      }
      raf = requestAnimationFrame(tick)
    }
    raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [playing, flow, suspended, duration, changeTime, stop])
  useEffect(() => {
    const hidden = () => {
      if (document.hidden) stop()
    }
    document.addEventListener('visibilitychange', hidden)
    if (document.hidden) queueMicrotask(hidden)
    return () => document.removeEventListener('visibilitychange', hidden)
  }, [stop])
  const atEnd = timeMs >= duration - CLIP_DRAFT_PREVIEW.frameToleranceMs
  const toggle = () => {
    if (document.hidden) {
      stop()
      return
    }
    if (playing || preparing) {
      stop()
      setPreparing(false)
      return
    }
    const playback = transport.current
    if (!playback || !ready) return
    const starts = atEnd ? 0 : timeMs
    if (atEnd) changeTime(0)
    invalidate()
    setPlayKey(runtimeKey)
    setPrepareKey(runtimeKey)
    setPreparing(true)
    setAudioFailure(undefined)
    const own = drawEpoch.current
    void playback
      .play(starts)
      .then(
        (started) => {
          if (own === drawEpoch.current) setPlaying(started)
        },
        (failure: unknown) => {
          if (own === drawEpoch.current)
            setAudioFailure({
              key: runtimeKey,
              error: failure instanceof Error ? failure.message : 'CLIP_SPEECH_UNAVAILABLE',
            })
        },
      )
      .finally(() => {
        if (own === drawEpoch.current) setPreparing(false)
      })
  }
  const schedule = browserAudioPlan(plan),
    selectedFlow = flowCut(timeline, timeMs)
  return (
    <section aria-label={t('preview.title')} className={compact ? 'contents' : 'space-y-3'}>
      {!compact && <Typography variant="fieldTitle">{t('preview.title')}</Typography>}
      <div
        className={stickyTop === undefined ? 'contents' : 'bg-surface-lowest sticky z-10'}
        style={{
          top:
            stickyTop === undefined
              ? undefined
              : `calc(var(--spacing-chrome, 0px) + ${stickyTop}px)`,
        }}
      >
        <div
          className="relative mx-auto w-full"
          style={{ maxWidth: maxHeight ? (maxHeight * canvas.width) / canvas.height : undefined }}
        >
          <div
            data-clip-preview-canvas
            className="bg-media-canvas-bg relative isolate w-full overflow-hidden rounded-md"
            style={{ aspectRatio: `${canvas.width} / ${canvas.height}` }}
          >
            <canvas
              ref={surface}
              width={Math.round(canvas.width * CLIP_LOCAL_PREVIEW.scale)}
              height={Math.round(canvas.height * CLIP_LOCAL_PREVIEW.scale)}
              className="absolute inset-0 h-full w-full"
              aria-hidden="true"
              data-clip-local-preview
            />
            {neutral && flow && (
              <Typography
                variant="fieldTitle"
                className="text-media-scrim-fg absolute inset-0 flex items-center justify-center"
              >
                {t('preview.cutNumber', { n: (selectedFlow?.index ?? 0) + 1 })}
              </Typography>
            )}
            {!flow && (
              <button
                type="button"
                aria-label={t(
                  playing ? 'preview.pause' : atEnd ? 'preview.replay' : 'preview.play',
                )}
                disabled={!timeline.length || !ready}
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
            )}
            {editingOverlay}
          </div>
          <div className="absolute top-2 right-2 z-30 flex items-center gap-1">
            {!flow && (
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
            )}
            <Button
              variant="scrim"
              size="icon"
              aria-label={t('preview.refresh')}
              onClick={() => {
                stop()
                preview.onRetry()
                setRefresh((value) => value + 1)
              }}
            >
              <RefreshCw aria-hidden="true" className="size-5" />
            </Button>
            <Button
              variant="scrim"
              size="icon"
              aria-label={t(flow ? 'preview.videoView' : 'preview.flowView')}
              disabled={!timeline.length}
              onClick={() => {
                stop()
                setFlow(!flow)
              }}
            >
              {flow ? (
                <Film aria-hidden="true" className="size-5" />
              ) : (
                <Images aria-hidden="true" className="size-5" />
              )}
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
            stop()
            changeTime(ms)
            draw.current(ms)
          }}
        />
      )}
      {schedule.speechIssues.map((issue) => (
        <Typography key={issue.segmentId} variant="meta" role="status">
          {t(
            `preview.speech${issue.state === 'stale' ? 'Stale' : issue.state === 'conflict' ? 'Conflict' : 'Missing'}`,
            {
              segment:
                plan.narration!.segments.findIndex((segment) => segment.id === issue.segmentId) + 1,
            },
          )}
          {issue.previous && ` ${t('preview.previousSpeech')}`}
        </Typography>
      ))}
      {audioError && (
        <Typography variant="meta" role="alert">
          {t(audioError === 'gesture' ? 'preview.playbackGesture' : 'preview.audioFailed')}
        </Typography>
      )}
      {preparing && (
        <Typography variant="meta" role="status">
          {t('preview.speechLoading')}
        </Typography>
      )}
      {error && (
        <Typography variant="meta" role="alert">
          {t(
            error.includes('EXPIRED')
              ? 'preview.expired'
              : error.includes('MISSING')
                ? 'preview.missing'
                : 'preview.mediaFailed',
          )}
        </Typography>
      )}
      <Typography
        variant="body"
        role="status"
        className={compact && !error && !preview.failure ? 'sr-only' : 'text-content-secondary'}
      >
        {error || preview.failure
          ? t('preview.preparationFailed')
          : !ready || preview.updating
            ? t('preview.updating')
            : t('preview.currentDraft')}
      </Typography>
      {(error || preview.failure) && (
        <Button
          variant="secondary"
          onClick={() => {
            stop()
            preview.onRetry()
            setRefresh((value) => value + 1)
          }}
        >
          {t('preview.retry')}
        </Button>
      )}
    </section>
  )
}
