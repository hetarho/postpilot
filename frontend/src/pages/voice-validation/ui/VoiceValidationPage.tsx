import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { createClient } from '@connectrpc/connect'
import { useMutation, useTransport } from '@connectrpc/connect-query'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useJob } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import { voiceValidationQueryKey, voiceValidationState } from '@/entities/voice'
import { appFailureFromConnect, appFailureFromProto, VoiceValidationService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { formatNumber, formatPercent } from '@/shared/lib'
import { AppFailureMessage, Badge, Button, Notice } from '@/shared/ui'

export function VoiceValidationPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { voiceId, id } = useParams({ from: '/authenticated/voices/$voiceId/validations/$id' })
  const { user } = useSession()
  const transport = useTransport()
  const key = voiceValidationQueryKey(transport, user?.id ?? '', voiceId, id)
  const query = useQuery({
    queryKey: key,
    queryFn: () =>
      createClient(VoiceValidationService, transport).getVoiceProfileValidation({
        validationId: id,
      }),
    refetchInterval: (state) =>
      ['queued', 'running'].includes(state.state.data?.validation?.status ?? '')
        ? POLL_INTERVAL_MS
        : false,
  })
  const jobState = useJob(query.data?.validation?.jobId ?? '', [key])
  const retry = useMutation(VoiceValidationService.method.retryVoiceProfileValidation)
  const retryFailure = retry.error ? appFailureFromConnect(retry.error) : undefined
  if (query.isPending) return <Placeholder>{t('validation.loading', { ns: 'voices' })}</Placeholder>
  const validation = query.data?.validation
  if (!validation || query.isError)
    return (
      <Placeholder>
        {t('validation.loadFailed', { ns: 'voices' })}{' '}
        <Button variant="ghost" onClick={() => void query.refetch()}>
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Placeholder>
    )
  if (validation.voiceId !== voiceId) {
    return (
      <Placeholder>
        <p role="alert">{t('validation.wrongVoice', { ns: 'voices' })}</p>
      </Placeholder>
    )
  }
  const canRetry =
    validation.status === 'partial' ||
    validation.status === 'failed' ||
    jobState.job?.status === 'failed'
  const status = voiceValidationState(validation.status)
  const retryValidation = async () => {
    try {
      await retry.mutateAsync({ validationId: id })
      await query.refetch()
    } catch {
      // The structured mutation failure is rendered beside the retry action.
    }
  }
  return (
    <main className="mx-auto w-full max-w-4xl px-4 py-6 sm:px-6">
      <Link
        to="/voices/$voiceId/validations"
        params={{ voiceId }}
        className="text-link-fg inline-flex min-h-11 items-center text-sm"
      >
        {t('validation.back', { ns: 'voices' })}
      </Link>
      <div className="mt-4 flex flex-wrap items-baseline justify-between gap-2">
        <h1 className="text-2xl font-semibold">
          {t('validation.title', { ns: 'voices', version: validation.profileVersion.toString() })}
        </h1>
        <Badge tone={status === 'done' ? 'success' : status === 'failed' ? 'danger' : 'neutral'}>
          {t(`validation.status.${status}`, { ns: 'voices' })}
        </Badge>
      </div>
      {canRetry && (
        <Button
          variant="secondary"
          className="mt-4"
          pending={retry.isPending}
          onClick={() => void retryValidation()}
        >
          {t('validation.retry', { ns: 'voices' })}
        </Button>
      )}
      {retryFailure && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={retryFailure} />
        </Notice>
      )}
      {validation.judgeEnabled && validation.totalCount > 0 && (
        <Notice tone="info" className="mt-4">
          {t('validation.score', {
            ns: 'voices',
            rate: formatPercent(validation.yCount / validation.totalCount),
            yes: formatNumber(validation.yCount),
            total: formatNumber(validation.totalCount),
          })}
        </Notice>
      )}
      <div className="mt-6 space-y-8">
        {validation.items.map((item, index) => (
          <article key={item.id}>
            <h2 className="font-semibold">
              {t('validation.item', { ns: 'voices', index: index + 1 })}
            </h2>
            {(item.failure || item.status === 'failed') && (
              <Notice tone="danger" className="mt-2">
                <AppFailureMessage failure={appFailureFromProto(item.failure)} />
              </Notice>
            )}
            <div className="mt-3 grid gap-4 md:grid-cols-2">
              <section className="bg-surface-raised rounded-lg p-4">
                <h3 className="text-sm font-medium">
                  {t('validation.original', { ns: 'voices' })}
                </h3>
                <p className="mt-2 text-sm leading-relaxed whitespace-pre-wrap">{item.original}</p>
              </section>
              <section className="bg-surface-raised rounded-lg p-4">
                <h3 className="text-sm font-medium">{t('validation.summary', { ns: 'voices' })}</h3>
                <p className="text-content-secondary mt-2 text-sm">
                  {item.neutralSummary || t('validation.generating', { ns: 'voices' })}
                </p>
                <h3 className="mt-4 text-sm font-medium">
                  {t('validation.rewritten', { ns: 'voices' })}
                </h3>
                <p className="mt-2 text-sm leading-relaxed whitespace-pre-wrap">
                  {item.regenerated || t('validation.generating', { ns: 'voices' })}
                </p>
                {item.scores.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {item.scores.map((score) => (
                      <Badge key={score.dimension} tone={score.matched ? 'success' : 'warning'}>
                        {score.dimension} {score.matched ? 'Y' : 'N'}
                      </Badge>
                    ))}
                  </div>
                )}
              </section>
            </div>
          </article>
        ))}
      </div>
    </main>
  )
}
function Placeholder({ children }: { children: ReactNode }) {
  return (
    <main className="text-content-tertiary mx-auto w-full max-w-2xl px-4 py-10 text-sm sm:px-6">
      {children}
    </main>
  )
}
