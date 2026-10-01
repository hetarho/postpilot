import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { Pencil } from 'lucide-react'
import {
  DefaultGuidelineDetail,
  GuidelineScopeBadge,
  GuidelineScopeBadges,
  useGuidelineCandidates,
  useGuidelines,
  useUpdateGuidelineCall,
  type BulkReviewOutcome,
  type DefaultGuideline,
  type Guideline,
  type GuidelineCandidate,
  type GuidelineKind,
} from '@/entities/guideline'
import { useSession } from '@/entities/session'
import { CreateGuidelineSheet } from '@/features/create-guideline'
import { DeleteGuidelineButton } from '@/features/delete-guideline'
import { GuidelineEditForm } from '@/features/edit-guideline'
import {
  ApproveGuidelineCandidateButton,
  BulkGuidelineCandidateActions,
  DismissGuidelineCandidateButton,
} from '@/features/review-guideline-candidate'
import {
  DefaultGuidelineSheet,
  StopDefaultGuidelineButton,
} from '@/features/toggle-default-guideline'
import {
  ActionBar,
  Badge,
  Button,
  Disclosure,
  FieldMessage,
  Notice,
  Typography,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** The account's 지침 of one kind: a post's 작문 지침 on /guidelines (GUIDE-20) and a clip's 영상
 *  지침 on /video-guidelines (GUIDE-44). One component with a kind rather than two pages, so the
 *  two screens stay one shape. Composition only — every action is its own feature.
 *
 *  ONE list in the server's order, because that order IS the injection order the writer will see
 *  (GUIDE-14): the 기본 지침 in use first, then the owner's own. Every row is closed to one line —
 *  a name or a title and one badge — because the screen is scanned for which rules apply, and a
 *  rule's text is read only once it is the one in question (GUIDE-47).
 *
 *  Nothing on this screen calls a model or enqueues a job: a guideline is authored text, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5]). */
export function GuidelineDirectory({
  kind,
  createOpen = false,
  onCreateClosed,
}: {
  kind: GuidelineKind
  /** Open an empty new guideline once, as the screen mounts (TMPL-61's 지침 만들기). */
  createOpen?: boolean
  onCreateClosed?: () => void
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { guidelines, defaults, isPending, isError, isFetching, refetch } = useGuidelines(
    ownerId,
    kind,
  )
  const page = kind === 'clip' ? 'clipPage' : 'page'
  const inUse = defaults.filter((guideline) => guideline.enabled)
  // A refused 적용 안함 is said here, not on its row: the write is optimistic, so the row it was
  // pressed on has already left the list when the refusal arrives (GUIDE-43).
  const [stopRefusal, setStopRefusal] = useState('')

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Typography variant="display">{t(`${page}.title`, { ns: 'guidelines' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t(`${page}.description`, { ns: 'guidelines' })}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t(`${page}.loadFailed`, { ns: 'guidelines' })}</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!isError && !isPending && (
        <>
          <div role="status" className="mt-6 empty:hidden">
            {stopRefusal && <FieldMessage>{stopRefusal}</FieldMessage>}
          </div>
          {inUse.length === 0 && guidelines.length === 0 ? (
            <EmptyState kind={kind} />
          ) : (
            <section aria-label={t(`${page}.listAria`, { ns: 'guidelines' })} className="mt-6">
              <Typography variant="body" as="p" className="text-content-secondary">
                {t(`${page}.order`, { ns: 'guidelines' })}
              </Typography>
              <ul className="divide-divider mt-3 divide-y">
                {inUse.map((guideline) => (
                  <DefaultGuidelineItem
                    key={guideline.key}
                    ownerId={ownerId}
                    kind={kind}
                    guideline={guideline}
                    onRefused={setStopRefusal}
                  />
                ))}
                {guidelines.map((guideline) => (
                  <GuidelineRow key={guideline.id} ownerId={ownerId} guideline={guideline} />
                ))}
              </ul>
            </section>
          )}

          <CandidateSection ownerId={ownerId} kind={kind} />

          {/* The page is the list; authoring happens behind these triggers, the shape every
              sibling directory uses (GUIDE-20, THEME-24). `mt-auto` puts the bar below a short
              list and `sticky` keeps it in reach once the list is long enough to scroll. */}
          <ActionBar
            dock="list"
            ariaLabel={t('create.dockAria', { ns: 'guidelines' })}
            className="mt-auto flex gap-2"
          >
            <DefaultGuidelineSheet ownerId={ownerId} kind={kind} />
            <CreateGuidelineSheet
              ownerId={ownerId}
              kind={kind}
              initialOpen={createOpen}
              onClosed={onCreateClosed}
            />
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** A closed row's one line: the name or title cut to the row, then its badge. The whole line is
 *  the disclosure's button, so its name is what a screen reader lists. */
function RowTitle({ label, badge }: { label: string; badge: ReactNode }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      <span className="min-w-0 truncate">{label}</span>
      <span className="shrink-0">{badge}</span>
    </span>
  )
}

/** One 기본 지침 in use (GUIDE-47): closed, its name and 추천; open, its text, its target note and
 *  `적용 안함`. */
function DefaultGuidelineItem({
  ownerId,
  kind,
  guideline,
  onRefused,
}: {
  ownerId: string
  kind: GuidelineKind
  guideline: DefaultGuideline
  onRefused: (message: string) => void
}) {
  const { t } = useTranslation('guidelines')
  return (
    <li>
      <Disclosure
        size="row"
        headingLevel={3}
        title={
          <RowTitle
            label={guideline.name}
            badge={<Badge tone="accent">{t('defaults.badge')}</Badge>}
          />
        }
      >
        <div className="pb-3 pl-6">
          <DefaultGuidelineDetail
            guideline={guideline}
            action={
              <StopDefaultGuidelineButton
                ownerId={ownerId}
                kind={kind}
                guideline={guideline}
                onRefused={onRefused}
              />
            }
          />
        </div>
      </Disclosure>
    </li>
  )
}

/** One of the owner's guidelines (GUIDE-47): closed, its title — or its text when it has none —
 *  and its scope's kind; open, the text, the scope in full, and 수정 · 삭제. 수정 turns the open
 *  row into one form for the title, the text and the scope. */
function GuidelineRow({ ownerId, guideline }: { ownerId: string; guideline: Guideline }) {
  const { t } = useTranslation(['guidelines', 'common'])
  const update = useUpdateGuidelineCall(ownerId, guideline.id, guideline.kind)
  const [editing, setEditing] = useState(false)
  return (
    <li>
      <Disclosure
        size="row"
        headingLevel={3}
        // Closing a row leaves its form: the next opening reads the saved rule first.
        onOpenChange={(open) => {
          if (!open) setEditing(false)
        }}
        title={
          <RowTitle
            label={guideline.title || guideline.text}
            badge={<GuidelineScopeBadge guideline={guideline} />}
          />
        }
      >
        <div className="pb-3 pl-6">
          {editing ? (
            <GuidelineEditForm
              ownerId={ownerId}
              guideline={guideline}
              save={update.save}
              errorMessage={update.errorMessage}
              pending={update.isPending}
              onDone={() => setEditing(false)}
            />
          ) : (
            <>
              <div className="flex flex-wrap items-start gap-1">
                <Typography
                  variant="body"
                  className="text-content-primary min-w-0 flex-1 pt-2 whitespace-pre-wrap"
                >
                  {guideline.text}
                </Typography>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label={t('action.editNamed', {
                    ns: 'common',
                    name: t('edit.text', { ns: 'guidelines' }),
                  })}
                  onClick={() => setEditing(true)}
                  className="shrink-0"
                >
                  <Pencil className="size-4" aria-hidden />
                </Button>
                <DeleteGuidelineButton
                  ownerId={ownerId}
                  kind={guideline.kind}
                  guidelineId={guideline.id}
                />
              </div>
              {/* 전역 is already the closed line's whole story; a narrower scope names its members
                  here, and a scope that reaches nothing says what to do about it. */}
              {guideline.scope !== 'global' && (
                <div className="mt-1">
                  <GuidelineScopeBadges guideline={guideline} />
                </div>
              )}
            </>
          )}
        </div>
      </Disclosure>
    </li>
  )
}

/** The 후보 queue: what completed revisions recorded, waiting to be reviewed (GUIDE-22).
 *
 *  Closed and below the saved list, because the saved rules are what the screen is for and an
 *  unreviewed suggestion is not a form. The summary carries the count, which is the whole reason
 *  to open it.
 *
 *  Nothing here is learned. Each row is one instruction the user typed, recorded verbatim, and it
 *  reaches no prompt until it is approved with a scope. A load failure renders nothing rather than
 *  a second error region: the saved list already owns the page's error state, and the candidates
 *  are an addition to this screen, not its subject. */
function CandidateSection({ ownerId, kind }: { ownerId: string; kind: GuidelineKind }) {
  const { t } = useTranslation('guidelines')
  const { candidates, queueFull, isError } = useGuidelineCandidates(ownerId, kind)
  // A bulk run's refusals belong to the rows they were refused for, so the page holds them and
  // hands each row its own. They are cleared whenever the queue is re-read.
  const [failures, setFailures] = useState<BulkReviewOutcome['failures']>([])
  // A finished run OUTLIVES the queue it emptied: accepting everything takes the rows and their
  // buttons away, and what the run did still has to be readable (THEME-24 — the live region is
  // mounted before its text changes).
  const [outcome, setOutcome] = useState<BulkOutcome | null>(null)
  const live = failures.filter((failure) => candidates.some((row) => row.id === failure.id))
  const waiting = candidates.length > 0 || queueFull

  // Nothing waiting, room to record more and nothing just done is the ordinary state, and it
  // needs no words.
  if (isError || (!waiting && !outcome)) return null

  return (
    <div className="mt-10">
      <Typography variant="meta" as="p" role="status">
        {outcome ? bulkReport(t, outcome) : ''}
      </Typography>
      {waiting && (
        <CandidateQueue
          ownerId={ownerId}
          kind={kind}
          candidates={candidates}
          queueFull={queueFull}
          failures={live}
          onFailures={setFailures}
          onFinished={setOutcome}
        />
      )}
    </div>
  )
}

interface BulkOutcome {
  kind: 'approve' | 'dismiss'
  moved: number
  attempted: number
}

/** What the run did, in the user's words: an acceptance says how many became rules and how many
 *  were left behind for a reason on their own row. */
function bulkReport(t: TFunction<'guidelines'>, outcome: BulkOutcome) {
  return outcome.kind === 'approve'
    ? t('candidate.approveAllResult', {
        count: outcome.moved,
        left: outcome.attempted - outcome.moved,
      })
    : t('candidate.dismissAllResult', { count: outcome.moved })
}

function CandidateQueue({
  ownerId,
  kind,
  candidates,
  queueFull,
  failures,
  onFailures,
  onFinished,
}: {
  ownerId: string
  kind: GuidelineKind
  candidates: readonly GuidelineCandidate[]
  queueFull: boolean
  failures: BulkReviewOutcome['failures']
  onFailures: (failures: BulkReviewOutcome['failures']) => void
  onFinished: (outcome: BulkOutcome) => void
}) {
  const { t } = useTranslation('guidelines')
  return (
    <details>
      <summary
        className={typographyStyles({
          variant: 'label',
          className:
            'active:bg-row-bg-active text-content-secondary min-h-11 cursor-pointer rounded-md px-4 py-3 select-none',
        })}
      >
        {/* A full queue stops recording corrections, so the closed summary says so from the
            server's queue_full (GUIDE-22); the notice inside says what to do about it. */}
        {kind === 'clip'
          ? t(queueFull ? 'candidate.summaryClipFull' : 'candidate.summaryClip', {
              count: candidates.length,
            })
          : t(queueFull ? 'candidate.summaryFull' : 'candidate.summary', {
              count: candidates.length,
            })}
      </summary>
      {/* Named by what it is, not by the summary's count, so the region a screen reader lands in
          keeps the same name as the queue empties. */}
      <section aria-label={t('candidate.section')} className="mt-2 px-4">
        <Typography variant="body" as="p" className="text-content-secondary max-w-measure">
          {t(kind === 'clip' ? 'candidate.sectionHelpClip' : 'candidate.sectionHelp')}
        </Typography>
        {/* The one thing an empty result cannot tell the user: recording has stopped, and clearing
            a row is what starts it again. The bound itself is the server's and is not mirrored. */}
        {queueFull && (
          <Notice tone="warning" role="status" className="mt-3">
            {t('candidate.queueFull')}
          </Notice>
        )}
        <BulkGuidelineCandidateActions
          ownerId={ownerId}
          kind={kind}
          candidates={candidates}
          onFailures={onFailures}
          onFinished={onFinished}
        />
        {candidates.length > 0 && (
          <ul className="divide-divider mt-3 divide-y">
            {candidates.map((candidate) => (
              <CandidateRow
                key={candidate.id}
                ownerId={ownerId}
                candidate={candidate}
                failure={failures.find((failure) => failure.id === candidate.id)?.message}
              />
            ))}
          </ul>
        )}
      </section>
    </details>
  )
}

/** One recorded instruction: its text, how often it was asked for, and where it came from.
 *
 *  The occurrence count is the signal the old on-the-spot button could not give — five identical
 *  corrections read as a standing rule. The source post or clip is a link only while it exists; a
 *  deleted one leaves the text and drops the link, so the row stays reviewable either way. */
function CandidateRow({
  ownerId,
  candidate,
  failure,
}: {
  ownerId: string
  candidate: GuidelineCandidate
  /** Why a bulk acceptance could not save this one. It stays here until the queue is re-read. */
  failure?: string
}) {
  const { t } = useTranslation('guidelines')
  return (
    <li className="py-4">
      <Typography variant="body" className="text-content-primary whitespace-pre-wrap">
        {candidate.text}
      </Typography>
      <div className="mt-2 flex flex-wrap items-center gap-2">
        {candidate.occurrences > 1 && (
          <Badge tone="accent">
            {t('candidate.occurrences', { count: candidate.occurrences })}
          </Badge>
        )}
        {candidate.kind === 'clip' ? (
          candidate.clipId ? (
            <Link
              to="/clips/$clipId"
              params={{ clipId: candidate.clipId }}
              className={sourceLinkStyles}
            >
              {t('candidate.sourceClip')}
            </Link>
          ) : (
            <Typography variant="meta" as="span" className="min-w-0 truncate">
              {t('candidate.sourceClipGone')}
            </Typography>
          )
        ) : candidate.postSlug ? (
          <Link
            to="/posts/$slug"
            params={{ slug: candidate.postSlug }}
            className={sourceLinkStyles}
          >
            {t('candidate.source')}
          </Link>
        ) : (
          <Typography variant="meta" as="span" className="min-w-0 truncate">
            {t('candidate.sourceGone')}
          </Typography>
        )}
      </div>
      {failure && <FieldMessage className="mt-2">{failure}</FieldMessage>}
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <ApproveGuidelineCandidateButton ownerId={ownerId} candidate={candidate} />
        <DismissGuidelineCandidateButton
          ownerId={ownerId}
          kind={candidate.kind}
          candidateId={candidate.id}
        />
      </div>
    </li>
  )
}

/** The worked example is copy, not a row: nothing here creates a guideline the user did not
 *  author (GUIDE-18, GUIDE-19 — no seeded library, no inference). */
function EmptyState({ kind }: { kind: GuidelineKind }) {
  const { t } = useTranslation('guidelines')
  const page = kind === 'clip' ? 'clipPage' : 'page'
  return (
    <section aria-labelledby="guidelines-empty-heading" className="mt-8">
      <Typography variant="title" id="guidelines-empty-heading">
        {t(`${page}.empty`)}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t(`${page}.emptyHelp`)}
      </Typography>
      <Typography variant="body" className="text-content-primary mt-3">
        {t(`${page}.example`)}
      </Typography>
    </section>
  )
}

const sourceLinkStyles = typographyStyles({
  variant: 'label',
  className: 'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center',
})
