import { useTranslation } from 'react-i18next'
import { ConfigurePublishingAgent } from '@/features/configure-publishing-agent'
import { CancelRetainedPublishJobButton } from '@/features/cancel-publish'
import { PairPublishingAgent } from '@/features/pair-publishing-agent'
import { RevokePublishingAgent } from '@/features/revoke-publishing-agent'
import { usePublishingAgents } from '@/entities/publishing-agent'
import { useRetryablePublishJobs } from '@/entities/publish-job'
import { RetryPublishJobButton } from '@/features/retry-publish'
import { useSession } from '@/entities/session'
import { AppFailureMessage, Badge, Button, Notice } from '@/shared/ui'
import { formatDateTime } from '@/shared/lib'

export function PublishingAgentsPage() {
  const { t } = useTranslation(['publishing', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const agents = usePublishingAgents(ownerId)
  const retryable = useRetryablePublishJobs(ownerId)
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-10 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">
        {t('agents.title', { ns: 'publishing' })}
      </h1>
      <p className="text-content-secondary mt-3 text-sm leading-relaxed">
        {t('agents.description', { ns: 'publishing' })}
      </p>
      <div className="mt-10">
        <PairPublishingAgent ownerId={ownerId} />
      </div>
      <section aria-labelledby="agents-heading" className="mt-12">
        <h2 id="agents-heading" className="text-lg font-semibold tracking-tight">
          {t('agents.list', { ns: 'publishing' })}
        </h2>
        {agents.isPending && (
          <p className="text-content-tertiary mt-4 text-sm">
            {t('state.loading', { ns: 'common' })}
          </p>
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
          <p className="text-content-tertiary mt-4 text-sm">
            {t('agents.empty', { ns: 'publishing' })}
          </p>
        )}
        <div className="mt-4 grid gap-4">
          {agents.agents.map((agent) => (
            <article key={agent.id} className="bg-surface-raised rounded-lg p-4">
              <div className="flex min-w-0 items-start justify-between gap-3">
                <div className="min-w-0">
                  <h3 className="font-semibold break-words">{agent.label}</h3>
                  <p className="text-content-secondary mt-1 text-sm break-words">
                    {agent.platformAccountLabel || t('agents.naverPending', { ns: 'publishing' })} ·{' '}
                    {agent.browserLabel}
                  </p>
                </div>
                <Badge tone={agent.revokedAt ? 'neutral' : agent.ready ? 'accent' : 'neutral'}>
                  {agent.revokedAt
                    ? t('agents.revoked', { ns: 'publishing' })
                    : agent.ready
                      ? t('agents.ready', { ns: 'publishing' })
                      : t('agents.setup', { ns: 'publishing' })}
                </Badge>
              </div>
              <p className="text-content-tertiary mt-3 text-xs">
                {t('lastSeen', { ns: 'publishing' })}{' '}
                {agent.lastSeenAt
                  ? formatDateTime(agent.lastSeenAt)
                  : t('state.none', { ns: 'common' })}
              </p>
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
        <h2 id="retryable-publishing-heading" className="text-lg font-semibold tracking-tight">
          {t('agents.retryTitle', { ns: 'publishing' })}
        </h2>
        <p className="text-content-secondary mt-2 text-sm leading-relaxed">
          {t('agents.retryDescription', { ns: 'publishing' })}
        </p>
        {retryable.isPending && (
          <p className="text-content-tertiary mt-4 text-sm">
            {t('agents.retryLoading', { ns: 'publishing' })}
          </p>
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
          <p className="text-content-tertiary mt-4 text-sm">
            {t('agents.retryEmpty', { ns: 'publishing' })}
          </p>
        )}
        <div className="mt-4 grid gap-4">
          {retryable.jobs.map((job) => (
            <article key={job.id} className="bg-surface-raised rounded-lg p-4">
              <h3 className="font-semibold break-words">{job.postSlug}</h3>
              <div className="text-content-secondary mt-1 text-sm">
                {job.failure ? (
                  <AppFailureMessage failure={job.failure} />
                ) : (
                  t('agents.loginCheck', { ns: 'publishing' })
                )}
              </div>
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
