import { useWritingTestTranslation } from '@/features/writing-test'
import { useMemo, useState } from 'react'
import { useExperiments } from '@/entities/model-experiment'
import { useVoiceChecks, useVoiceChecksQueryKey } from '@/entities/voice'
import { ProgressLine, FailureNotice, isTerminal, useJob } from '@/entities/generation-job'
import { useWritingTests, type TestList, type WritingTest } from '@/entities/writing-test'
import { formatDateTime } from '@/shared/lib'
import {
  Badge,
  Button,
  Notice,
  Typography,
  buttonStyles,
  useNavigationContext,
  type BadgeTone,
  type NavigationLinkProps,
} from '@/shared/ui'

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}
function HistoryLink(props: NavigationLinkProps) {
  const Link = useNavigationContext()?.Link ?? NativeLink
  return <Link {...props} />
}

export interface WritingTestHistoryProps {
  ownerId: string
  /** The route may already display the same history heading and guidance. */
  showHeading?: boolean
  voiceId?: string
  stage?: WritingTest['modelStage'] | 'voice'
  sourcePostSlug?: string
  testHref?: (id: string) => string
}

export function WritingTestHistory(props: WritingTestHistoryProps) {
  return (
    <OwnedHistory
      key={JSON.stringify([
        props.ownerId,
        props.sourcePostSlug ?? '',
        props.voiceId ?? '',
        props.stage ?? '',
      ])}
      {...props}
    />
  )
}

function mergeTests(previous: WritingTest[], incoming: WritingTest[]) {
  const records = new Map(previous.map((test) => [test.id, test]))
  for (const test of incoming) {
    const existing = records.get(test.id)
    if (!existing || existing.revision <= test.revision) records.set(test.id, test)
  }
  return [...records.values()].sort(
    (left, right) =>
      Date.parse(right.createdAt || right.updatedAt) - Date.parse(left.createdAt || left.updatedAt),
  )
}
function OwnedHistory({
  ownerId,
  showHeading = true,
  voiceId,
  stage,
  sourcePostSlug,
  testHref,
}: WritingTestHistoryProps) {
  const { t } = useWritingTestTranslation()
  const [cursor, setCursor] = useState('')
  const query = useWritingTests(ownerId, {
    pageSize: 20,
    pageToken: cursor,
    sourcePostSlug,
    voiceId,
  })
  const [loaded, setLoaded] = useState<{
    lastData?: TestList
    tests: WritingTest[]
    tokens: string[]
  }>({ tests: [], tokens: [] })
  // Derived server pages accumulate without discarding earlier readable records. Owner changes remount.
  if (query.data && query.data !== loaded.lastData)
    setLoaded({
      lastData: query.data,
      tests: mergeTests(loaded.tests, query.data.tests),
      tokens: loaded.tokens.includes(cursor) ? loaded.tokens : [...loaded.tokens, cursor],
    })
  const next = query.data?.nextPageToken
  // The authenticated server selects source/style membership without exposing
  // blind contestant refs. Filtering hidden identities here would drop active work.
  const tests = loaded.tests.filter(
    (test) => !stage || (stage === 'voice' ? test.factor === 'voice' : test.modelStage === stage),
  )
  const historySearch = new URLSearchParams()
  if (stage) historySearch.set('stage', stage)
  if (sourcePostSlug) historySearch.set('source', sourcePostSlug)
  if (voiceId) historySearch.set('voiceId', voiceId)
  const historyEntry = historySearch.size ? `/tests/history?${historySearch}` : '/tests/history'
  const entrySearch = historyEntry ? `?entry=${encodeURIComponent(historyEntry)}` : ''
  return (
    <div className="grid gap-8">
      <section aria-labelledby="writing-test-history-title" className="min-w-0">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className={showHeading ? 'min-w-0' : 'sr-only'}>
            <Typography variant="title" id="writing-test-history-title">
              {t('history')}
            </Typography>
            <Typography variant="body" className="text-content-secondary mt-2">
              {t('historyHelp')}
            </Typography>
          </div>
          <Button
            variant="secondary"
            pending={query.isFetching}
            disabled={!ownerId}
            onClick={() => {
              if (cursor) setCursor('')
              else void query.refetch()
            }}
          >
            {t('refresh')}
          </Button>
        </div>
        {query.isError && (
          <Notice tone="danger" role="alert" className="mt-4">
            {t('failed')}
          </Notice>
        )}
        {query.isPending && ownerId && (
          <Notice tone="info" role="status" className="mt-4">
            {t('loading')}
          </Notice>
        )}
        {query.isSuccess && tests.length === 0 && (
          <Typography variant="body" className="mt-6">
            {t('historyEmpty')}
          </Typography>
        )}
        <ol className="mt-4 grid gap-0">
          {tests.map((test) => (
            <TestRecord
              key={test.id}
              test={test}
              href={testHref?.(test.id) ?? `/tests/${encodeURIComponent(test.id)}${entrySearch}`}
            />
          ))}
        </ol>
        {next && !loaded.tokens.includes(next) && (
          <Button
            variant="secondary"
            className="mt-5"
            pending={query.isFetching}
            onClick={() => setCursor(next)}
          >
            {t('more')}
          </Button>
        )}
      </section>
      {ownerId && (
        <LegacyRecords
          key={`legacy:${ownerId}:${voiceId ?? ''}`}
          ownerId={ownerId}
          voiceId={voiceId}
          stage={stage}
          sourcePostSlug={sourcePostSlug}
          entry={historyEntry}
        />
      )}
      {ownerId && voiceId && !sourcePostSlug && (!stage || stage === 'voice') && (
        <LegacyVoiceChecks
          key={`checks:${ownerId}:${voiceId}`}
          ownerId={ownerId}
          voiceId={voiceId}
        />
      )}
    </div>
  )
}
const tones: Record<WritingTest['status'], BadgeTone> = {
  queued: 'info',
  running: 'info',
  partial: 'warning',
  review: 'accent',
  completed: 'success',
  cancelled: 'neutral',
  failed: 'danger',
}
const labels: Record<WritingTest['status'], string> = {
  queued: 'working',
  running: 'working',
  partial: 'partial',
  review: 'compareTitle',
  completed: 'winnerTitle',
  cancelled: 'cancelled',
  failed: 'failed',
}
function TestRecord({ test, href }: { test: WritingTest; href: string }) {
  const { t } = useWritingTestTranslation()
  const [observedAt] = useState(() => Date.now())
  const kind = t(`factor.${test.factor}`)
  const champion = test.revealed
    ? test.candidates.find((candidate) => candidate.id === test.winnerCandidateId)
    : undefined
  const expired =
    (!!test.contentExpiresAt && Date.parse(test.contentExpiresAt) <= observedAt) ||
    (test.status === 'completed' && test.candidates.every((candidate) => !candidate.output))
  return (
    <li className="border-divider grid min-w-0 gap-4 border-b py-5 sm:grid-cols-[minmax(0,1fr)_auto]">
      <div className="min-w-0">
        <Typography variant="fieldTitle" as="h3" className="break-words">
          {kind} · {t(`formatLabel.${test.count}`)}
        </Typography>
        <div className="mt-2 flex flex-wrap items-center gap-3">
          <Badge tone={tones[test.status]}>
            {t(test.status === 'completed' ? `winnerTitle.${test.factor}` : labels[test.status], {
              kind,
            })}
          </Badge>
          {(test.createdAt || test.updatedAt) && (
            <Typography variant="meta" as="time" dateTime={test.createdAt || test.updatedAt}>
              {formatDateTime(test.createdAt || test.updatedAt)}
            </Typography>
          )}
        </div>
        <Typography variant="body" className="mt-3">
          {t('progress', {
            ready: test.candidates.filter((candidate) => candidate.status === 'succeeded').length,
            total: test.count,
          })}
        </Typography>
        <Typography variant="meta" as="p" className="text-content-secondary mt-1">
          {t('matchProgress', {
            done: test.matches.filter((match) => match.winnerCandidateId).length,
            total: test.count - 1,
          })}
        </Typography>
        {champion && (
          <Typography variant="body" className="mt-2 break-words">
            {champion.identity?.label ||
              champion.displayLabel ||
              t('contender', { number: test.candidates.indexOf(champion) + 1 })}
          </Typography>
        )}
        {expired && (
          <Typography variant="body" className="text-content-secondary mt-2">
            {t('expired')} · {t('expiredHelp')}
          </Typography>
        )}
      </div>
      <div className="self-center">
        <HistoryLink href={href} className={buttonStyles({ variant: 'ghost' })}>
          {t('resume')}
        </HistoryLink>
      </div>
    </li>
  )
}
function LegacyRecords({
  ownerId,
  voiceId,
  stage,
  sourcePostSlug,
  entry,
}: Omit<WritingTestHistoryProps, 'testHref'> & { entry?: string }) {
  const { t } = useWritingTestTranslation()
  const modelStage = stage === 'voice' ? 'write' : stage
  const source =
    stage === 'voice' || (!stage && voiceId && !sourcePostSlug)
      ? 'voice'
      : stage
        ? 'post'
        : undefined
  const query = useExperiments(modelStage, source, ownerId)
  const experiments = query.experiments
    .filter(
      (record) =>
        (!modelStage || record.stage === modelStage) &&
        (!source || record.source === source) &&
        (!voiceId || record.voiceId === voiceId) &&
        (!sourcePostSlug || record.postSlug === sourcePostSlug),
    )
    .toSorted((left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt))
  return (
    <section aria-labelledby="legacy-writing-test-history-title" className="min-w-0">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <Typography variant="title" id="legacy-writing-test-history-title">
            {t('legacy')}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-2">
            {t('legacyReadOnly')}
          </Typography>
        </div>
        <Button variant="ghost" pending={query.isFetching} onClick={() => void query.refetch()}>
          {t('refresh')}
        </Button>
      </div>
      {query.isPending && (
        <Notice tone="info" role="status" className="mt-4">
          {t('loading')}
        </Notice>
      )}
      {query.isError && (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('failed')}
        </Notice>
      )}
      {query.isSuccess && experiments.length === 0 && (
        <Typography variant="body" className="mt-4">
          {t('historyEmpty')}
        </Typography>
      )}
      <ol className="mt-4 grid gap-0">
        {experiments.map((experiment) => (
          <li
            key={experiment.id}
            className="border-divider grid min-w-0 gap-3 border-b py-5 sm:grid-cols-[minmax(0,1fr)_auto]"
          >
            <div className="min-w-0">
              <Typography variant="fieldTitle" as="h3">
                {experiment.source === 'voice' ? t('factor.voice') : t(`stage.${experiment.stage}`)}
              </Typography>
              {experiment.templateName && (
                <Typography variant="body" className="mt-2 break-words">
                  {experiment.templateName}
                </Typography>
              )}
              {experiment.createdAt && (
                <Typography
                  variant="meta"
                  as="time"
                  dateTime={experiment.createdAt}
                  className="mt-2 block"
                >
                  {formatDateTime(experiment.createdAt)}
                </Typography>
              )}
              {experiment.candidates.some(
                (candidate) => candidate.status === 'succeeded' && !candidate.output,
              ) && (
                <Typography variant="body" className="text-content-secondary mt-2">
                  {t('expired')}
                </Typography>
              )}
            </div>
            <div className="self-center">
              <HistoryLink
                href={`/tests/records/${encodeURIComponent(experiment.id)}${entry ? `?entry=${encodeURIComponent(entry)}` : ''}`}
                className={buttonStyles({ variant: 'ghost' })}
              >
                {t('keep')}
              </HistoryLink>
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}
function LegacyVoiceChecks({ ownerId, voiceId }: { ownerId: string; voiceId: string }) {
  const { t } = useWritingTestTranslation()
  const query = useVoiceChecks(ownerId, voiceId)
  const checksKey = useVoiceChecksQueryKey(ownerId, voiceId)
  const refresh = useMemo(() => [checksKey], [checksKey])
  const job = useJob(query.activeJobId, refresh)
  return (
    <section aria-labelledby="legacy-voice-check-title" className="min-w-0">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <Typography variant="title" id="legacy-voice-check-title">
          {t('factor.voice')} · {t('legacy')}
        </Typography>
        <Button variant="ghost" pending={query.isPending} onClick={query.refetch}>
          {t('refresh')}
        </Button>
      </div>
      <Typography variant="body" className="text-content-secondary mt-2">
        {t('legacyReadOnly')}
      </Typography>
      {query.activeJobId && (
        <div className="mt-4">
          {job.isError ? (
            <FailureNotice message={t('failed')} onRetry={job.refetch} />
          ) : job.job && !isTerminal(job.job) ? (
            <ProgressLine job={job.job} />
          ) : null}
        </div>
      )}
      {query.isPending && (
        <Notice tone="info" role="status" className="mt-4">
          {t('loading')}
        </Notice>
      )}
      {query.isError && (
        <Notice tone="danger" role="alert" className="mt-4">
          {t('failed')}
        </Notice>
      )}
      {!query.isPending && !query.isError && query.checks.length === 0 && (
        <Typography variant="body" className="mt-4">
          {t('historyEmpty')}
        </Typography>
      )}
      <ol className="mt-4 grid gap-0">
        {query.checks
          .toSorted((left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt))
          .map((check) => (
            <li key={check.id} className="border-divider min-w-0 border-b py-5">
              {check.prompt?.text && (
                <Typography variant="fieldTitle" as="h3">
                  {check.prompt.text}
                </Typography>
              )}
              {check.createdAt && (
                <Typography
                  variant="meta"
                  as="time"
                  dateTime={check.createdAt}
                  className="mt-2 block"
                >
                  {formatDateTime(check.createdAt)}
                </Typography>
              )}
              {check.stale && <Badge className="mt-2">{t('legacyOldAnalysis')}</Badge>}
              {(check.answer || check.answerDeleted) && (
                <section className="mt-3" aria-label={t('legacyAnswer')}>
                  <Typography variant="label" as="h4">
                    {t('legacyAnswer')}
                  </Typography>
                  <Typography variant="body" className="mt-1 break-words whitespace-pre-wrap">
                    {check.answerDeleted ? t('legacyAnswerWithdrawn') : check.answer}
                  </Typography>
                </section>
              )}
              {check.piece && (
                <Typography
                  variant="body"
                  as="blockquote"
                  className="mt-3 break-words whitespace-pre-wrap"
                >
                  {check.piece}
                </Typography>
              )}
              {check.status === 'failed' && (
                <div className="mt-2">
                  <FailureNotice
                    failure={check.failure}
                    message={!check.failure ? t('failed') : undefined}
                  />
                </div>
              )}
            </li>
          ))}
      </ol>
    </section>
  )
}
