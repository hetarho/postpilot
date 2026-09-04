import { useTranslation } from 'react-i18next'
import { ConfigurePublishingAgent } from '@/features/configure-publishing-agent'
import { CancelRetainedPublishJobButton } from '@/features/cancel-publish'
import { PairPublishingAgent } from '@/features/pair-publishing-agent'
import { RevokePublishingAgent } from '@/features/revoke-publishing-agent'
import { usePublishingAgents } from '@/entities/publishing-agent'
import { useRetryablePublishJobs } from '@/entities/publish-job'
import { RetryPublishJobButton } from '@/features/retry-publish'
import { useSession } from '@/entities/session'
import { AppFailureMessage, Badge, Button, Notice, Typography, pageStyles } from '@/shared/ui'
import { formatDateTime } from '@/shared/lib'

export function PublishingAgentsPage() {
  const { t } = useTranslation(['publishing', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const agents = usePublishingAgents(ownerId)
  const retryable = useRetryablePublishJobs(ownerId)
  return (
    <main className={pageStyles({ className: 'py-10' })}>
      <Typography variant="display">{t('agents.title', { ns: 'publishing' })}</Typography>
      <Typography variant="body" className="text-content-secondary mt-3">
        {t('agents.description', { ns: 'publishing' })}
      </Typography>
      <div className="mt-10">
        <PairPublishingAgent ownerId={ownerId} />
      </div>
      <section aria-labelledby="agents-heading" className="mt-12">
        <Typography variant="title" id="agents-heading">
          {t('agents.list', { ns: 'publishing' })}
        </Typography>
        {agents.isPending && (
          <Typography variant="body" className="text-content-tertiary mt-4">
            {t('state.loading', { ns: 'common' })}
          </Typography>
        )}
        {agents.isError && (
          <Notice tone="danger" role="alert" className="mt-4">
            {t('agents.loadFailed', { ns: 'publishing' })}
            <Button variant="ghost" onClick={agents.refetch}>
              {t('action.retry', { ns: 'common' })}
            </Button>
          </Notice>
        )}
        {!agents.isPending && !agents.isError && agents.agents.length === 0 && (
          <Typography variant="body" className="text-content-tertiary mt-4">
            {t('agents.empty', { ns: 'publishing' })}
          </Typography>
        )}
        <div className="mt-4 grid gap-4">
          {agents.agents.map((agent) => (
            <article key={agent.id} className="bg-surface-raised rounded-lg p-4">
              <div className="flex min-w-0 items-start justify-between gap-3">
                <div className="min-w-0">
                  <Typography variant="title" as="h3" className="break-words">
                    {agent.label}
                  </Typography>
                  <Typography variant="label" as="p" className="mt-1 break-words">
                    {agent.platformAccountLabel || t('agents.naverPending', { ns: 'publishing' })} ·{' '}
                    {agent.browserLabel}
                  </Typography>
                </div>
                <Badge tone={agent.revokedAt ? 'neutral' : agent.ready ? 'accent' : 'neutral'}>
                  {agent.revokedAt
                    ? t('agents.revoked', { ns: 'publishing' })
                    : agent.ready
                      ? t('agents.ready', { ns: 'publishing' })
                      : t('agents.setup', { ns: 'publishing' })}
                </Badge>
              </div>
              <Typography variant="meta" as="p" className="mt-3">
                {t('lastSeen', { ns: 'publishing' })}{' '}
                {agent.lastSeenAt
                  ? formatDateTime(agent.lastSeenAt)
                  : t('state.none', { ns: 'common' })}
              </Typography>
              <ConfigurePublishingAgent
                key={`${agent.id}:${agent.label}:${agent.defaultCategoryId}:${agent.defaultVisibility}`}
                ownerId={ownerId}
                agent={agent}
              />
              {!agent.revokedAt && (
                <div className="mt-4">
                  <RevokePublishingAgent ownerId={ownerId} agentId={agent.id} label={agent.label} />
                </div>
              )}
            </article>
          ))}
        </div>
      </section>
      <section aria-labelledby="retryable-publishing-heading" className="mt-12">
        <Typography variant="title" id="retryable-publishing-heading">
          {t('agents.retryTitle', { ns: 'publishing' })}
        </Typography>
        <Typography variant="body" className="text-content-secondary mt-2">
          {t('agents.retryDescription', { ns: 'publishing' })}
        </Typography>
        {retryable.isPending && (
          <Typography variant="body" className="text-content-tertiary mt-4">
            {t('agents.retryLoading', { ns: 'publishing' })}
          </Typography>
        )}
        {retryable.isError && (
          <Notice tone="danger" role="alert" className="mt-4">
            {t('agents.retryLoadFailed', { ns: 'publishing' })}
            <Button variant="ghost" onClick={retryable.refetch}>
              {t('action.retry', { ns: 'common' })}
            </Button>
          </Notice>
        )}
        {!retryable.isPending && !retryable.isError && retryable.jobs.length === 0 && (
          <Typography variant="body" className="text-content-tertiary mt-4">
            {t('agents.retryEmpty', { ns: 'publishing' })}
          </Typography>
        )}
        <div className="mt-4 grid gap-4">
          {retryable.jobs.map((job) => (
            <article key={job.id} className="bg-surface-raised rounded-lg p-4">
              <Typography variant="title" as="h3" className="break-words">
                {job.postSlug}
              </Typography>
              <Typography variant="label" as="div" className="mt-1">
                {job.failure ? (
                  <AppFailureMessage failure={job.failure} />
                ) : (
                  t('agents.loginCheck', { ns: 'publishing' })
                )}
              </Typography>
              <div className="mt-4 flex flex-wrap gap-2">
                <RetryPublishJobButton ownerId={ownerId} jobId={job.id} />
                <CancelRetainedPublishJobButton
                  ownerId={ownerId}
                  jobId={job.id}
                  postSlug={job.postSlug}
                />
              </div>
            </article>
          ))}
        </div>
      </section>
    </main>
  )
}
