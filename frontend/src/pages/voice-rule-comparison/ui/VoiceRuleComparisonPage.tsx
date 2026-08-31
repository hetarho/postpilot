import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { createClient } from '@connectrpc/connect'
import { useTransport } from '@connectrpc/connect-query'
import { useMutation } from '@connectrpc/connect-query'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { useJob } from '@/entities/generation-job'
import { useSession } from '@/entities/session'
import {
  voiceComparisonQueryKey,
  voiceProfileQueryKey,
  voiceVersionsQueryKey,
} from '@/entities/voice'
import { appFailureFromConnect, appFailureFromProto, VoiceValidationService } from '@/shared/api'
import { POLL_INTERVAL_MS } from '@/shared/config'
import { ActionBar, AppFailureMessage, Button, Notice, SegmentedControl } from '@/shared/ui'
import { TextCandidateComparison } from '@/widgets/candidate-comparison'

export function VoiceRuleComparisonPage() {
  const { t } = useTranslation(['voices', 'common'])
  const { voiceId, id } = useParams({ from: '/authenticated/voices/$voiceId/rules/$id/compare' })
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const transport = useTransport()
  const queryClient = useQueryClient()
  const key = voiceComparisonQueryKey(transport, ownerId, voiceId, id)
  const query = useQuery({
    queryKey: key,
    queryFn: () =>
      createClient(VoiceValidationService, transport).getVoiceRuleComparison({ comparisonId: id }),
    refetchInterval: (state) =>
      ['queued', 'running'].includes(state.state.data?.comparison?.status ?? '')
        ? POLL_INTERVAL_MS
        : false,
  })
  const jobState = useJob(query.data?.comparison?.jobId ?? '', [key])
  const decide = useMutation(VoiceValidationService.method.decideVoiceRuleComparison)
  const retry = useMutation(VoiceValidationService.method.retryVoiceRuleComparison)
  const decideFailure = decide.error ? appFailureFromConnect(decide.error) : undefined
  const retryFailure = retry.error ? appFailureFromConnect(retry.error) : undefined
  const [active, setActive] = useState('')
  if (query.isPending) return <Placeholder>{t('comparison.loading', { ns: 'voices' })}</Placeholder>
  const comparison = query.data?.comparison
  if (!comparison || query.isError) {
    return (
      <Placeholder>
        {t('comparison.loadFailed', { ns: 'voices' })}{' '}
        <Button variant="ghost" onClick={() => void query.refetch()}>
          {t('action.retry', { ns: 'common' })}
        </Button>
      </Placeholder>
    )
  }
  // The RPC derives ownership from the comparison id, while the route also names a voice.
  // Refuse a crafted same-account mismatch here so a voice-A decision cannot be presented
  // as (or invalidate caches for) voice B.
  if (comparison.voiceId !== voiceId) {
    return (
      <Placeholder>
        <p role="alert">{t('comparison.wrongVoice', { ns: 'voices' })}</p>
      </Placeholder>
    )
  }
  const candidates = comparison.candidates.map((candidate, index) => ({
    id: candidate.id,
    label: candidate.side || (index === 0 ? 'A' : 'B'),
    text: candidate.output,
    status: candidate.status,
    failure:
      candidate.failure || candidate.status === 'failed'
        ? appFailureFromProto(candidate.failure)
        : undefined,
  }))
  const activeId = candidates.some((candidate) => candidate.id === active)
    ? active
    : (candidates[0]?.id ?? '')
  const ready =
    comparison.status === 'review' &&
    candidates.every((candidate) => candidate.status === 'succeeded')
  const choose = async (candidateId: string) => {
    const side = comparison.candidates.find((candidate) => candidate.id === candidateId)?.side
    if (!side) return
    try {
      await decide.mutateAsync({ comparisonId: id, chosenSide: side })
    } catch {
      // The structured mutation failure is rendered in the action bar.
      return
    }
    // The decision publishes to the comparison's own voice, so only that voice's profile and
    // version list are stale.
    await queryClient.invalidateQueries({ queryKey: key })
    await queryClient.invalidateQueries({
      queryKey: voiceProfileQueryKey(transport, ownerId, voiceId),
    })
    await queryClient.invalidateQueries({
      queryKey: voiceVersionsQueryKey(transport, ownerId, voiceId),
    })
  }
  const canRetry =
    comparison.status === 'partial' ||
    comparison.status === 'failed' ||
    jobState.job?.status === 'failed'
  const retryComparison = async () => {
    try {
      await retry.mutateAsync({ comparisonId: id })
      await query.refetch()
    } catch {
      // The structured mutation failure is rendered beside the retry action.
    }
  }
  return (
    <main className="mx-auto w-full max-w-6xl px-4 py-6 sm:px-6">
      <Link
        to="/voices/$voiceId/rules"
        params={{ voiceId }}
        className="text-link-fg inline-flex min-h-11 items-center text-sm"
      >
        {t('comparison.back', { ns: 'voices' })}
      </Link>
      <h1 className="mt-4 text-2xl font-semibold">{t('comparison.title', { ns: 'voices' })}</h1>
      <p className="text-content-secondary mt-2 text-sm">
        {t('comparison.description', { ns: 'voices' })}
      </p>
      {canRetry && (
        <Button
          variant="secondary"
          className="mt-4"
          pending={retry.isPending}
          onClick={() => void retryComparison()}
        >
          {t('comparison.retry', { ns: 'voices' })}
        </Button>
      )}
      {retryFailure && (
        <Notice tone="danger" role="alert" className="mt-2">
          <AppFailureMessage failure={retryFailure} />
        </Notice>
      )}
      <div className="mt-6">
        <TextCandidateComparison candidates={candidates} activeCandidateId={activeId} />
      </div>
      {activeId && (
        <ActionBar ariaLabel={t('comparison.actionAria', { ns: 'voices' })}>
          <div className="grid gap-3">
            <SegmentedControl
              value={activeId}
              options={candidates.map((candidate) => ({
                value: candidate.id,
                label: candidate.label,
              }))}
              onChange={setActive}
              ariaLabel={t('comparison.selectAria', { ns: 'voices' })}
            />
            {comparison.chosenSide ? (
              <p className="text-sm">{t('comparison.applied', { ns: 'voices' })}</p>
            ) : (
              <Button
                variant="cta"
                disabled={!ready}
                pending={decide.isPending}
                onClick={() => void choose(activeId)}
              >
                {t('comparison.prefer', { ns: 'voices' })}
              </Button>
            )}
            {decideFailure && (
              <Notice tone="danger" role="alert">
                <AppFailureMessage failure={decideFailure} />
              </Notice>
            )}
          </div>
        </ActionBar>
      )}
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
