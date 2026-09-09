import { useEffect, useRef, useState } from 'react'
import { Link, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipProject, type ClipProject } from '@/entities/clip-project'
import { useSession } from '@/entities/session'
import { isTerminal, progressLabel, progressRatio } from '@/entities/generation-job'
import { ClipProjectForm } from '@/features/edit-clip-project'
import { ClipGenerationFailure, ClipResult, useGenerateClip } from '@/features/generate-clip'
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
  const [uploadAllowed, setUploadAllowed] = useState(false)
  const upload = useClipSourceUpload(project.id)
  const generation = useGenerateClip(ownerId, project)
  const handledJob = useRef(project.latestJob?.id)
  const uploading = ['reading', 'uploading', 'cancelling'].includes(upload.phase)
  const job = generation.job
  const running = job && !isTerminal(job)
  useEffect(() => {
    if (!job) return
    const changed = job.id !== handledJob.current
    handledJob.current = job.id
    if (upload.readyBatch && (running || changed)) upload.finishAttempt()
  }, [job, running, upload])
  const ratio = running ? progressRatio(job) : undefined
  const label = running ? progressLabel(job) : t('source.phase.uploading')
  return (
    <ClipProjectForm
      ownerId={ownerId}
      stored={project}
      disabled={generation.busy || uploading}
      onUploadAllowed={setUploadAllowed}
      status={(saved) => (
        <Typography variant="meta">
          {running
            ? label
            : job?.status === 'failed'
              ? t('generation.failedAt', { stage: progressLabel(job) })
              : upload.phase !== 'idle'
                ? t(`source.phase.${upload.phase}`)
                : saved
                  ? t('project.saved')
                  : project.result
                    ? t('generation.finished')
                    : t('source.phase.idle')}
        </Typography>
      )}
      progress={
        (running || uploading) && (
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
        )
      }
      refusal={<ClipGenerationFailure failure={generation.failure} />}
      actions={(ready) => (
        <Button
          variant={ready ? 'cta' : 'secondary'}
          className="w-full sm:w-auto"
          pending={generation.starting}
          disabled={!ready || !generation.modelsReady || !upload.readyBatch || generation.busy}
          onClick={() => void generation.start(upload.readyBatch, ready, upload.finishAttempt)}
        >
          {t(
            project.result || job?.status === 'failed' ? 'generation.retry' : 'generation.generate',
          )}
        </Button>
      )}
    >
      {generation.pollFailed && (
        <div role="alert" className="mt-4">
          <Typography variant="body">{t('generation.pollingFailed')}</Typography>
          <Button variant="ghost" onClick={generation.checkAgain}>
            {t('project.retry')}
          </Button>
        </div>
      )}
      {project.result && (
        <ClipResult key={project.result.createdAt} ownerId={ownerId} project={project} />
      )}
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
        <StageModelSelect stage="observe" disabled={generation.busy} requireVideoInput />
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
