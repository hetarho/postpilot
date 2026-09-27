import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Typography } from '@/shared/ui'
import type { ClipScene } from '../model/storyline-edits'

/** One scene as the storyline space shows it (CLIP-178): a still frame at the scene's source start
 *  where the session holds the footage — or the retained original does — and otherwise the scene's
 *  number and what was observed in it. A missing frame is silence, never a refusal. */
export function ClipSceneFrame({
  scene,
  sceneId,
  localSources,
  resolvePlayback,
}: {
  scene: ClipScene | undefined
  /** Shown when the scene is not among the observations this read carries. */
  sceneId: string
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
}) {
  const { t } = useTranslation('clips')
  const fingerprint = scene?.fingerprint
  const local = fingerprint
    ? localSources.find((source) => source.fingerprint === fingerprint)?.url
    : undefined
  const [retained, setRetained] = useState<{ fingerprint: string; url: string }>()
  useEffect(() => {
    let active = true
    if (local || !resolvePlayback || !fingerprint) return
    resolvePlayback(fingerprint).then(
      (url) => {
        if (active) setRetained({ fingerprint, url })
      },
      () => {},
    )
    return () => {
      active = false
    }
  }, [local, fingerprint, resolvePlayback])
  const url = local ?? (retained && retained.fingerprint === fingerprint ? retained.url : undefined)
  const name = scene ? t('storylineSpace.scene', { n: scene.number }) : sceneId
  return (
    <figure className="bg-surface-sunken flex w-28 flex-col overflow-hidden rounded-md">
      {url && scene ? (
        <video
          key={url}
          muted
          playsInline
          preload="metadata"
          src={url}
          aria-label={t('storylineSpace.frame', { scene: name })}
          className="aspect-video w-full object-cover"
          onLoadedMetadata={(event) => {
            event.currentTarget.currentTime = Math.max(0, scene.startMs / 1000)
          }}
        />
      ) : null}
      <figcaption className="px-2 py-1">
        <Typography variant="meta" as="span" className="block truncate">
          {name}
        </Typography>
        {!url && scene?.event && (
          <Typography
            variant="meta"
            as="span"
            className="text-content-secondary line-clamp-2 block"
          >
            {scene.event}
          </Typography>
        )}
      </figcaption>
    </figure>
  )
}
