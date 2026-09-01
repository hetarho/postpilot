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
import { AppFailureMessage, Button, Dialog, FieldLabel, Listbox, Notice } from '@/shared/ui'
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
              <FieldLabel id="publish-agent-label" htmlFor="publish-agent">
                {t('form.agent')}
              </FieldLabel>
              <Listbox
                id="publish-agent"
                aria-labelledby="publish-agent-label"
                value={selected.id}
                options={available.map((agent) => ({
                  value: agent.id,
                  label: `${agent.label} · ${agent.platformAccountLabel}`,
                }))}
                disabled={retry}
                onChange={(id) => {
                  const next = available.find((agent) => agent.id === id)
                  setAgentId(id)
                  if (next) {
                    setCategoryId(next.defaultCategoryId)
                    setVisibility(next.defaultVisibility)
                  }
                }}
                className="mt-2"
              />
            </div>
            <div>
              <FieldLabel id="publish-category-label" htmlFor="publish-category">
                {t('form.category')}
              </FieldLabel>
              <Listbox
                id="publish-category"
                aria-labelledby="publish-category-label"
                value={effectiveCategoryId}
                options={selected.categories.map((category) => ({
                  value: category.id,
                  label: category.name,
                }))}
                disabled={retry}
                onChange={(categoryId) => {
                  setAgentId(selected.id)
                  setCategoryId(categoryId)
                  setVisibility(effectiveVisibility)
                }}
                className="mt-2"
              />
            </div>
            <div>
              <FieldLabel id="publish-visibility-label" htmlFor="publish-visibility">
                {t('form.visibility')}
              </FieldLabel>
              <Listbox<PublishVisibility>
                id="publish-visibility"
                aria-labelledby="publish-visibility-label"
                value={effectiveVisibility}
                options={[
                  { value: PublishVisibility.PUBLIC, label: t('visibility.public') },
                  { value: PublishVisibility.PRIVATE, label: t('visibility.private') },
                ]}
                disabled={retry}
                onChange={(visibility) => {
                  setAgentId(selected.id)
                  setCategoryId(effectiveCategoryId)
                  setVisibility(visibility)
                }}
                className="mt-2"
              />
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
