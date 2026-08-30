import { Link } from '@tanstack/react-router'
import { usePublishJob, PUBLISH_STAGE_LABELS } from '@/entities/publish-job'
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
        발행하기
      </h2>
      <p className="text-content-secondary mt-2 text-sm leading-relaxed">
        연결한 Mac이 네이버 편집기를 열어 글과 JPEG 사진을 입력하고 최종 발행까지 마칩니다.
      </p>
      {(agents.isPending || publication.isPending) && (
        <p className="text-content-tertiary mt-4 text-sm">발행 상태를 불러오는 중…</p>
      )}
      {(agents.isError || publication.isError) && (
        <Notice tone="danger" role="alert" className="mt-4">
          발행 상태를 불러오지 못했어요.
          <Button
            variant="ghost"
            onClick={() => {
              agents.refetch()
              publication.refetch()
            }}
          >
            발행 상태 다시 불러오기
          </Button>
        </Notice>
      )}
      {!agents.isPending &&
        !agents.isError &&
        agents.agents.filter((agent) => !agent.revokedAt && agent.ready).length === 0 && (
          <Notice tone="info" className="mt-4">
            <span>
              발행할 수 있는 Mac 연결이 아직 없어요. 네이버 로그인 정보는 Mac 밖으로 전송되지
              않습니다.
            </span>
            <Link
              to="/publishing-agents"
              className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
            >
              Mac 연결하기
            </Link>
          </Notice>
        )}
      {job?.status === PublishStatus.PUBLISHED && (
        <Notice tone="success" role="status" className="mt-4">
          발행을 마쳤어요.
          <a
            href={job.platformPostUrl}
            target="_blank"
            rel="noreferrer"
            className="text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-2 underline"
          >
            네이버 글 보기
          </a>
        </Notice>
      )}
      {(job?.status === PublishStatus.QUEUED || job?.status === PublishStatus.RUNNING) && (
        <Notice tone="info" role="status" className="mt-4">
          {PUBLISH_STAGE_LABELS[job.stage] ?? '발행 진행 중'}
        </Notice>
      )}
      {job?.status === PublishStatus.FAILED && (
        <Notice tone="danger" role="alert" className="mt-4">
          {job.errorMessage || '최종 발행 전에 안전하게 중단했어요.'}
        </Notice>
      )}
      {job?.status === PublishStatus.NEEDS_ATTENTION && (
        <Notice tone="warning" role="alert" className="mt-4">
          {job.errorMessage || 'Mac의 전용 브라우저에서 네이버 로그인을 확인해 주세요.'} 확인한 뒤
          아래에서 같은 작업을 다시 시도할 수 있어요.
        </Notice>
      )}
      {job?.status === PublishStatus.OUTCOME_UNKNOWN && (
        <Notice tone="warning" role="alert" className="mt-4">
          최종 발행 버튼이 눌렸을 수 있어요. 중복을 막기 위해 자동 재시도하지 않습니다. 네이버
          블로그에서 직접 확인해 주세요.
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
