import { useCallback, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  ClipSourceStrip,
  observationCutUsage,
  observationSummary,
  type ClipEditPlan,
  type ClipObservedSegment,
  type ClipProject,
  type ClipSourceObservation,
} from '@/entities/clip-project'
import { formatDuration } from '@/shared/lib/video'
import { Button, Sheet, Typography } from '@/shared/ui'

function timecode(ms: number) {
  const fraction = ms % 1000
  return `${formatDuration(ms - fraction)}${fraction ? `.${String(fraction).padStart(3, '0')}` : ''}`
}
function range(startMs: number, endMs: number) {
  return `${timecode(startMs)}–${timecode(endMs)}`
}

function ObservationRange({
  segment,
  observation,
  plan,
  onPreview,
}: {
  segment: ClipObservedSegment
  observation: ClipSourceObservation
  plan?: ClipEditPlan
  onPreview?: () => void
}) {
  const { t } = useTranslation('clips')
  const usage = observationCutUsage(segment, observation.source, plan)
  return (
    <li className="space-y-3 py-4">
      <Typography variant="fieldTitle" as="h4">
        {range(segment.startMs, segment.endMs)}
      </Typography>
      <dl className="space-y-3">
        {(
          [
            ['event', segment.event],
            ['subjects', segment.subjects.join(', ')],
            ['speech', segment.speech],
            ['quality', segment.quality],
          ] as const
        ).map(([field, value]) => (
          <div key={field}>
            <Typography variant="label" as="dt" className="text-content-secondary">
              {t(`observation.${field}`)}
            </Typography>
            <Typography variant="body" as="dd" className="break-words whitespace-pre-wrap">
              {value || t('observation.notRecorded')}
            </Typography>
          </div>
        ))}
      </dl>
      {usage.length ? (
        <ul className="space-y-1" aria-label={t('observation.usedRanges')}>
          {usage.map((cut) => (
            <li key={cut.cutId}>
              <Typography variant="body">
                {t('observation.usedCut', {
                  number: cut.number,
                  range: range(cut.startMs, cut.endMs),
                })}
              </Typography>
            </li>
          ))}
        </ul>
      ) : (
        <Typography variant="body" className="text-content-secondary">
          {t(plan ? 'observation.unused' : 'observation.noPlan')}
        </Typography>
      )}
      {onPreview && (
        <Button variant="ghost" onClick={onPreview}>
          {t('observation.previewRange', { range: range(segment.startMs, segment.endMs) })}
        </Button>
      )}
    </li>
  )
}

function SourceObservations({
  observation,
  plan,
  localURL,
}: {
  observation: ClipSourceObservation
  plan?: ClipEditPlan
  localURL?: string
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const [expanded, setExpanded] = useState(false)
  const [preview, setPreview] = useState<ClipObservedSegment>()
  const closePreview = useCallback(() => setPreview(undefined), [])
  const [previewSource, setPreviewSource] = useState(localURL)
  if (previewSource !== localURL) {
    setPreviewSource(localURL)
    setPreview(undefined)
  }
  const summary = observationSummary(observation)
  return (
    <div className="space-y-4">
      <Typography variant="fieldTitle" className="break-all">
        {observation.source.filename}
      </Typography>
      <Typography variant="body" className="line-clamp-2 break-words whitespace-pre-wrap">
        {summary || t('observation.noSegments')}
      </Typography>
      {!localURL && (
        <Typography variant="body" className="text-content-secondary">
          {t('observation.sourceMissing')}
        </Typography>
      )}
      {observation.segments.length > 0 && (
        <>
          <Button
            variant="ghost"
            aria-expanded={expanded}
            aria-controls={`${id}-ranges`}
            onClick={() => setExpanded(!expanded)}
          >
            {t(expanded ? 'observation.hideDetails' : 'observation.showDetails', {
              count: observation.segments.length,
            })}
          </Button>
          <ol id={`${id}-ranges`} hidden={!expanded} aria-label={t('observation.ranges')}>
            {observation.segments.map((segment, index) => (
              <ObservationRange
                key={index}
                segment={segment}
                observation={observation}
                plan={plan}
                onPreview={localURL ? () => setPreview(segment) : undefined}
              />
            ))}
          </ol>
        </>
      )}
      <Sheet
        open={!!preview && !!localURL}
        labelledBy={`${id}-preview-title`}
        header={
          <Typography variant="title" id={`${id}-preview-title`}>
            {t('observation.previewTitle')}
          </Typography>
        }
        onClose={closePreview}
        footer={
          <Button variant="secondary" onClick={closePreview}>
            {t('observation.closePreview')}
          </Button>
        }
      >
        {preview && localURL && (
          <video
            key={`${localURL}-${preview.startMs}-${preview.endMs}`}
            src={localURL}
            controls
            tabIndex={0}
            playsInline
            preload="metadata"
            aria-label={t('observation.previewTitle')}
            className="aspect-video w-full rounded-md object-contain"
            onLoadedMetadata={(event) => {
              event.currentTarget.currentTime = preview.startMs / 1000
            }}
            onTimeUpdate={(event) => {
              if (event.currentTarget.currentTime >= preview.endMs / 1000) {
                event.currentTarget.pause()
              }
            }}
            onPlay={(event) => {
              if (
                event.currentTarget.currentTime >= preview.endMs / 1000 ||
                event.currentTarget.currentTime < preview.startMs / 1000
              )
                event.currentTarget.currentTime = preview.startMs / 1000
            }}
          />
        )}
      </Sheet>
    </div>
  )
}

export function ClipObservationViewer({
  project,
  localSources,
}: {
  project: ClipProject
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const [selected, setSelected] = useState('')
  const observations = project.observations
  const sources = observations?.status === 'available' ? observations.sources : []
  const active = sources.find((item) => item.source.fingerprint === selected) ?? sources[0]
  const job = project.latestJob
  const previous =
    job?.kind === 'generate_clip' && ['queued', 'running', 'failed'].includes(job.status)
  const savedPlan = !project.result || project.editPlanRevision !== project.renderedPlanRevision
  return (
    <section aria-labelledby={`${id}-heading`} className="mt-10 mb-8 min-w-0 space-y-4">
      <Typography variant="title" id={`${id}-heading`}>
        {t('observation.title')}
      </Typography>
      <Typography variant="body" className="text-content-secondary">
        {t('observation.description')}
      </Typography>
      {!active ? (
        <Typography variant="body" className="text-content-secondary">
          {t(
            observations?.status === 'unavailable'
              ? 'observation.unavailable'
              : 'observation.empty',
          )}
        </Typography>
      ) : (
        <>
          {previous && (
            <Typography variant="body" className="text-content-secondary">
              {t('observation.previous')}
            </Typography>
          )}
          <Typography variant="body">
            {t(savedPlan ? 'observation.savedPlan' : 'observation.renderedPlan')}
          </Typography>
          <ClipSourceStrip
            label={t('observation.sources')}
            sources={sources.map(({ source, segments }) => ({
              ...source,
              previewURL: localSources.find((local) => local.fingerprint === source.fingerprint)
                ?.url,
              status: t('observation.segmentCount', { count: segments.length }),
            }))}
            selected={active.source.fingerprint}
            onSelect={setSelected}
          />
          <SourceObservations
            key={active.source.fingerprint}
            observation={active}
            plan={project.editing?.plan}
            localURL={
              localSources.find((local) => local.fingerprint === active.source.fingerprint)?.url
            }
          />
        </>
      )}
    </section>
  )
}
