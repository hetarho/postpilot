import { useId, useState } from 'react'
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
}: {
  upload: ReturnType<typeof useClipSourceUpload>
  disabled?: boolean
  processing?: boolean
  correction?: boolean
}) {
  const { t } = useTranslation('clips')
  const inputId = useId()
  const busy = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  const locked = disabled || !!upload.attempt
  const error = upload.error
  const [selected, setSelected] = useState('')
  const selectedEntry =
    upload.entries.find((entry) => entry.metadata.fingerprint === selected) ?? upload.entries[0]
  return (
    <section aria-labelledby={`${inputId}-heading`} className="mt-10 space-y-4">
      <Typography variant="title" id={`${inputId}-heading`}>
        {t('source.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary" id={`${inputId}-disclosure`}>
        {t(correction ? 'correction.sourceDisclosure' : 'source.disclosure')}
      </Typography>
      {disabled && !processing && (
        <Typography variant="body" className="text-content-secondary">
          {t(correction ? 'correction.saveFirst' : 'source.saveFirst')}
        </Typography>
      )}
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
          {t(upload.phase === 'ready' ? 'source.replace' : 'source.select')}
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
      {selectedEntry && (
        <>
          <ClipSourceStrip
            label={t('source.selected')}
            selected={selectedEntry.metadata.fingerprint}
            onSelect={setSelected}
            sources={upload.entries.map((entry) => ({
              ...entry.metadata,
              previewURL: entry.previewURL,
              status: entry.confirmed
                ? t('source.confirmed')
                : t('source.uploadProgress', { percent: entry.percent }),
            }))}
          />
          {!correction && (
            <video
              key={selectedEntry.previewURL}
              controls
              playsInline
              preload="metadata"
              src={selectedEntry.previewURL}
              width={selectedEntry.metadata.width}
              height={selectedEntry.metadata.height}
              aria-label={selectedEntry.metadata.filename}
              className="aspect-video w-full rounded-md object-contain"
            />
          )}
          {!selectedEntry.confirmed && (
            <ProgressBar
              label={selectedEntry.metadata.filename}
              done={selectedEntry.percent}
              total={100}
            />
          )}
        </>
      )}
      {!!upload.summaries?.length && (
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
