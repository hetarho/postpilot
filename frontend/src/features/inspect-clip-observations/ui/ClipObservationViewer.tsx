import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { type ClipEditPlan } from '@/entities/clip-plan'
import {
  observationCutUsage,
  observationSummary,
  type ClipAddCutSelection,
  type ClipObservations,
  type ClipObservedSegment,
  type ClipSourceObservation,
} from '@/entities/clip-observation'
import { ClipSourceStrip, type ClipProject } from '@/entities/clip-project'
import { formatDuration } from '@/shared/lib/media'
import { Badge, Button, FieldLabel, FieldMessage, Sheet, Textarea, Typography } from '@/shared/ui'
import { AddObservedCut } from './AddObservedCut'
import {
  observationId,
  OBSERVATION_FIELDS,
  type ObservationField,
} from '../model/observation-edits'
import type { ClipObservationEditing } from '../model/useClipObservationEditing'

function timecode(ms: number) {
  const fraction = ms % 1000
  return `${formatDuration(ms - fraction)}${fraction ? `.${String(fraction).padStart(3, '0')}` : ''}`
}
function range(startMs: number, endMs: number) {
  return `${timecode(startMs)}–${timecode(endMs)}`
}

function ObservationRange({
  segment,
  index,
  observation,
  plan,
  onPreview,
  onAddCut,
  draftPlan,
  editing,
}: {
  segment: ClipObservedSegment
  index: number
  observation: ClipSourceObservation
  plan?: ClipEditPlan
  onPreview?: () => void
  onAddCut?: (selection: ClipAddCutSelection) => Promise<void>
  draftPlan?: ClipEditPlan
  editing?: ClipObservationEditing
}) {
  const { t } = useTranslation('clips')
  const fieldId = useId()
  const usage = observationCutUsage(segment, observation.source, plan)
  const id = observationId(observation.source.id, index)
  const editable = !!editing?.editable
  return (
    <li className="space-y-3 py-4">
      <div className="flex flex-wrap items-center gap-2">
        <Typography variant="fieldTitle" as="h4">
          {range(segment.startMs, segment.endMs)}
        </Typography>
        {segment.ownerEdited && <Badge>{t('observation.ownerEdited')}</Badge>}
      </div>
      {editable && editing && (
        // The descriptive text edits in place and autosaves (CLIP-235, CLIP-39); a scene observed
        // silent has no speech to correct, since audibility stays as observed.
        <div className="space-y-4">
          {OBSERVATION_FIELDS.filter((field) => field !== 'speech' || !!segment.speech.trim()).map(
            (field: ObservationField) => {
              const refused = editing.refused(id) === field
              return (
                <div key={field} className="space-y-2">
                  <FieldLabel htmlFor={`${fieldId}-${field}`}>
                    {t(field === 'subjects' ? 'observation.subjectsLines' : `observation.${field}`)}
                  </FieldLabel>
                  <Textarea
                    id={`${fieldId}-${field}`}
                    autoGrow
                    rows={field === 'subjects' ? 2 : 1}
                    value={editing.text(id, field, segment)}
                    aria-invalid={refused || undefined}
                    aria-describedby={refused ? `${fieldId}-${field}-error` : undefined}
                    onChange={(event) =>
                      editing.change(id, field, segment, event.currentTarget.value)
                    }
                  />
                  {refused && (
                    <FieldMessage id={`${fieldId}-${field}-error`}>
                      {t('observation.speechRequired')}
                    </FieldMessage>
                  )}
                </div>
              )
            },
          )}
        </div>
      )}
      <dl className="space-y-3">
        {(
          [
            ['event', segment.event],
            ['action', segment.action],
            ['motion', segment.motion],
            ['subjects', segment.subjects.join(', ')],
            ['speech', segment.speech],
            ['quality', segment.quality],
            // A record written before the v2 contract carries no status at all,
            // and says so: it is never shown as if it had been judged certain.
            ['certainty', t(`observation.certaintyValue.${segment.certainty}`)],
            ['usability', t(`observation.usabilityValue.${segment.usability}`)],
          ] as const
        )
          .filter(
            ([field]) =>
              !editable ||
              !(OBSERVATION_FIELDS as readonly string[]).includes(field) ||
              (field === 'speech' && !segment.speech.trim()),
          )
          .map(([field, value]) => (
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
                  rate: cut.playbackRatePermille / 1000,
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
      {onAddCut && (
        <AddObservedCut
          observation={observation}
          segment={segment}
          plan={draftPlan ?? plan}
          onAddCut={onAddCut}
        />
      )}
    </li>
  )
}

function SourceObservations({
  observation,
  plan,
  localURL,
  refreshPlayback,
  onAddCut,
  draftPlan,
  editing,
}: {
  observation: ClipSourceObservation
  plan?: ClipEditPlan
  localURL?: string
  refreshPlayback?: () => Promise<string>
  onAddCut?: (selection: ClipAddCutSelection) => Promise<void>
  draftPlan?: ClipEditPlan
  editing?: ClipObservationEditing
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const [expanded, setExpanded] = useState(false)
  const [preview, setPreview] = useState<ClipObservedSegment>()
  const closePreview = useCallback(() => setPreview(undefined), [])
  const restoreTime = useRef<number | undefined>(undefined)
  const summary = observationSummary(observation)
  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center gap-2">
        <Typography variant="fieldTitle" className="min-w-0 break-all">
          {observation.source.filename}
        </Typography>
        {observation.ownerEdited && <Badge>{t('observation.ownerEdited')}</Badge>}
      </div>
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
                index={index}
                editing={editing}
                segment={segment}
                observation={observation}
                plan={plan}
                onPreview={localURL ? () => setPreview(segment) : undefined}
                onAddCut={onAddCut}
                draftPlan={draftPlan}
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
              event.currentTarget.currentTime = restoreTime.current ?? preview.startMs / 1000
            }}
            onError={(event) => {
              restoreTime.current = event.currentTarget.currentTime
              void refreshPlayback?.().catch(() => {})
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
  resolvePlayback,
  attemptObservations,
  onAddCut,
  draftPlan,
  editing,
  headingAction,
}: {
  attemptObservations?: ClipObservations
  project: ClipProject
  /** ②'s in-place corrections (CLIP-235); absent where observations are only read. */
  editing?: ClipObservationEditing
  /** The heading row's action: ②'s 다시 관찰하기 (CLIP-234). */
  headingAction?: ReactNode
  localSources: ReadonlyArray<{ fingerprint: string; url: string }>
  resolvePlayback?: (fingerprint: string, refresh?: boolean) => Promise<string>
  onAddCut?: (selection: ClipAddCutSelection) => Promise<void>
  draftPlan?: ClipEditPlan
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  const [selected, setSelected] = useState('')
  const observations = attemptObservations ?? project.observations
  const sources = observations?.status === 'available' ? observations.sources : []
  const active = sources.find((item) => item.source.fingerprint === selected) ?? sources[0]
  const fingerprint = active?.source.fingerprint
  useEffect(() => {
    if (fingerprint) void resolvePlayback?.(fingerprint).catch(() => {})
  }, [fingerprint, resolvePlayback])
  const job = project.latestJob
  const previous =
    job?.kind === 'generate_clip' &&
    ['queued', 'running', 'failed', 'cancelled'].includes(job.status)
  const savedPlan = !project.result || project.editPlanRevision !== project.renderedPlanRevision
  return (
    <section aria-labelledby={`${id}-heading`} className="mt-10 mb-8 min-w-0 space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <Typography variant="title" id={`${id}-heading`}>
          {t(attemptObservations ? 'inspection.observationsTitle' : 'observation.title')}
        </Typography>
        {headingAction}
      </div>
      <Typography variant="body" className="text-content-secondary">
        {t(attemptObservations ? 'inspection.observationsHelp' : 'observation.description')}
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
          {previous && !attemptObservations && (
            <Typography variant="body" className="text-content-secondary">
              {t('observation.previous')}
            </Typography>
          )}
          {!attemptObservations && (
            <Typography variant="body">
              {t(savedPlan ? 'observation.savedPlan' : 'observation.renderedPlan')}
            </Typography>
          )}
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
            plan={attemptObservations ? undefined : project.editing?.plan}
            onAddCut={
              !attemptObservations &&
              !project.finalized &&
              !['queued', 'running'].includes(project.latestJob?.status ?? '')
                ? onAddCut
                : undefined
            }
            draftPlan={draftPlan}
            editing={attemptObservations ? undefined : editing}
            refreshPlayback={
              resolvePlayback ? () => resolvePlayback(active.source.fingerprint, true) : undefined
            }
            localURL={
              localSources.find((local) => local.fingerprint === active.source.fingerprint)?.url
            }
          />
        </>
      )}
    </section>
  )
}
