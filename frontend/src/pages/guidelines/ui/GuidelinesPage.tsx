import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import {
  useGuidelineCandidates,
  useGuidelines,
  useUpdateGuidelineCall,
  type BulkReviewOutcome,
  type Guideline,
  type GuidelineCandidate,
} from '@/entities/guideline'
import { useSession } from '@/entities/session'
import { CreateGuidelineSheet } from '@/features/create-guideline'
import { DeleteGuidelineButton } from '@/features/delete-guideline'
import { EditableGuidelineScope, EditableGuidelineText } from '@/features/edit-guideline'
import {
  ApproveGuidelineCandidateButton,
  BulkGuidelineCandidateActions,
  DismissGuidelineCandidateButton,
} from '@/features/review-guideline-candidate'
import {
  ActionBar,
  Badge,
  Button,
  FieldMessage,
  Notice,
  Typography,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** The account's 작문 지침 (plan 16). Composition only — every action is its own feature.
 *
 *  Nothing on this screen calls a model or enqueues a job: a guideline is authored text, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5]). The list is rendered in the
 *  server's order because that order IS the injection order the writer will see. */
export function GuidelinesPage() {
  const { t } = useTranslation(['guidelines', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { guidelines, isPending, isError, isFetching, refetch } = useGuidelines(ownerId)

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Typography variant="display">{t('title', { ns: 'guidelines' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.description', { ns: 'guidelines' })}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed', { ns: 'guidelines' })}</span>
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
          {guidelines.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="guidelines-heading" className="mt-8">
              <Typography variant="title" id="guidelines-heading">
                {t('page.saved', { ns: 'guidelines' })}
              </Typography>
              <Typography variant="body" as="p" className="text-content-secondary mt-1">
                {t('page.order', { ns: 'guidelines' })}
              </Typography>
              <ul className="divide-divider mt-3 divide-y">
                {guidelines.map((guideline) => (
                  <GuidelineRow key={guideline.id} ownerId={ownerId} guideline={guideline} />
                ))}
              </ul>
            </section>
          )}

          <CandidateSection ownerId={ownerId} />

          {/* The page is the list; authoring happens behind this one trigger, the shape every
              sibling directory uses (GUIDE-20, THEME-24). `mt-auto` puts the bar below a short
              list and `sticky` keeps it in reach once the list is long enough to scroll. */}
          <ActionBar
            dock="list"
            ariaLabel={t('create.dockAria', { ns: 'guidelines' })}
            className="mt-auto"
          >
            <CreateGuidelineSheet ownerId={ownerId} className="w-full sm:w-auto" />
          </ActionBar>
        </>
      )}
    </main>
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
function CandidateSection({ ownerId }: { ownerId: string }) {
  const { t } = useTranslation('guidelines')
  const { candidates, queueFull, isError } = useGuidelineCandidates(ownerId)
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
  candidates,
  queueFull,
  failures,
  onFailures,
  onFinished,
}: {
  ownerId: string
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
        {t('candidate.summary', { count: candidates.length })}
      </summary>
      {/* Named by what it is, not by the summary's count, so the region a screen reader lands in
          keeps the same name as the queue empties. */}
      <section aria-label={t('candidate.section')} className="mt-2 px-4">
        <Typography variant="body" as="p" className="text-content-secondary max-w-measure">
          {t('candidate.sectionHelp')}
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
 *  corrections read as a standing rule. The source post is a link only while it exists; a deleted
 *  post leaves the text and drops the link, so the row stays reviewable either way. */
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
        {candidate.postSlug ? (
          <Link
            to="/posts/$slug"
            params={{ slug: candidate.postSlug }}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 min-w-0 items-center',
            })}
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
        <DismissGuidelineCandidateButton ownerId={ownerId} candidateId={candidate.id} />
      </div>
    </li>
  )
}

/** The worked example is copy, not a row: nothing here creates a guideline the user did not
 *  author (plan 16 — no seeded library, no inference). */
function EmptyState() {
  const { t } = useTranslation('guidelines')
  return (
    <section aria-labelledby="guidelines-empty-heading" className="mt-8">
      <Typography variant="title" id="guidelines-empty-heading">
        {t('page.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.emptyHelp')}
      </Typography>
      <Typography variant="body" className="text-content-primary mt-3">
        {t('page.example')}
      </Typography>
    </section>
  )
}

/** One saved guideline: its text and its scope, each read-first and each saving on its own so the
 *  two edited from two places cannot overwrite each other. One mutation hook serves both — they
 *  never run at the same time, and sharing it keeps one refusal message under one field. */
function GuidelineRow({ ownerId, guideline }: { ownerId: string; guideline: Guideline }) {
  const update = useUpdateGuidelineCall(ownerId, guideline.id)
  return (
    <li className="py-4">
      <EditableGuidelineText
        value={guideline.text}
        save={update.saveText}
        errorMessage={update.errorMessage}
        pending={update.isPending}
      />
      <EditableGuidelineScope
        ownerId={ownerId}
        guideline={guideline}
        save={update.saveScope}
        errorMessage={update.errorMessage}
        pending={update.isPending}
        className="mt-3"
      />
      <div className="mt-4 flex flex-wrap items-center gap-2">
        <DeleteGuidelineButton ownerId={ownerId} guidelineId={guideline.id} />
      </div>
    </li>
  )
}
