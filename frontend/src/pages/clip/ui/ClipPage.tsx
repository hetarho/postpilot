import { useEffect, useState } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipProject, requiredClipSources, type ClipProject } from '@/entities/clip-project'
import { useClipCorrection, ClipCorrectionWorkspace } from '@/features/correct-clip'
import { useSession } from '@/entities/session'
import { isTerminal, progressLabel, progressRatio } from '@/entities/generation-job'
import { ClipProjectForm } from '@/features/edit-clip-project'
import {
  ClipApprovalAction,
  ClipCreditSettlement,
  ClipGenerationFailure,
  ClipResult,
  useGenerateClip,
} from '@/features/generate-clip'
import { StageModelSelect } from '@/features/select-model'
import { ClipSourcePicker, useClipSourceUpload } from '@/features/upload-clip-sources'
import { appFailureFromConnect } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  ProgressBar,
  Typography,
  buttonStyles,
  pageStyles,
} from '@/shared/ui'

function ExistingClip({ ownerId, project }: { ownerId: string; project: ClipProject }) {
  const { t } = useTranslation('clips')
  const [editing, setEditing] = useState(false)
  const [setupSaved, setSetupSaved] = useState(false)
  const [uploadAllowed, setUploadAllowed] = useState(false)
  const correction = useClipCorrection(ownerId, project)
  const required =
    editing && project.editing
      ? requiredClipSources(correction.draft, project.editing.sources)
      : undefined
  const upload = useClipSourceUpload(project.id, required)
  const generation = useGenerateClip(ownerId, project, upload.attempt?.jobId)
  const ownership = {
    begin: upload.beginAttempt,
    owned: upload.markOwned,
    rejected: upload.rejectAttempt,
  }
  const uploading = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  const job = generation.job
  const running = job && !isTerminal(job)
  useEffect(() => {
    if (job && (job.status === 'done' || job.status === 'failed'))
      upload.finishAttempt(job.id, job.status)
  }, [job, upload])
  const ratio = running ? progressRatio(job) : undefined
  const label = running ? progressLabel(job) : t('source.phase.uploading')
  const pending = generation.busy || uploading
  const status = running
    ? label
    : job?.status === 'failed'
      ? t('generation.failedAt', { stage: progressLabel(job) })
      : editing && correction.dirty
        ? t('correction.dirty')
        : upload.phase !== 'idle'
          ? t(`source.phase.${upload.phase}`)
          : project.editPlanRevision > project.renderedPlanRevision
            ? t('correction.needsRender')
            : setupSaved
              ? t('project.saved')
              : project.result
                ? t(editing ? 'correction.matched' : 'generation.finished')
                : t('source.phase.idle')
  return (
    <>
      {(running || uploading) && (
        <div className="sm:top-header sticky top-0 z-10 -mx-4 sm:-mx-6 lg:-mx-8">
          <ProgressBar
            label={label}
            done={
              running
                ? ratio?.done
                : upload.phase === 'uploading'
                  ? upload.entries.reduce((sum, e) => sum + e.metadata.bytes * e.percent, 0)
                  : undefined
            }
            total={
              running
                ? ratio?.total
                : upload.entries.reduce((sum, e) => sum + e.metadata.bytes * 100, 0)
            }
          />
        </div>
      )}
      <div role="status" aria-live="polite" className="mt-4">
        <Typography variant="meta">{status}</Typography>
      </div>
      {generation.pollFailed && (
        <div role="alert" className="mt-4">
          <Typography variant="body">{t('generation.pollingFailed')}</Typography>
          <Button variant="ghost" onClick={generation.checkAgain}>
            {t('project.retry')}
          </Button>
        </div>
      )}
      {generation.uncertain && (
        <div role="status" className="mt-4 space-y-2">
          <Typography variant="body">{t('credits.uncertain')}</Typography>
          <Button variant="ghost" onClick={generation.checkAgain}>
            {t('credits.checkAttempt')}
          </Button>
        </div>
      )}
      {project.result && (
        <ClipResult key={project.result.createdAt} ownerId={ownerId} project={project} />
      )}
      <ClipCreditSettlement job={job} accounting={generation.accounting} />
      {editing && project.editing ? (
        <ClipCorrectionWorkspace
          correction={correction}
          state={project.editing}
          disabled={pending}
          renderReady={!!upload.readyBatch && correction.revision === project.editPlanRevision}
          renderPending={generation.starting}
          renderFailure={generation.failure}
          onRender={() => {
            if (!correction.dirty && !correction.pending)
              void generation.render(upload.readyBatch, correction.revision, ownership)
          }}
          onExit={() => {
            correction.reset()
            setEditing(false)
          }}
          localSources={upload.entries.map((e) => ({
            fingerprint: e.metadata.fingerprint,
            url: e.previewURL,
          }))}
          sourcePicker={
            <>
              <section aria-labelledby="clip-required-sources" className="mt-10 space-y-3">
                <Typography variant="title" id="clip-required-sources">
                  {t('correction.requiredSources')}
                </Typography>
                <ul className="space-y-2">
                  {required?.map((s) => (
                    <li key={s.fingerprint}>
                      <Typography variant="body" className="break-words">
                        {s.filename}
                      </Typography>
                    </li>
                  ))}
                </ul>
              </section>
              <ClipSourcePicker
                upload={upload}
                correction
                disabled={
                  correction.dirty ||
                  correction.pending ||
                  generation.busy ||
                  !correction.validation?.valid
                }
                processing={generation.busy}
              />
            </>
          }
        />
      ) : (
        <ClipProjectForm
          ownerId={ownerId}
          stored={project}
          showStatus={false}
          onSaveStateChange={setSetupSaved}
          disabled={pending}
          onUploadAllowed={setUploadAllowed}
          refusal={<ClipGenerationFailure failure={generation.failure} />}
          actions={(ready, dirty) => (
            <>
              {project.editing && (
                <Button
                  variant="secondary"
                  disabled={pending || dirty}
                  onClick={() => setEditing(true)}
                >
                  {t('correction.enter')}
                </Button>
              )}
              <ClipApprovalAction
                ownerId={ownerId}
                project={project}
                batch={upload.readyBatch}
                observe={generation.observeRef}
                write={generation.writeRef}
                ready={ready && generation.modelsReady && !generation.busy}
                pending={generation.starting}
                onApprove={(quote) =>
                  void generation.start(upload.readyBatch, ready, quote, ownership)
                }
              />
            </>
          )}
        >
          {(project.result || job?.status === 'failed') && (
            <Typography variant="body" className="text-content-secondary mt-6">
              {t('generation.reselection')}
            </Typography>
          )}
          <ClipSourcePicker
            upload={upload}
            disabled={!uploadAllowed || generation.busy}
            processing={generation.busy}
          />
          <section aria-labelledby="clip-models-heading" className="mt-10 mb-8 space-y-4">
            <Typography id="clip-models-heading" variant="title">
              {t('generation.models')}
            </Typography>
            <StageModelSelect stage="observe" disabled={generation.busy} requireInlineStaticVideo />
            <StageModelSelect stage="write" disabled={generation.busy} />
            {!generation.modelsReady && (
              <Typography variant="body" className="text-content-secondary">
                {t('generation.videoRequired')}
              </Typography>
            )}
            <Typography variant="body" className="text-content-secondary">
              {t('generation.creditPolicy')}
            </Typography>
          </section>
        </ClipProjectForm>
      )}
    </>
  )
}
export function ClipPage() {
  const { t } = useTranslation('clips')
  const { user } = useSession()
  const { clipId } = useParams({ strict: false })
  const query = useClipProject(user?.id ?? '', clipId)
  return (
    <main className={pageStyles({ width: 'prose', className: 'flex flex-1 flex-col pb-8' })}>
      <Link to="/clips" className={buttonStyles({ variant: 'ghost', className: 'self-start' })}>
        {t('project.back')}
      </Link>
      <Typography variant="display" className="mt-6">
        {t(clipId ? 'project.edit' : 'project.new')}
      </Typography>
      {!clipId ? (
        <ClipProjectForm ownerId={user?.id ?? ''} />
      ) : query.data ? (
        <ExistingClip key={`${user?.id}-${clipId}`} ownerId={user?.id ?? ''} project={query.data} />
      ) : query.isPending ? (
        <Typography variant="body" role="status" className="mt-6">
          {t('project.loading')}
        </Typography>
      ) : (
        <div role="alert" className="mt-6">
          <AppFailureMessage failure={appFailureFromConnect(query.error)} />
          <Button variant="ghost" onClick={() => void query.refetch()}>
            {t('project.retry')}
          </Button>
        </div>
      )}
    </main>
  )
}
