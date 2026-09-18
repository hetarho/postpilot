import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipEditCut } from '@/entities/clip-project'

/** The frame a cut's own controls are edited against, inside its sheet
 *  (CLIP-53). It is the SOURCE, played locally and seeked to the cut's start,
 *  rather than the composed output: trimming, splitting and rate are
 *  source-time work, and the composed preview above the timeline already shows
 *  the output (CLIP-67).
 *
 *  Where the session holds no copy of this footage the unexpired retained
 *  original stands in, as the caption's own stage does (CLIP-50, CLIP-57); with
 *  neither, the sheet simply carries no frame — the controls are all still
 *  usable, so a missing frame is silence rather than a refusal. */
export function ClipCutSourceFrame({
  cut,
  localSources,
  resolvePlayback,
}: {
  cut: ClipEditCut
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
}) {
  const { t } = useTranslation('clips')
  const local = localSources.find((s) => s.fingerprint === cut.fingerprint)?.url
  // Remembered WITH the fingerprint it belongs to, so selecting another cut
  // never shows the frame of the last one.
  const [retained, setRetained] = useState<{ fingerprint: string; url: string }>()
  useEffect(() => {
    let active = true
    if (local || !resolvePlayback) return
    resolvePlayback(cut.fingerprint).then(
      (url) => {
        if (active) setRetained({ fingerprint: cut.fingerprint, url })
      },
      () => {},
    )
    return () => {
      active = false
    }
  }, [local, cut.fingerprint, resolvePlayback])
  const url = local ?? (retained?.fingerprint === cut.fingerprint ? retained.url : undefined)
  if (!url) return null
  return (
    <video
      // Keyed by the FOOTAGE, not by the range: re-mounting on every keystroke
      // of a trim would take the frame away while it is being typed.
      key={url}
      controls
      preload="metadata"
      src={url}
      aria-label={t('correction.sourcePreview')}
      className="aspect-video w-full rounded-md"
      onLoadedMetadata={(event) => {
        event.currentTarget.currentTime = Number.isFinite(cut.startMs)
          ? Math.max(0, cut.startMs / 1000)
          : 0
      }}
    />
  )
}
