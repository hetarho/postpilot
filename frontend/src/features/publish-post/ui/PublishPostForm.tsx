import { useState } from 'react'
import type { PublishingAgent } from '@/entities/publishing-agent'
import type { PublishJob } from '@/entities/publish-job'
import { PublishStage, PublishStatus, PublishVisibility } from '@/shared/api'
import { PUBLISH_AGENT_STALE_MS } from '@/shared/config'
import { Button, Dialog, FieldLabel, Notice, Select } from '@/shared/ui'
import { usePublishPost } from '../model/usePublishPost'

export function PublishPostForm({
  ownerId,
  postSlug,
  contentRevision,
  finalizedRevision,
  finalized,
  beforePublish,
  agents,
  observedAt,
  job,
}: {
  ownerId: string
  postSlug: string
  contentRevision: bigint
  finalizedRevision: bigint
  finalized: boolean
  beforePublish: () => Promise<bigint>
  agents: PublishingAgent[]
  observedAt: number
  job: PublishJob | undefined
}) {
  const available = agents.filter((agent) => !agent.revokedAt && agent.ready)
  const retryJob = job?.status === PublishStatus.NEEDS_ATTENTION ? job : undefined
  const retryAgent = retryJob ? available.find((agent) => agent.id === retryJob.agentId) : undefined
  const initial = retryJob ? retryAgent : available[0]
  const [agentId, setAgentId] = useState(initial?.id ?? '')
  const selected = retryJob
    ? retryAgent
    : (available.find((agent) => agent.id === agentId) ?? initial)
  const [categoryId, setCategoryId] = useState(
    retryJob?.categoryId ?? selected?.defaultCategoryId ?? '',
  )
  const [visibility, setVisibility] = useState(
    retryJob?.visibility ?? selected?.defaultVisibility ?? PublishVisibility.PUBLIC,
  )
  const fellBackToAnotherAgent = !retryJob && selected !== undefined && selected.id !== agentId
  const categoryStillAvailable =
    selected?.categories.some((category) => category.id === categoryId) ?? false
  const effectiveCategoryId =
    retryJob?.categoryId ??
    (fellBackToAnotherAgent || !categoryStillAvailable
      ? (selected?.defaultCategoryId ?? '')
      : categoryId)
  const effectiveVisibility =
    retryJob?.visibility ??
    (fellBackToAnotherAgent
      ? (selected?.defaultVisibility ?? PublishVisibility.PUBLIC)
      : visibility)
  const [confirming, setConfirming] = useState(false)
  const [preparing, setPreparing] = useState(false)
  const [prepareError, setPrepareError] = useState('')
  const { start, cancel } = usePublishPost(ownerId, postSlug)
  const offline =
    !selected?.lastSeenAt ||
    observedAt - new Date(selected.lastSeenAt).getTime() > PUBLISH_AGENT_STALE_MS
  const running = job?.status === PublishStatus.QUEUED || job?.status === PublishStatus.RUNNING
  const retry = Boolean(retryJob)
  const canCancel =
    job !== undefined &&
    (running || job.status === PublishStatus.NEEDS_ATTENTION) &&
    job.stage < PublishStage.COMMITTING

  if (!selected) {
    if (!canCancel || !job) return null
    return (
      <div className="mt-5">
        {retryJob && (
          <Notice tone="warning" role="alert" className="mb-4">
            이 작업에 연결된 Mac을 현재 사용할 수 없어 다른 Mac으로 바꾸어 재시도하지 않았어요. Mac
            연결을 다시 활성화하거나 이 작업을 취소해 주세요.
          </Notice>
        )}
        <Button variant="danger" onClick={() => cancel.mutate(job.id)} pending={cancel.isPending}>
          발행 취소
        </Button>
        {cancel.isError && (
          <Notice tone="danger" role="alert" className="mt-4">
            발행을 취소하지 못했어요.
          </Notice>
        )}
      </div>
    )
  }
  return (
    <div className="mt-5">
      {!finalized && !retry && (
        <Notice tone="warning">현재 내용을 먼저 확정해야 정확히 이 버전을 발행할 수 있어요.</Notice>
      )}
      {offline && !running && (
        <Notice tone="info" className="mt-3">
          Mac이 지금 응답하지 않아도 요청은 서버에 보관되고, 에이전트가 켜지면 자동으로 시작됩니다.
        </Notice>
      )}
      {!running &&
        job?.status !== PublishStatus.PUBLISHED &&
        job?.status !== PublishStatus.OUTCOME_UNKNOWN && (
          <div className="mt-5 grid gap-4 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <FieldLabel htmlFor="publish-agent">Mac 연결</FieldLabel>
              <Select
                id="publish-agent"
                value={selected.id}
                disabled={retry}
                onChange={(event) => {
                  const next = available.find((agent) => agent.id === event.target.value)
                  setAgentId(event.target.value)
                  if (next) {
                    setCategoryId(next.defaultCategoryId)
                    setVisibility(next.defaultVisibility)
                  }
                }}
                className="mt-2"
              >
                {available.map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.label} · {agent.platformAccountLabel}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel htmlFor="publish-category">카테고리</FieldLabel>
              <Select
                id="publish-category"
                value={effectiveCategoryId}
                disabled={retry}
                onChange={(event) => {
                  setAgentId(selected.id)
                  setCategoryId(event.target.value)
                  setVisibility(effectiveVisibility)
                }}
                className="mt-2"
              >
                {selected.categories.map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <FieldLabel htmlFor="publish-visibility">공개 설정</FieldLabel>
              <Select
                id="publish-visibility"
                value={effectiveVisibility}
                disabled={retry}
                onChange={(event) => {
                  setAgentId(selected.id)
                  setCategoryId(effectiveCategoryId)
                  setVisibility(Number(event.target.value) as PublishVisibility)
                }}
                className="mt-2"
              >
                <option value={PublishVisibility.PUBLIC}>전체 공개</option>
                <option value={PublishVisibility.PRIVATE}>비공개</option>
              </Select>
            </div>
          </div>
        )}
      <div className="mt-5 flex flex-wrap gap-2">
        {!running &&
          job?.status !== PublishStatus.PUBLISHED &&
          job?.status !== PublishStatus.OUTCOME_UNKNOWN && (
            <Button
              variant="cta"
              className="w-full sm:w-auto"
              disabled={(!retry && !finalized) || !effectiveCategoryId || preparing}
              pending={preparing}
              onClick={async () => {
                setPrepareError('')
                if (retry) {
                  setConfirming(true)
                  return
                }
                setPreparing(true)
                try {
                  const savedRevision = await beforePublish()
                  if (savedRevision !== finalizedRevision) {
                    setPrepareError('방금 수정한 내용을 다시 확정한 뒤 발행해 주세요.')
                    return
                  }
                  setConfirming(true)
                } catch {
                  setPrepareError('수정 내용을 저장하지 못해 발행을 시작하지 않았어요.')
                } finally {
                  setPreparing(false)
                }
              }}
            >
              {retry ? '안전하게 다시 시도' : '네이버에 발행'}
            </Button>
          )}
        {canCancel && (
          <Button variant="danger" onClick={() => cancel.mutate(job.id)} pending={cancel.isPending}>
            발행 취소
          </Button>
        )}
      </div>
      <div aria-live="polite" className="mt-4 empty:hidden">
        {prepareError && (
          <Notice tone="warning" role="alert">
            {prepareError}
          </Notice>
        )}
        {start.error && (
          <Notice tone="danger" role="alert">
            {start.error.message}
          </Notice>
        )}
        {cancel.isError && (
          <Notice tone="danger" role="alert">
            발행을 취소하지 못했어요.
          </Notice>
        )}
      </div>
      <Dialog
        open={confirming}
        title="네이버에 최종 발행할까요?"
        confirmLabel="네이버에 발행"
        onClose={() => setConfirming(false)}
        pending={start.isPending}
        onConfirm={() =>
          start.mutate(
            {
              expectedContentRevision: retryJob?.contentRevision ?? contentRevision,
              agentId: retryJob?.agentId ?? selected.id,
              categoryId: effectiveCategoryId,
              visibility: effectiveVisibility,
            },
            { onSuccess: () => setConfirming(false) },
          )
        }
      >
        <strong>{selected.platformAccountLabel}</strong> 블로그의{' '}
        <strong>
          {selected.categories.find((category) => category.id === effectiveCategoryId)?.name ??
            effectiveCategoryId}
        </strong>{' '}
        카테고리에 올립니다. Mac 에이전트가 사진과 글을 입력한 뒤 네이버의 최종 발행 버튼까지
        누르며, 그 순간에는 추가 확인을 요청하지 않습니다.
      </Dialog>
    </div>
  )
}
