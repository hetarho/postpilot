import { useEffect, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { ClipProject } from '@/entities/clip-project'
import { Button, Typography } from '@/shared/ui'
import { formatAppFailure } from '@/shared/lib/localization'
import { ClipObservationViewer } from './ClipObservationViewer'

type Inspection = NonNullable<ClipProject['attemptInspection']>
type Candidate = Inspection['ranges'][number]

const checkExplanations = {
  // The RENDER's own refusal: a preset or a caption style it could not draw.
  // The template grammar has no such reason any more — a template names no
  // design (CLIP-14).
  composition_invalid_design: 'composition.errors.invalid_design',
  composition_invalid_skeleton: 'composition.errors.invalid_skeleton',
  composition_unknown_attribute: 'composition.errors.unknown_attribute',
  intro_slot_shortened: 'notices.introSlotShortened',
  outro_slot_shortened: 'notices.outroSlotShortened',
  intro_slot_omitted: 'notices.introSlotOmitted',
  outro_slot_omitted: 'notices.outroSlotOmitted',
  composition_text_shortened: 'notices.textShortened',
  composition_text_omitted: 'notices.longTextOmitted',
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
  observe_coverage_start: 'inspection.observationCoverage',
  observe_coverage_gap: 'inspection.observationCoverage',
  observe_coverage_end: 'inspection.observationCoverage',
  observe_status: 'inspection.observationStatus',
  // Render substages and the delivered properties checked after encoding.
  render_layout: 'inspection.renderLayout',
  render_footage: 'inspection.renderFootage',
  render_audio: 'inspection.renderAudio',
  render_overlay: 'inspection.renderOverlay',
  render_encode: 'inspection.renderEncode',
  render_validate: 'inspection.renderValidate',
  render_output_canvas: 'inspection.outputCanvas',
  render_output_rotation: 'inspection.outputRotation',
  render_output_pixel_format: 'inspection.outputPixelFormat',
  render_output_aspect: 'inspection.outputAspect',
  render_output_frame_rate: 'inspection.outputFrameRate',
  render_output_audio: 'inspection.outputAudioTrack',
  render_output_duration: 'inspection.outputDuration',
  render_output_codec: 'inspection.outputCodec',
  render_output_audio_rate: 'inspection.outputAudioRate',
  composition_cut_identity: 'inspection.sourceInvalid',
  plan_source: 'inspection.sourceInvalid',
  plan_cut_identity: 'inspection.sourceInvalid',
  composition_observation_gap: 'inspection.evidenceGap',
  composition_cut_evidence: 'inspection.evidenceGap',
  composition_section_order: 'inspection.orderInvalid',
  composition_item_order: 'inspection.orderInvalid',
  plan_cut_range: 'inspection.rangeInvalid',
  plan_caption_time: 'inspection.captionTimeInvalid',
  composition_invalid_style: 'inspection.compositionStyle',
  plan_style: 'inspection.compositionStyle',
  composition_invalid_position: 'inspection.compositionPosition',
  composition_invalid_interval: 'inspection.compositionInterval',
  composition_invalid_rows: 'inspection.compositionRows',
  composition_generated_rows: 'inspection.compositionRows',
  composition_invalid_role: 'inspection.compositionRole',
  composition_copy_limit: 'inspection.compositionCopyLimit',
  composition_generated_bounds: 'inspection.compositionCopyLimit',
  composition_readability: 'inspection.compositionReadability',
  composition_caption_overlap: 'inspection.captionOverlap',
  composition_safe_area: 'inspection.compositionSafeArea',
  composition_invalid_manifest: 'inspection.compositionManifest',
  composition_generated_identity: 'inspection.compositionIdentity',
  composition_plan_bounds: 'inspection.compositionBounds',
  caption_measurement: 'inspection.captionMeasurement',
  output_encoding_or_size: 'inspection.responseEncoding',
  output_field_type: 'inspection.responseFields',
  output_shape: 'inspection.responseFields',
  output_json: 'inspection.responseJSON',
  plan_accent: 'inspection.planAccent',
  plan_required: 'inspection.planFields',
  plan_cut_fields: 'inspection.planFields',
  plan_caption_fields: 'inspection.planFields',
  plan_chip_count: 'inspection.planChipCount',
  plan_chip_label: 'inspection.planChipLabel',
  plan_copy_chars: 'inspection.planCopyChars',
  plan_copy_classes: 'inspection.planCopyClasses',
  plan_copy_count: 'inspection.planCopyCount',
  plan_copy_exposure: 'inspection.planCopyExposure',
  plan_copy_format: 'inspection.planCopyFormat',
  plan_copy_keyword: 'inspection.planCopyKeyword',
  plan_copy_lines: 'inspection.planCopyLines',
  plan_copy_second_cut: 'inspection.planCopySecondCut',
  plan_copy_sequence: 'inspection.planCopySequence',
  plan_cut_count: 'inspection.planCutCount',
  plan_cut_fade: 'inspection.planCutFade',
  plan_cut_transition: 'inspection.planCutTransition',
  plan_duration_limit: 'inspection.planDuration',
  plan_duration_range: 'inspection.planDuration',
  plan_focal: 'inspection.planFocal',
  plan_hook: 'inspection.planHook',
  plan_ratio: 'inspection.planRatio',
  plan_source_metadata: 'inspection.planSourceMetadata',
  plan_target_duration: 'inspection.planTargetDuration',
  plan_timeline: 'inspection.lengthMismatch',
  // The floor's own check always means the same thing, whatever phase reached
  // it: the selected footage could not fill the minimum (CLIP-120).
  plan_length_floor: 'inspection.tooShort',
  plan_volume: 'inspection.planVolume',
  plan_cut_rate: 'inspection.planCutRate',
  plan_cut_scene: 'inspection.planCutScene',
  plan_cut_usability: 'inspection.planCutUsability',
  plan_source_overlap: 'inspection.planSourceOverlap',
  plan_source_audio: 'inspection.planSourceAudio',
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
  const timelineExplanation =
    phase === 'timeline_grow'
      ? 'inspection.tooShort'
      : phase === 'timeline_shrink'
        ? 'inspection.cannotTrim'
        : phase === 'timeline_total'
          ? 'inspection.lengthMismatch'
          : undefined
  const namedExplanation =
    check === 'plan_timeline'
      ? (timelineExplanation ?? checkExplanations.plan_timeline)
      : check
        ? checkExplanations[check as keyof typeof checkExplanations]
        : undefined
  const explanation = namedExplanation ?? timelineExplanation ?? 'inspection.validationFailed'
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
              <Typography variant="body">
                {job.failure ? formatAppFailure(job.failure) : t(explanation)}
              </Typography>
              {/* A reason code says what to do next; the recorded check says which
                  property missed. The second is the only one a generic reason carries. */}
              {job.failure && namedExplanation && (
                <Typography variant="body" className="text-content-secondary">
                  {t(namedExplanation)}
                </Typography>
              )}
              {!job.failure && (!check || check === 'unknown') && (
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
                'width',
                'height',
                'expected_width',
                'expected_height',
                'rotation',
                'frame_rate_numerator',
                'frame_rate_denominator',
                'expected_fps',
                'video_frames',
                'expected_duration_ms',
                'decoded_duration_ms',
                'container_duration_ms',
                'audio_rate',
                'expected_audio_rate',
                'stream_index',
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
