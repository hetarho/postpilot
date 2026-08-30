import { ConfigurePublishingAgent } from '@/features/configure-publishing-agent'
import { CancelRetainedPublishJobButton } from '@/features/cancel-publish'
import { PairPublishingAgent } from '@/features/pair-publishing-agent'
import { RevokePublishingAgent } from '@/features/revoke-publishing-agent'
import { usePublishingAgents } from '@/entities/publishing-agent'
import { useRetryablePublishJobs } from '@/entities/publish-job'
import { RetryPublishJobButton } from '@/features/retry-publish'
import { useSession } from '@/entities/session'
import { Badge, Button, Notice } from '@/shared/ui'

export function PublishingAgentsPage() {
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const agents = usePublishingAgents(ownerId)
  const retryable = useRetryablePublishJobs(ownerId)
  return (
    <main className="mx-auto w-full max-w-2xl px-4 py-10 sm:px-6">
      <h1 className="text-2xl font-semibold tracking-tight">발행 Mac</h1>
      <p className="text-content-secondary mt-3 text-sm leading-relaxed">
        계정마다 별도의 Mac 토큰, Hermes 프로필과 전용 브라우저 프로필을 사용합니다.
      </p>
      <div className="mt-10">
        <PairPublishingAgent ownerId={ownerId} />
      </div>
      <section aria-labelledby="agents-heading" className="mt-12">
        <h2 id="agents-heading" className="text-lg font-semibold tracking-tight">
          연결 목록
        </h2>
        {agents.isPending && <p className="text-content-tertiary mt-4 text-sm">불러오는 중…</p>}
        {agents.isError && (
          <Notice tone="danger" role="alert" className="mt-4">
            연결 목록을 불러오지 못했어요.
            <Button variant="ghost" onClick={agents.refetch}>
              다시 시도
            </Button>
          </Notice>
        )}
        {!agents.isPending && !agents.isError && agents.agents.length === 0 && (
          <p className="text-content-tertiary mt-4 text-sm">아직 연결한 Mac이 없어요.</p>
        )}
        <div className="mt-4 grid gap-4">
          {agents.agents.map((agent) => (
            <article key={agent.id} className="bg-surface-raised rounded-lg p-4">
              <div className="flex min-w-0 items-start justify-between gap-3">
                <div className="min-w-0">
                  <h3 className="font-semibold break-words">{agent.label}</h3>
                  <p className="text-content-secondary mt-1 text-sm break-words">
                    {agent.platformAccountLabel || '네이버 확인 대기'} · {agent.browserLabel}
                  </p>
                </div>
                <Badge tone={agent.revokedAt ? 'neutral' : agent.ready ? 'accent' : 'neutral'}>
                  {agent.revokedAt ? '해제됨' : agent.ready ? '준비됨' : '설정 필요'}
                </Badge>
              </div>
              <p className="text-content-tertiary mt-3 text-xs">
                마지막 확인{' '}
                {agent.lastSeenAt ? new Date(agent.lastSeenAt).toLocaleString('ko-KR') : '없음'}
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
          로그인 복구 후 다시 시도
        </h2>
        <p className="text-content-secondary mt-2 text-sm leading-relaxed">
          Mac의 같은 전용 브라우저에서 로그인·CAPTCHA·2단계 인증을 해결한 뒤, 고정해 둔 발행 작업을
          그대로 다시 시작합니다. 원본 글을 삭제했어도 이 목록에서 재개할 수 있어요.
        </p>
        {retryable.isPending && (
          <p className="text-content-tertiary mt-4 text-sm">복구 대기 작업을 불러오는 중…</p>
        )}
        {retryable.isError && (
          <Notice tone="danger" role="alert" className="mt-4">
            복구 대기 작업을 불러오지 못했어요.
            <Button variant="ghost" onClick={retryable.refetch}>
              다시 시도
            </Button>
          </Notice>
        )}
        {!retryable.isPending && !retryable.isError && retryable.jobs.length === 0 && (
          <p className="text-content-tertiary mt-4 text-sm">복구 후 다시 시도할 작업이 없어요.</p>
        )}
        <div className="mt-4 grid gap-4">
          {retryable.jobs.map((job) => (
            <article key={job.id} className="bg-surface-raised rounded-lg p-4">
              <h3 className="font-semibold break-words">{job.postSlug}</h3>
              <p className="text-content-secondary mt-1 text-sm">
                {job.errorMessage || 'Mac의 전용 네이버 로그인을 확인해 주세요.'}
              </p>
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
