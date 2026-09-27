import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { RetainedClipSource } from '@/entities/clip-plan/@x/clip-preview'
import { Typography } from '@/shared/ui'
import {
  flowAssets,
  flowCut,
  previewCrop,
  type ClipPreviewOverlay,
  type PreviewCut,
  type PreviewSourceAccess,
} from '../model/draft-preview'

/** One cut as the flow simulation stands it (CLIP-173, CLIP-175): the frame at the cut's own
 *  source start, never played — from the session's copy of the footage or the retained original
 *  the resolver authorizes — and, where neither answers, the cut's number on a neutral ground. */
function CutStill({
  item,
  source,
  access,
  canvas,
}: {
  item: PreviewCut
  source?: RetainedClipSource
  access?: PreviewSourceAccess['resolvePlayback']
  canvas: { width: number; height: number }
}) {
  const { t } = useTranslation('clips')
  const fp = item.cut.fingerprint
  // Remembered with the footage it belongs to, so another cut never shows this one's frame.
  const [media, setMedia] = useState<{ fingerprint: string; url?: string; failed?: boolean }>({
    fingerprint: '',
  })
  useEffect(() => {
    let active = true
    if (!access) return
    access(fp).then(
      (url) => {
        if (active) setMedia({ fingerprint: fp, url })
      },
      () => {
        if (active) setMedia({ fingerprint: fp, failed: true })
      },
    )
    return () => {
      active = false
    }
  }, [access, fp])
  const url = media.fingerprint === fp && !media.failed ? media.url : undefined
  const crop = previewCrop(
    source?.width || canvas.width,
    source?.height || canvas.height,
    canvas.width,
    canvas.height,
    item.cut.focal,
  )
  return (
    <>
      <div
        data-flow-fallback={item.index + 1}
        className="bg-media-canvas-bg absolute inset-0 flex items-center justify-center"
      >
        <Typography variant="title" as="span" className="text-media-scrim-fg">
          {t('preview.cutNumber', { n: item.index + 1 })}
        </Typography>
      </div>
      {url && (
        <video
          src={url}
          muted
          playsInline
          preload="auto"
          aria-hidden="true"
          data-flow-still={item.cut.id}
          className="absolute max-w-none"
          style={{
            width: `${crop.width}%`,
            height: `${crop.height}%`,
            left: `${crop.left}%`,
            top: `${crop.top}%`,
          }}
          onLoadedMetadata={(event) => {
            event.currentTarget.currentTime = Math.max(0, item.cut.startMs / 1000)
          }}
          onError={() => setMedia({ fingerprint: fp, failed: true })}
        />
      )}
    </>
  )
}

/** ②'s flow simulation (CLIP-173 – CLIP-176): the draft at the playhead as one still per cut
 *  with the server-drawn overlay held still over it — each region block, caption and badge shown
 *  while its interval holds the playhead — with no playback, motion or sound. The scrubber's end
 *  draws the last instant, never the empty one after it. */
export function ClipFlowFrame({
  timeline,
  timeMs,
  preview,
  sources,
  resolvePlayback,
  canvas,
}: {
  timeline: readonly PreviewCut[]
  timeMs: number
  preview: ClipPreviewOverlay
  sources: readonly RetainedClipSource[]
  resolvePlayback?: PreviewSourceAccess['resolvePlayback']
  canvas: { width: number; height: number }
}) {
  const item = flowCut(timeline, timeMs)
  return (
    <div data-flow-simulation className="absolute inset-0">
      {item && (
        <CutStill
          key={`${item.cut.id}:${item.cut.startMs}`}
          item={item}
          source={sources.find(
            (s) => s.id === item.cut.sourceId && s.fingerprint === item.cut.fingerprint,
          )}
          access={resolvePlayback}
          canvas={canvas}
        />
      )}
      {preview.ready &&
        flowAssets(preview.assets, timeline, timeMs).map((asset, index) => (
          <img
            key={`${asset.instanceId}-${asset.startMs}-${index}`}
            src={asset.url}
            alt=""
            aria-hidden="true"
            data-flow-asset={asset.instanceId}
            className="pointer-events-none absolute max-w-none"
            style={{
              width: `${(asset.width / preview.canvasWidth) * 100}%`,
              height: `${(asset.height / preview.canvasHeight) * 100}%`,
              left: `${(asset.x / preview.canvasWidth) * 100}%`,
              top: `${(asset.y / preview.canvasHeight) * 100}%`,
              zIndex: 1 + asset.layer,
            }}
          />
        ))}
    </div>
  )
}
