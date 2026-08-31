import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { PublishingAgent } from '@/entities/publishing-agent'
import type { PublishJob } from '@/entities/publish-job'
import {
  appFailureFromConnect,
  type AppFailure,
  PublishStage,
  PublishStatus,
  PublishVisibility,
} from '@/shared/api'
import { PUBLISH_AGENT_STALE_MS } from '@/shared/config'
import { AppFailureMessage, Button, Dialog, FieldLabel, Notice, Select } from '@/shared/ui'
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
  const { t } = useTranslation('publishing')
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
  const [prepareFailure, setPrepareFailure] = useState<AppFailure>()
  const { start, cancel, startFailure, cancelFailure } = usePublishPost(ownerId, postSlug)
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
            {t('form.unavailableAgent')}
          </Notice>
        )}
        <Button variant="danger" onClick={() => cancel.mutate(job.id)} pending={cancel.isPending}>
          {t('form.cancel')}
        </Button>
        {cancelFailure && (
          <Notice tone="danger" role="alert" className="mt-4">
            <AppFailureMessage failure={cancelFailure} />
          </Notice>
        )}
      </div>
    )
  }
  return (
    <div className="mt-5">
      {!finalized && !retry && <Notice tone="warning">{t('form.finalizeFirst')}</Notice>}
      {offline && !running && (
        <Notice tone="info" className="mt-3">
          {t('form.offline')}
        </Notice>
      )}
      {!running &&
        job?.status !== PublishStatus.PUBLISHED &&
        job?.status !== PublishStatus.OUTCOME_UNKNOWN && (
          <div className="mt-5 grid gap-4 sm:grid-cols-2">
            <div className="sm:col-span-2">
              <FieldLabel htmlFor="publish-agent">{t('form.agent')}</FieldLabel>
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
              <FieldLabel htmlFor="publish-category">{t('form.category')}</FieldLabel>
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
              <FieldLabel htmlFor="publish-visibility">{t('form.visibility')}</FieldLabel>
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
                <option value={PublishVisibility.PUBLIC}>{t('visibility.public')}</option>
                <option value={PublishVisibility.PRIVATE}>{t('visibility.private')}</option>
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
                setPrepareFailure(undefined)
                if (retry) {
                  setConfirming(true)
                  return
                }
                setPreparing(true)
                try {
                  const savedRevision = await beforePublish()
                  if (savedRevision !== finalizedRevision) {
                    setPrepareError(t('form.changedFinalize'))
                    return
                  }
                  setConfirming(true)
                } catch (cause) {
                  setPrepareFailure(appFailureFromConnect(cause))
                } finally {
                  setPreparing(false)
                }
              }}
            >
              {retry ? t('form.retry') : t('form.publish')}
            </Button>
          )}
        {canCancel && (
          <Button variant="danger" onClick={() => cancel.mutate(job.id)} pending={cancel.isPending}>
            {t('form.cancel')}
          </Button>
        )}
      </div>
      <div aria-live="polite" className="mt-4 empty:hidden">
        {prepareError && (
          <Notice tone="warning" role="alert">
            {prepareError}
          </Notice>
        )}
        {prepareFailure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={prepareFailure} />
          </Notice>
        )}
        {startFailure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={startFailure} />
          </Notice>
        )}
        {cancelFailure && (
          <Notice tone="danger" role="alert">
            <AppFailureMessage failure={cancelFailure} />
          </Notice>
        )}
      </div>
      <Dialog
        open={confirming}
        title={t('form.confirmTitle')}
        confirmLabel={t('form.publish')}
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
        {t('form.confirmDescription', {
          account: selected.platformAccountLabel,
          category:
            selected.categories.find((category) => category.id === effectiveCategoryId)?.name ??
            effectiveCategoryId,
        })}
      </Dialog>
    </div>
  )
}
