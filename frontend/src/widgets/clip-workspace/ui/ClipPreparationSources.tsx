import { Typography } from '@/shared/ui'
import { useTranslation } from 'react-i18next'
import type { useClipWorkspace } from '../model/useClipWorkspace'

type WorkspaceSources = ReturnType<typeof useClipWorkspace>['sources']
type SummarySources = {
  upload: Pick<WorkspaceSources['upload'], 'entries'>
  binding: Pick<WorkspaceSources['binding'], 'items' | 'value'>
  sound: Pick<WorkspaceSources['sound'], 'value'>
}

/** Source order, readiness, owner audio choices and optional bindings from their current owner. */
export function ClipPreparationSources({
  sources,
  ready,
  now,
}: {
  sources: SummarySources
  ready: boolean
  now: number
}) {
  const { t } = useTranslation('clips')
  const entries = sources.upload.entries
  const unavailable = (entry: (typeof entries)[number]) =>
    ['expired', 'missing', 'cleanup_pending'].includes(entry.availability ?? '') ||
    (!!entry.retentionExpiresAt && Date.parse(entry.retentionExpiresAt) <= now)
  const readyCount = entries.filter((entry) => entry.confirmed && !unavailable(entry)).length
  return (
    <>
      <Typography variant="body" as="p" className="break-words">
        {t('guide.sourceCount', { count: entries.length, ready: readyCount })}
      </Typography>
      {!ready && (
        <Typography variant="body" as="p" className="break-words">
          {t(entries.length ? 'guide.sourceRefusal' : 'guide.noSources')}
        </Typography>
      )}
      <ol className="mt-3 space-y-3">
        {entries.map((entry, index) => {
          const source = {
            batchId: entry.batchId ?? '',
            sourceId: entry.sourceId ?? '',
            fingerprint: entry.metadata.fingerprint,
            retainOriginalAudio: entry.retainOriginalAudio ?? false,
          }
          const item = sources.binding.items.find(
            (item) => item.value === sources.binding.value(source),
          )
          const missing = unavailable(entry)
          return (
            <li
              key={entry.metadata.fingerprint}
              className="bg-card-well min-w-0 space-y-1 rounded-lg p-4"
            >
              <Typography variant="body" as="p" className="break-words">
                {t('guide.sourceItem', {
                  position: index + 1,
                  filename: entry.metadata.filename,
                  // Probed durations are fractional milliseconds; review reads tenths.
                  seconds: Math.round(entry.metadata.durationMs / 100) / 10,
                  status: t(
                    missing
                      ? 'guide.sourceUnavailable'
                      : entry.confirmed
                        ? 'guide.sourceReady'
                        : 'guide.sourcePending',
                  ),
                  sound: t(sources.sound.value(source) ? 'guide.soundOn' : 'guide.soundOff'),
                })}
              </Typography>
              <Typography variant="body" as="p" className="break-words">
                {item ? t('guide.binding', { item: item.label }) : t('guide.noBinding')}
              </Typography>
            </li>
          )
        })}
      </ol>
    </>
  )
}
