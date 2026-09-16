import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { appFailureFromConnect } from '@/shared/api'
import { CLIP_SOURCE_CONTAINERS } from '@/shared/config'
import { DirectUploadError } from '@/shared/lib/upload'
import { ClipSourceStrip } from '@/entities/clip-project'
import { AppFailureMessage, Button, ProgressBar, Typography, buttonStyles } from '@/shared/ui'
import { ClipSelectionError } from '../model/manifest'
import { ClipSourceMismatchError } from '../model/reselection'
import type { useClipSourceUpload } from '../model/useClipSourceUpload'

const ACCEPT = [
  ...Object.keys(CLIP_SOURCE_CONTAINERS).map((v) => `.${v}`),
  ...new Set(Object.values(CLIP_SOURCE_CONTAINERS).flat()),
].join(',')
export function ClipSourcePicker({
  upload,
  disabled,
  processing = false,
  correction = false,
  readOnly = false,
  binding,
  sound,
}: {
  upload: ReturnType<typeof useClipSourceUpload>
  disabled?: boolean
  processing?: boolean
  correction?: boolean
  readOnly?: boolean
  /** Binding one whole source to one item before generation (CLIP-123). Absent
   *  where the project declares no group, or on a read-only surface. */
  binding?: {
    items: readonly { value: string; label: string }[]
    value: (source: { sourceId: string; fingerprint: string }) => string
    change: (
      source: { sourceId: string; fingerprint: string; durationMs: number },
      item: string,
    ) => void
  }
  sound?: {
    value: (source: {
      batchId: string
      sourceId: string
      fingerprint: string
      retainOriginalAudio: boolean
    }) => boolean
    change: (
      source: {
        batchId: string
        sourceId: string
        fingerprint: string
        retainOriginalAudio: boolean
      },
      enabled: boolean,
    ) => void
    disabled?: boolean
    pending?: boolean
    failed?: boolean
    retry: () => void
  }
}) {
  const { t } = useTranslation('clips')
  const inputId = useId()
  const busy = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  const locked = disabled || !!upload.attempt
  const error = upload.error
  const [selected, setSelected] = useState('')
  const [now, setNow] = useState(Date.now)
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined
    const refresh = () => {
      const time = Date.now()
      setNow(time)
      const expires = upload.entries
        .map((e) => Date.parse(e.retentionExpiresAt ?? ''))
        .filter((at) => at > time)
      if (!expires.length) return
      const delay = Math.min(...expires) - time
      if (delay <= 2 ** 31 - 1) timer = setTimeout(refresh, delay)
    }
    refresh()
    return () => clearTimeout(timer)
  }, [upload.entries])
  const [loadingSources, setLoadingSources] = useState(readOnly)
  const [failedPreview, setFailedPreview] = useState<string>()
  const refreshRetained = upload.refreshRetained
  useEffect(() => {
    if (!readOnly) return
    let active = true
    void refreshRetained().finally(() => {
      if (active) setLoadingSources(false)
    })
    return () => {
      active = false
    }
  }, [readOnly, refreshRetained])
  const initialEntry = readOnly
    ? (upload.entries.find((entry) => entry.previewURL || entry.availability === 'available') ??
      upload.entries[0])
    : upload.entries[0]
  const selectedEntry =
    upload.entries.find((entry) => entry.metadata.fingerprint === selected) ?? initialEntry
  const restoreTime = useRef(0)
  const fingerprint = selectedEntry?.metadata.fingerprint
  const ensurePlayback = upload.ensurePlayback
  useEffect(() => {
    restoreTime.current = 0
    if (fingerprint) void ensurePlayback(fingerprint).catch(() => {})
  }, [fingerprint, ensurePlayback])
  const player =
    !correction && selectedEntry?.previewURL ? (
      <video
        key={selectedEntry.previewURL}
        controls
        playsInline
        preload="metadata"
        src={selectedEntry.previewURL}
        onLoadedMetadata={(event) => {
          if (restoreTime.current) event.currentTarget.currentTime = restoreTime.current
        }}
        onError={(event) => {
          restoreTime.current = event.currentTarget.currentTime
          setFailedPreview(selectedEntry.previewURL)
          void upload.ensurePlayback(selectedEntry.metadata.fingerprint, true).catch(() => {})
        }}
        width={selectedEntry.metadata.width}
        height={selectedEntry.metadata.height}
        aria-label={selectedEntry.metadata.filename}
        className={`bg-media-canvas-bg aspect-video w-full rounded-md object-contain ${readOnly ? 'max-h-48' : ''}`}
      />
    ) : null
  return (
    <section
      aria-labelledby={`${inputId}-heading`}
      className={readOnly ? 'min-w-0 space-y-3' : 'mt-10 space-y-4'}
    >
      <Typography variant={readOnly ? 'fieldTitle' : 'title'} id={`${inputId}-heading`}>
        {t(readOnly ? 'source.runningTitle' : 'source.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary" id={`${inputId}-disclosure`}>
        {t(
          readOnly
            ? 'source.runningHelp'
            : correction
              ? 'correction.sourceDisclosure'
              : 'source.disclosure',
        )}
      </Typography>
      {!readOnly && disabled && !processing && (
        <Typography variant="body" className="text-content-secondary">
          {t(correction ? 'correction.saveFirst' : 'source.saveFirst')}
        </Typography>
      )}
      {!readOnly && (
        <div className="flex flex-wrap items-center gap-3">
          <input
            id={inputId}
            type="file"
            accept={ACCEPT}
            multiple
            disabled={locked || busy}
            aria-describedby={`${inputId}-disclosure`}
            className="peer/source sr-only"
            onChange={(event) => {
              const files = Array.from(event.target.files ?? [])
              event.target.value = ''
              void upload.select(files)
            }}
          />
          <label
            htmlFor={inputId}
            aria-disabled={locked || busy || undefined}
            className={buttonStyles({
              variant: 'secondary',
              className:
                'peer-focus-visible/source:outline-focus-ring peer-focus-visible/source:outline-2 peer-focus-visible/source:outline-offset-2',
            })}
          >
            {t(upload.entries.length ? 'source.replace' : 'source.select')}
          </label>
          {(busy || upload.phase === 'ready' || upload.phase === 'failed') && (
            <Button
              variant="ghost"
              pending={upload.phase === 'cancelling'}
              disabled={processing}
              onClick={() => void upload.cancel()}
            >
              {t('source.cancel')}
            </Button>
          )}
        </div>
      )}
      {error !== undefined && (
        <div role="alert">
          {error instanceof ClipSourceMismatchError ? (
            <>
              {!!error.missing.length && (
                <Typography variant="body" className="break-words">
                  {t('correction.missingSources', { names: error.missing.join(', ') })}
                </Typography>
              )}
              {!!error.unexpected.length && (
                <Typography variant="body" className="break-words">
                  {t('correction.unexpectedSources', { names: error.unexpected.join(', ') })}
                </Typography>
              )}
            </>
          ) : error instanceof ClipSelectionError ? (
            <Typography variant="body">{t(`source.error.${error.reason}`)}</Typography>
          ) : error instanceof DirectUploadError ? (
            <Typography variant="body">{t('source.error.network')}</Typography>
          ) : (
            <AppFailureMessage failure={appFailureFromConnect(error)} />
          )}
        </div>
      )}
      {readOnly && !selectedEntry && !error && (
        <Typography variant="body">
          {t(loadingSources ? 'source.previewLoading' : 'source.previewMissing')}
        </Typography>
      )}
      {selectedEntry && (
        <>
          {readOnly && player}
          <ClipSourceStrip
            label={t('source.selected')}
            selected={selectedEntry.metadata.fingerprint}
            onSelect={setSelected}
            onSoundChange={
              !readOnly && sound
                ? (fingerprint, enabled) => {
                    const entry = upload.entries.find((e) => e.metadata.fingerprint === fingerprint)
                    if (
                      !entry?.sourceId ||
                      !entry.batchId ||
                      !entry.current ||
                      sound.disabled ||
                      busy ||
                      upload.attempt ||
                      entry.availability !== 'available' ||
                      !entry.retentionExpiresAt ||
                      Date.parse(entry.retentionExpiresAt) <= Date.now()
                    )
                      return
                    sound.change(
                      {
                        batchId: entry.batchId,
                        sourceId: entry.sourceId,
                        fingerprint,
                        retainOriginalAudio: entry.retainOriginalAudio ?? false,
                      },
                      enabled,
                    )
                  }
                : undefined
            }
            items={binding?.items}
            onItemChange={
              binding && !readOnly
                ? (fingerprint, item) => {
                    const entry = upload.entries.find((e) => e.metadata.fingerprint === fingerprint)
                    if (!entry?.sourceId || !entry.current) return
                    binding.change(
                      {
                        sourceId: entry.sourceId,
                        fingerprint,
                        durationMs: entry.metadata.durationMs,
                      },
                      item,
                    )
                  }
                : undefined
            }
            sources={upload.entries.map((entry) => ({
              ...entry.metadata,
              previewURL: entry.previewURL,
              boundItem:
                binding && entry.sourceId
                  ? binding.value({
                      sourceId: entry.sourceId,
                      fingerprint: entry.metadata.fingerprint,
                    })
                  : undefined,
              retainOriginalAudio:
                entry.sourceId && entry.batchId && sound
                  ? sound.value({
                      batchId: entry.batchId,
                      sourceId: entry.sourceId,
                      fingerprint: entry.metadata.fingerprint,
                      retainOriginalAudio: entry.retainOriginalAudio ?? false,
                    })
                  : (entry.retainOriginalAudio ?? false),
              soundDisabled:
                sound?.disabled ||
                busy ||
                !!upload.attempt ||
                !entry.current ||
                !entry.confirmed ||
                !entry.sourceId ||
                !entry.batchId ||
                entry.availability !== 'available' ||
                !entry.retentionExpiresAt ||
                Date.parse(entry.retentionExpiresAt) <= now,
              status: readOnly
                ? undefined
                : entry.confirmed
                  ? t('source.confirmed')
                  : t('source.uploadProgress', { percent: entry.percent }),
            }))}
          />
          {!readOnly && sound?.failed && (
            <div role="alert" className="space-y-2">
              <Typography variant="body">{t('source.soundFailed')}</Typography>
              <Button variant="secondary" pending={sound.pending} onClick={sound.retry}>
                {t('source.soundRetry')}
              </Button>
            </div>
          )}
          {!readOnly && selectedEntry.retentionExpiresAt && (
            <Typography variant="meta" className="text-content-secondary">
              {t('source.retainedUntil', {
                time: new Date(selectedEntry.retentionExpiresAt).toLocaleString(),
              })}
            </Typography>
          )}
          {(selectedEntry.playbackError ||
            (readOnly && !!failedPreview && failedPreview === selectedEntry.previewURL)) && (
            <Typography variant="body" role="status">
              {t(
                readOnly
                  ? 'source.previewUnavailable'
                  : `source.access.${selectedEntry.playbackError ?? 'unavailable'}`,
              )}
            </Typography>
          )}
          {readOnly && !selectedEntry.previewURL && !selectedEntry.playbackError && (
            <Typography variant="body">{t('source.previewLoading')}</Typography>
          )}
          {!readOnly && player}
          {!readOnly && !selectedEntry.confirmed && (
            <ProgressBar
              label={selectedEntry.metadata.filename}
              done={selectedEntry.percent}
              total={100}
            />
          )}
        </>
      )}
      {!readOnly && !!upload.summaries?.length && (
        <ul
          className="flex snap-x snap-mandatory gap-4 overflow-x-auto overscroll-x-contain pb-2"
          aria-label={t('source.completed')}
        >
          {upload.summaries.map((entry, index) => (
            <li key={index} className="w-60 min-w-0 shrink-0 snap-start">
              <Typography variant="body" className="break-words">
                {entry.filename} · {t(`source.outcome.${entry.status}`)}
              </Typography>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
