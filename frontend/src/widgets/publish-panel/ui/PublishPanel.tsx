import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { publishStageLabel, usePublishJob } from '@/entities/publish-job'
import { usePublishingAgents } from '@/entities/publishing-agent'
import { PublishPostForm } from '@/features/publish-post'
import { PublishStage, PublishStatus } from '@/shared/api'
import { Button, Notice } from '@/shared/ui'

export function PublishPanel({
  ownerId,
  postSlug,
  contentRevision,
  finalizedRevision,
  status,
  beforePublish,
}: {
  ownerId: string
  postSlug: string
  contentRevision: bigint
  finalizedRevision: bigint
  status: string
  beforePublish: () => Promise<bigint>
}) {
  const { t } = useTranslation('publishing')
  const agents = usePublishingAgents(ownerId)
  const publication = usePublishJob(ownerId, postSlug)
  const job = publication.job
  const finalized = status === 'finalized' && finalizedRevision === contentRevision
  const cancelable =
    job !== undefined &&
    (job.status === PublishStatus.QUEUED ||
      job.status === PublishStatus.RUNNING ||
      job.status === PublishStatus.NEEDS_ATTENTION) &&
    job.stage < PublishStage.COMMITTING

  return (
    <section aria-labelledby="publish-heading" className="mt-12">
      <h2 id="publish-heading" className="text-lg font-semibold tracking-tight">
        {t('title')}
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        {t('panel.description')}
      </p>
      {(agents.isPending || publication.isPending) && (
        <p className="text-content-tertiary mt-4 text-sm">{t('panel.loading')}</p>
      )}
      {(agents.isError || publication.isError) && (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('panel.loadFailed')}
          <Button
            variant="ghost"
            onClick={() => {
              agents.refetch()
              publication.refetch()
            }}
          >
            {t('panel.reload')}
          </Button>
        </Notice>
      )}
      {!agents.isPending &&
        !agents.isError &&
        agents.agents.filter((agent) => !agent.revokedAt && agent.ready).length === 0 && (
          <Notice tone="info" className="mt-4">
            <span>{t('panel.noAgent')}</span>
            <Link
              to="/publishing-agents"
              className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
            >
              {t('panel.connectAgent')}
            </Link>
          </Notice>
        )}
      {job?.status === PublishStatus.PUBLISHED && (
        <Notice tone="success" role="status" className="mt-4">
          {t('panel.published')}
          <a
            href={job.platformPostUrl}
            target="_blank"
            rel="noreferrer"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
          >
            {t('panel.viewPost')}
          </a>
        </Notice>
      )}
      {(job?.status === PublishStatus.QUEUED || job?.status === PublishStatus.RUNNING) && (
        <Notice tone="info" role="status" className="mt-4">
          {publishStageLabel(job.stage)}
        </Notice>
      )}
      {job?.status === PublishStatus.FAILED && (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('panel.failed')}
        </Notice>
      )}
      {job?.status === PublishStatus.NEEDS_ATTENTION && (
        <Notice tone="warning" role="alert" className="mt-4">
          {t('panel.needsAttention')}
        </Notice>
      )}
      {job?.status === PublishStatus.OUTCOME_UNKNOWN && (
        <Notice tone="warning" role="alert" className="mt-4">
          {t('panel.outcomeUnknown')}
        </Notice>
      )}
      {!agents.isPending &&
        !agents.isError &&
        !publication.isPending &&
        !publication.isError &&
        (cancelable || agents.agents.some((agent) => !agent.revokedAt && agent.ready)) && (
          <PublishPostForm
            key={`${job?.id ?? 'new'}:${job?.status ?? 'none'}:${job?.categoryId ?? ''}:${job?.visibility ?? ''}`}
            ownerId={ownerId}
            postSlug={postSlug}
            contentRevision={contentRevision}
            finalizedRevision={finalizedRevision}
            finalized={finalized}
            beforePublish={beforePublish}
            agents={agents.agents}
            observedAt={agents.observedAt}
            job={job}
          />
        )}
    </section>
  )
}
