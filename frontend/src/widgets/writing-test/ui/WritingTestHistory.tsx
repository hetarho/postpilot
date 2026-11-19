import { useWritingTestTranslation } from '@/features/writing-test'
import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type ComponentType,
  type ReactNode,
} from 'react'
import { focusManager } from '@tanstack/react-query'
import { Camera, LayoutTemplate, ListChecks, MessageSquareQuote, PenLine } from 'lucide-react'
import { useExperiments } from '@/entities/model-experiment'
import { useVoiceChecks, useVoiceChecksQueryKey, type VoiceCheck } from '@/entities/voice'
import { ProgressLine, FailureNotice, isTerminal, useJob } from '@/entities/generation-job'
import {
  useWritingTests,
  writingTestCandidateReadable,
  type TestList,
  type WritingTest,
} from '@/entities/writing-test'
import { formatDateTime } from '@/shared/lib'
import {
  Badge,
  Button,
  ListRowIcon,
  LoadingState,
  Notice,
  Sheet,
  SurfaceButton,
  Typography,
  WorkRow,
  WorkRowList,
  typographyStyles,
  useNavigationContext,
  type BadgeTone,
  type NavigationLinkProps,
  type WorkRowLinkProps,
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
/** Calls `onReturn` when the owner comes back to the window, so the list refreshes itself without
 *  a refresh button (MODEL-44). It reads the query client's own focus signal. */
function useOnReturn(onReturn: () => void) {
  const latest = useRef(onReturn)
  useEffect(() => {
    latest.current = onReturn
  })
  useEffect(
    () =>
      focusManager.subscribe((focused) => {
        if (focused) latest.current()
      }),
    [],
  )
}
const ACTIVE_STATUSES: ReadonlySet<WritingTest['status']> = new Set([
  'queued',
  'running',
  'partial',
  'review',
])
function OwnedHistory({
  ownerId,
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
  // A return re-reads the newest page: from a later page it moves back to the first, then reads it.
  const [returned, setReturned] = useState(0)
  const { refetch } = query
  useEffect(() => {
    if (returned) void refetch()
  }, [returned, refetch])
  useOnReturn(() => {
    if (!ownerId) return
    if (cursor) setCursor('')
    setReturned((count) => count + 1)
  })
  const next = query.data?.nextPageToken
  // The authenticated server selects source/style membership without exposing
  // blind contestant refs. Filtering hidden identities here would drop active work.
  const tests = loaded.tests.filter(
    (test) => !stage || (stage === 'voice' ? test.factor === 'voice' : test.modelStage === stage),
  )
  const active = tests.filter((test) => ACTIVE_STATUSES.has(test.status))
  const ended = tests.filter((test) => !ACTIVE_STATUSES.has(test.status))
  const historySearch = new URLSearchParams()
  if (stage) historySearch.set('stage', stage)
  if (sourcePostSlug) historySearch.set('source', sourcePostSlug)
  if (voiceId) historySearch.set('voiceId', voiceId)
  const historyEntry = historySearch.size ? `/tests/history?${historySearch}` : '/tests/history'
  const entrySearch = `?entry=${encodeURIComponent(historyEntry)}`
  const row = (test: WritingTest) => (
    <WritingTestRow
      key={test.id}
      test={test}
      href={testHref?.(test.id) ?? `/tests/${encodeURIComponent(test.id)}${entrySearch}`}
    />
  )
  return (
    <div className="grid gap-10">
      <div className="grid min-w-0 gap-8">
        {query.isError && (
          <Notice tone="danger" role="alert">
            {t('failed')}
          </Notice>
        )}
        {query.isPending && ownerId && (
          <LoadingState variant={loaded.tests.length ? 'inline' : 'table'} label={t('loading')} />
        )}
        {query.isSuccess && tests.length === 0 && (
          <Typography variant="body" className="text-content-tertiary">
            {t('historyEmpty')}
          </Typography>
        )}
        {active.length > 0 && (
          <HistoryGroup heading={t('group.active')} count={active.length}>
            {active.map(row)}
          </HistoryGroup>
        )}
        {ended.length > 0 && (
          <HistoryGroup heading={t('group.ended')} count={ended.length}>
            {ended.map(row)}
          </HistoryGroup>
        )}
        {next && !loaded.tokens.includes(next) && (
          <Button
            variant="secondary"
            className="justify-self-start"
            pending={query.isFetching}
            onClick={() => setCursor(next)}
          >
            {t('more')}
          </Button>
        )}
      </div>
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
    </div>
  )
}
/** One group of the history — 진행 중, 끝난 비교 or 이전 기록 — a heading with its count over its
 *  work rows on one card (THEME-83). */
function HistoryGroup({
  heading,
  count,
  children,
  footer,
}: {
  heading: string
  count: number
  children: ReactNode
  footer?: ReactNode
}) {
  const id = useId()
  return (
    <section aria-labelledby={id} className="min-w-0">
      <h2 id={id} className="mb-3 flex items-baseline gap-2">
        <span className={typographyStyles({ variant: 'title' })}>{heading}</span>
        <span
          className={typographyStyles({ variant: 'label', className: 'text-content-tertiary' })}
        >
          {count}
        </span>
      </h2>
      {footer}
      <WorkRowList label={heading}>{children}</WorkRowList>
    </section>
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
  queued: 'row.status.working',
  running: 'row.status.working',
  partial: 'row.status.partial',
  review: 'row.status.review',
  completed: 'row.status.completed',
  cancelled: 'row.status.cancelled',
  failed: 'row.status.failed',
}
const kindIcons = {
  observe: Camera,
  write: PenLine,
  voice: MessageSquareQuote,
  template: LayoutTemplate,
  guideline: ListChecks,
} as const
/** What a comparison compared: the AI by its stage, or the setting it varied. */
function testKind(test: WritingTest): keyof typeof kindIcons {
  if (test.output === 'observation') return 'observe'
  return test.factor === 'model'
    ? test.modelStage === 'observe'
      ? 'observe'
      : 'write'
    : test.factor
}
/** One comparison as a work row (THEME-83): its kind glyph, `글 쓰는 AI · 2개`, then the state —
 *  진행 중, 고를 차례 or 우승 with the champion's name — and its time; the whole row opens the test.
 *  AI 토너먼트's latest comparisons draw the same row. */
export function WritingTestRow({ test, href }: { test: WritingTest; href: string }) {
  const { t } = useWritingTestTranslation()
  const describedBy = useId()
  const [observedAt] = useState(() => Date.now())
  const observing = test.output === 'observation'
  const kind = testKind(test)
  const Icon = kindIcons[kind]
  const champion = test.revealed
    ? test.candidates.find((candidate) => candidate.id === test.winnerCandidateId)
    : undefined
  const expired =
    (!!test.contentExpiresAt && Date.parse(test.contentExpiresAt) <= observedAt) ||
    (test.status === 'completed' &&
      test.candidates.every((candidate) => !writingTestCandidateReadable(candidate)))
  const ready = test.candidates.filter((candidate) => candidate.status === 'succeeded').length
  const time = test.createdAt || test.updatedAt
  // A running comparison's progress takes the time's place (THEME-83).
  const progress =
    test.status === 'queued' || test.status === 'running' || test.status === 'partial'
      ? t(observing ? 'observation.progress' : 'progress', { ready, total: test.count })
      : test.status === 'review'
        ? t('matchProgress', {
            done: test.matches.filter((match) => match.winnerCandidateId).length,
            total: test.count - 1,
          })
        : undefined
  return (
    <WorkRow
      data-test-id={test.id}
      leading={<ListRowIcon icon={Icon} />}
      title={t('row.title', { kind: t(`row.kind.${kind}`), count: test.count })}
      meta={
        <>
          <Badge tone={tones[test.status]} className="shrink-0">
            {t(labels[test.status])}
          </Badge>
          {champion && (
            <span className="max-w-full min-w-0 truncate">
              {champion.identity?.label ||
                champion.displayLabel ||
                t('contender', { number: test.candidates.indexOf(champion) + 1 })}
            </span>
          )}
          {expired && (
            <span className="min-w-0 break-words">
              {t(observing ? 'observation.expired' : 'expired')}
            </span>
          )}
          {progress ? (
            <span className="shrink-0">{progress}</span>
          ) : (
            time && (
              <time dateTime={time} className="shrink-0">
                {formatDateTime(time)}
              </time>
            )
          )}
        </>
      }
      link={({ className, children }) => (
        <HistoryLink
          href={href}
          className={className}
          aria-label={t('resume')}
          aria-describedby={describedBy}
        >
          <span id={describedBy} className="contents">
            {children}
          </span>
        </HistoryLink>
      )}
    />
  )
}
/** 이전 기록: the paid comparison records and, for one explicit voice, its voice checks — read-only,
 *  each one row. A record opens at its address; a voice check has none, so its row opens it in a
 *  sheet. */
function LegacyRecords({
  ownerId,
  voiceId,
  stage,
  sourcePostSlug,
  entry,
}: Omit<WritingTestHistoryProps, 'testHref'> & { entry: string }) {
  const { t } = useWritingTestTranslation()
  const modelStage = stage === 'voice' ? 'write' : stage
  const source =
    stage === 'voice' || (!stage && voiceId && !sourcePostSlug)
      ? 'voice'
      : stage
        ? 'post'
        : undefined
  const query = useExperiments(modelStage, source, ownerId)
  // Voice checks belong to one explicit voice and never to a post's history.
  const checkVoiceId = voiceId && !sourcePostSlug && (!stage || stage === 'voice') ? voiceId : ''
  const checks = useVoiceChecks(ownerId, checkVoiceId)
  const checksKey = useVoiceChecksQueryKey(ownerId, checkVoiceId)
  const refresh = useMemo(() => [checksKey], [checksKey])
  const job = useJob(checks.activeJobId, refresh)
  const [openCheck, setOpenCheck] = useState<VoiceCheck>()
  const { refetch: refetchExperiments } = query
  const { refetch: refetchChecks } = checks
  useOnReturn(() => {
    void refetchExperiments()
    if (checkVoiceId) refetchChecks()
  })
  const experiments = query.experiments
    .filter(
      (record) =>
        (!modelStage || record.stage === modelStage) &&
        (!source || record.source === source) &&
        (!voiceId || record.voiceId === voiceId) &&
        (!sourcePostSlug || record.postSlug === sourcePostSlug),
    )
    .toSorted((left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt))
  const voiceChecks = checkVoiceId
    ? checks.checks.toSorted(
        (left, right) => Date.parse(right.createdAt) - Date.parse(left.createdAt),
      )
    : []
  const failed = query.isError || (!!checkVoiceId && checks.isError)
  const running = !!checkVoiceId && !!checks.activeJobId
  const count = experiments.length + voiceChecks.length
  // An account with no earlier record never sees this group.
  if (!count && !failed && !running) return null
  const entrySearch = `?entry=${encodeURIComponent(entry)}`
  return (
    <HistoryGroup
      heading={t('group.legacy')}
      count={count}
      footer={
        <>
          <Typography variant="body" className="text-content-secondary mb-3">
            {t('legacyReadOnly')}
          </Typography>
          {running && (
            <div className="mb-3">
              {job.isError ? (
                <FailureNotice message={t('failed')} onRetry={job.refetch} />
              ) : job.job && !isTerminal(job.job) ? (
                <ProgressLine job={job.job} />
              ) : null}
            </div>
          )}
          {failed && (
            <Notice tone="danger" role="alert" className="mb-3">
              {t('failed')}
            </Notice>
          )}
        </>
      }
    >
      {experiments.map((experiment) => (
        <LegacyRow
          key={experiment.id}
          icon={experiment.source === 'voice' ? MessageSquareQuote : kindIcons[experiment.stage]}
          title={experiment.source === 'voice' ? t('factor.voice') : t(`stage.${experiment.stage}`)}
          meta={
            <>
              {experiment.templateName && (
                <span className="max-w-full min-w-0 truncate">{experiment.templateName}</span>
              )}
              {experiment.candidates.some(
                (candidate) => candidate.status === 'succeeded' && !candidate.output,
              ) && <span className="min-w-0 break-words">{t('expired')}</span>}
              {experiment.createdAt && (
                <time dateTime={experiment.createdAt} className="shrink-0">
                  {formatDateTime(experiment.createdAt)}
                </time>
              )}
            </>
          }
          name={t('keep')}
          open={({ className, children, ...aria }) => (
            <HistoryLink
              href={`/tests/records/${encodeURIComponent(experiment.id)}${entrySearch}`}
              className={className}
              {...aria}
            >
              {children}
            </HistoryLink>
          )}
        />
      ))}
      {voiceChecks.map((check) => (
        <LegacyRow
          key={check.id}
          icon={MessageSquareQuote}
          title={check.prompt?.text || t('factor.voice')}
          meta={
            <>
              {check.status === 'failed' && (
                <Badge tone="danger" className="shrink-0">
                  {t('row.status.failed')}
                </Badge>
              )}
              {check.stale && <span className="shrink-0">{t('legacyOldAnalysis')}</span>}
              {check.createdAt && (
                <time dateTime={check.createdAt} className="shrink-0">
                  {formatDateTime(check.createdAt)}
                </time>
              )}
            </>
          }
          name={t('keep')}
          open={({ className, children, ...aria }) => (
            <SurfaceButton
              presentation="content"
              className={className}
              onClick={() => setOpenCheck(check)}
              {...aria}
            >
              {children}
            </SurfaceButton>
          )}
        />
      ))}
      <VoiceCheckSheet check={openCheck} onClose={() => setOpenCheck(undefined)} />
    </HistoryGroup>
  )
}
type LegacyOpenProps = WorkRowLinkProps & {
  'aria-label': string
  'aria-describedby': string
}
/** A read-only earlier record as a work row whose one target carries its old name. */
function LegacyRow({
  icon,
  title,
  meta,
  name,
  open,
}: {
  icon: ComponentType<{ className?: string }>
  title: string
  meta: ReactNode
  name: string
  open: (props: LegacyOpenProps) => ReactNode
}) {
  const describedBy = useId()
  return (
    <WorkRow
      leading={<ListRowIcon icon={icon} />}
      title={title}
      meta={meta}
      link={({ className, children }) =>
        open({
          className,
          'aria-label': name,
          'aria-describedby': describedBy,
          children: (
            <span id={describedBy} className="contents">
              {children}
            </span>
          ),
        })
      }
    />
  )
}
/** A voice check's read-only record, opened from its row: it has no address of its own. */
function VoiceCheckSheet({ check, onClose }: { check?: VoiceCheck; onClose: () => void }) {
  const { t } = useWritingTestTranslation()
  const headingId = useId()
  // The sheet keeps the last record while it closes.
  const [shown, setShown] = useState(check)
  if (check && check !== shown) setShown(check)
  return (
    <Sheet
      open={!!check}
      labelledBy={headingId}
      onClose={onClose}
      header={
        <div className="px-5 pt-5 sm:px-6">
          <Typography variant="title" as="h2" id={headingId} className="break-words">
            {shown?.prompt?.text || t('factor.voice')}
          </Typography>
          {shown?.createdAt && (
            <Typography variant="meta" as="time" dateTime={shown.createdAt} className="mt-2 block">
              {formatDateTime(shown.createdAt)}
            </Typography>
          )}
        </div>
      }
    >
      {shown && (
        <div className="grid gap-4">
          {shown.stale && <Badge className="justify-self-start">{t('legacyOldAnalysis')}</Badge>}
          {(shown.answer || shown.answerDeleted) && (
            <section aria-label={t('legacyAnswer')}>
              <Typography variant="label" as="h3">
                {t('legacyAnswer')}
              </Typography>
              <Typography variant="body" className="mt-1 break-words whitespace-pre-wrap">
                {shown.answerDeleted ? t('legacyAnswerWithdrawn') : shown.answer}
              </Typography>
            </section>
          )}
          {shown.piece && (
            <Typography variant="body" as="blockquote" className="break-words whitespace-pre-wrap">
              {shown.piece}
            </Typography>
          )}
          {shown.status === 'failed' && (
            <FailureNotice
              failure={shown.failure}
              message={!shown.failure ? t('failed') : undefined}
            />
          )}
        </div>
      )}
    </Sheet>
  )
}
