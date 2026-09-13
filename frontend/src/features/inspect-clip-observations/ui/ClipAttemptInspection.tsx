import { useEffect, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipProject } from '@/entities/clip-project'
import { Button, Typography } from '@/shared/ui'
import { ClipObservationViewer } from './ClipObservationViewer'

type Inspection = NonNullable<ClipProject['attemptInspection']>
type Candidate = Inspection['ranges'][number]

const observationChecks = {
  input_prompt_limit: 'inspection.inputTooLarge',
  input_settings: 'inspection.inputInvalid',
  input_sources: 'inspection.inputInvalid',
  observe_source_identity: 'inspection.observationIdentity',
  observe_chunk_identity: 'inspection.observationIdentity',
  observe_segment_fields: 'inspection.observationFields',
  observe_segment_count: 'inspection.observationCount',
  observe_segment_time: 'inspection.observationTime',
  observe_segment_overlap: 'inspection.observationOverlap',
  observe_focal: 'inspection.observationGeometry',
  observe_subject_bounds: 'inspection.observationGeometry',
  observe_text_length: 'inspection.observationLength',
  observe_quality: 'inspection.observationQuality',
  observe_subject_count: 'inspection.observationCount',
  observe_subject_text: 'inspection.observationFields',
  observe_description: 'inspection.observationDescription',
  observe_scene: 'inspection.observationScene',
  observe_silent_speech: 'inspection.observationAudio',
} as const

function CandidatePlayback({
  range,
  source,
  localURL,
  resolvePlayback,
}: {
  range: Candidate
  source: Inspection['observations']['sources'][number]['source']
  localURL?: string
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
}) {
  const { t } = useTranslation('clips')
  const [remoteURL, setRemoteURL] = useState('')
  const [failed, setFailed] = useState(false)
  const url = localURL || remoteURL
  useEffect(() => {
    let active = true
    if (!localURL && resolvePlayback) {
      resolvePlayback(source.fingerprint).then(
        (next) => {
          if (active) setRemoteURL(next)
        },
        () => {
          if (active) setFailed(true)
        },
      )
    }
    return () => {
      active = false
    }
  }, [localURL, resolvePlayback, source.fingerprint])
  return (
    <div className="min-w-0 space-y-2">
      <Typography variant="body" className="break-all">
        {t('inspection.playing', {
          cut: range.cut,
          name: source.filename,
          start: range.startMs / 1000,
          end: range.endMs / 1000,
        })}
      </Typography>
      {url && !failed ? (
        <video
          className="bg-surface-lowest max-h-64 w-full rounded-lg"
          aria-label={t('inspection.originalRange')}
          src={url}
          controls
          playsInline
          preload="metadata"
          onLoadedMetadata={(e) => {
            e.currentTarget.currentTime = range.startMs / 1000
          }}
          onPlay={(e) => {
            if (
              e.currentTarget.currentTime < range.startMs / 1000 ||
              e.currentTarget.currentTime >= range.endMs / 1000
            )
              e.currentTarget.currentTime = range.startMs / 1000
          }}
          onTimeUpdate={(e) => {
            if (e.currentTarget.currentTime >= range.endMs / 1000) e.currentTarget.pause()
          }}
          onError={() => setFailed(true)}
        />
      ) : (
        <Typography variant="body" className="text-content-secondary" role="status">
          {t(failed || !resolvePlayback ? 'inspection.mediaMissing' : 'inspection.mediaLoading')}
        </Typography>
      )}
    </div>
  )
}

export function ClipAttemptInspection({
  project,
  currentJobId,
  localSources,
  resolvePlayback,
}: {
  project: ClipProject
  currentJobId?: string
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const [selected, setSelected] = useState<number>()
  const job = project.latestJob
  const inspection =
    project.attemptInspection?.jobId === job?.id ? project.attemptInspection : undefined
  if (
    project.finalized ||
    !job ||
    !['failed', 'cancelled'].includes(job.status) ||
    (currentJobId && currentJobId !== job.id)
  )
    return null
  const available = inspection?.status === 'available'
  const active = available
    ? inspection.ranges.find((r) => r.cut === selected && r.valid)
    : undefined
  const source = active ? inspection?.observations.sources[active.source - 1]?.source : undefined
  const phase = inspection?.validationPhase
  const check = inspection?.validationCheck
  const observationExplanation = check
    ? observationChecks[check as keyof typeof observationChecks]
    : undefined
  const explanation =
    observationExplanation ??
    (phase === 'timeline_grow'
      ? 'inspection.tooShort'
      : phase === 'timeline_shrink'
        ? 'inspection.cannotTrim'
        : phase === 'timeline_total'
          ? 'inspection.lengthMismatch'
          : check === 'composition_cut_identity' || check === 'plan_source'
            ? 'inspection.sourceInvalid'
            : check === 'composition_observation_gap' || check === 'composition_cut_evidence'
              ? 'inspection.evidenceGap'
              : check === 'composition_section_order' || check === 'composition_item_order'
                ? 'inspection.orderInvalid'
                : check === 'plan_cut_range'
                  ? 'inspection.rangeInvalid'
                  : check === 'plan_caption_time'
                    ? 'inspection.captionTimeInvalid'
                    : 'inspection.validationFailed')
  const stage = job.stage
  const knownStage =
    stage && ['prepare', 'analyze', 'plan', 'render', 'save', 'cleanup'].includes(stage)
      ? (stage as 'prepare' | 'analyze' | 'plan' | 'render' | 'save' | 'cleanup')
      : undefined
  return (
    <section aria-labelledby={`${id}-title`} className="mt-8 min-w-0 space-y-4">
      <Typography variant="title" id={`${id}-title`}>
        {t('inspection.title')}
      </Typography>
      <Typography variant="body">
        {t(job.status === 'cancelled' ? 'inspection.cancelledHelp' : 'inspection.help')}
      </Typography>
      {knownStage && (
        <Typography variant="body">
          {t('inspection.stoppedAt', { stage: t(`generation.stage.${knownStage}`) })}
        </Typography>
      )}
      {!available ? (
        <Typography variant="body" className="text-content-secondary">
          {t(
            inspection?.status === 'unavailable' ? 'inspection.unavailable' : 'inspection.missing',
          )}
        </Typography>
      ) : (
        <>
          <Typography variant="body">
            {job.kind === 'render_clip'
              ? t('inspection.manualHelp')
              : t('inspection.progress', {
                  done: inspection.completedChunks,
                  total: inspection.totalChunks,
                })}
          </Typography>
          {inspection.evidenceLimited && (
            <Typography variant="body">{t('inspection.limited')}</Typography>
          )}
          {job.status === 'failed' && (
            <>
              <Typography variant="body">{t(explanation)}</Typography>
              {(!check || check === 'unknown') && (
                <Typography variant="body" className="text-content-secondary">
                  {t('inspection.detailUnknown')}
                </Typography>
              )}
            </>
          )}
          <dl className="grid grid-cols-1 gap-x-4 gap-y-2 sm:grid-cols-2">
            {(
              [
                'input_bytes',
                'input_limit_bytes',
                'target_ms',
                'before_ms',
                'after_ms',
                'remaining_ms',
                'cut',
                'source',
                'chunk',
                'segment',
                'segment_count',
                'duration_ms',
                'raw_start_ms',
                'raw_end_ms',
                'focal_x_ppm',
                'focal_y_ppm',
                'subject_x_ppm',
                'subject_y_ppm',
                'subject_width_ppm',
                'subject_height_ppm',
              ] as const
            ).map((key) => {
              const value = inspection.measurements[key]
              return value === undefined ? null : (
                <div key={key} className="min-w-0">
                  <Typography as="dt" variant="label" className="text-content-secondary">
                    {t(`inspection.measurements.${key}`)}
                  </Typography>
                  <Typography as="dd" variant="body">
                    {key.endsWith('_ms')
                      ? t('inspection.seconds', { value: value / 1000 })
                      : key.endsWith('_ppm')
                        ? t('inspection.percent', { value: value / 10000 })
                        : key.endsWith('_bytes')
                          ? t('inspection.bytes', { value })
                          : value}
                  </Typography>
                </div>
              )
            })}
          </dl>
          <Typography variant="fieldTitle">{t('inspection.ranges')}</Typography>
          <Typography variant="body" className="text-content-secondary">
            {t('inspection.rangeHelp')}
          </Typography>
          {inspection.ranges.length ? (
            <ul className="flex flex-wrap gap-2">
              {inspection.ranges.map((range) => (
                <li key={range.cut}>
                  <Button
                    variant={active?.cut === range.cut ? 'secondary' : 'ghost'}
                    disabled={!range.valid}
                    aria-pressed={active?.cut === range.cut}
                    onClick={() => setSelected(range.cut)}
                  >
                    {t(range.valid ? 'inspection.cut' : 'inspection.invalidCut', {
                      number: range.cut,
                    })}
                  </Button>
                </li>
              ))}
            </ul>
          ) : (
            <Typography variant="body">{t('inspection.noRanges')}</Typography>
          )}
          {active && source && (
            <CandidatePlayback
              key={`${job.id}-${active.cut}`}
              range={active}
              source={source}
              localURL={localSources.find((s) => s.fingerprint === source.fingerprint)?.url}
              resolvePlayback={resolvePlayback}
            />
          )}
          {job.kind !== 'render_clip' && inspection.observations.sources.length > 0 && (
            <ClipObservationViewer
              key={job.id}
              project={project}
              attemptObservations={inspection.observations}
              localSources={localSources}
              resolvePlayback={resolvePlayback}
            />
          )}
        </>
      )}
    </section>
  )
}
