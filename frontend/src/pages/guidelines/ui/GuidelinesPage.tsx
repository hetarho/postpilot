import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import {
  useGuidelineCandidates,
  useGuidelines,
  useUpdateGuidelineCall,
  type Guideline,
  type GuidelineCandidate,
} from '@/entities/guideline'
import { useSession } from '@/entities/session'
import { CreateGuidelineForm } from '@/features/create-guideline'
import { DeleteGuidelineButton } from '@/features/delete-guideline'
import { EditableGuidelineScope, EditableGuidelineText } from '@/features/edit-guideline'
import {
  ApproveGuidelineCandidateButton,
  DismissGuidelineCandidateButton,
} from '@/features/review-guideline-candidate'
import { Badge, Button, Notice, Typography, pageStyles, typographyStyles } from '@/shared/ui'

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
    <main className={pageStyles({ width: 'wide' })}>
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
          <CandidateSection ownerId={ownerId} />
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

          <section aria-labelledby="create-guideline-heading" className="mt-10">
            <Typography variant="title" id="create-guideline-heading">
              {t('page.new', { ns: 'guidelines' })}
            </Typography>
            <CreateGuidelineForm ownerId={ownerId} className="mt-3" />
          </section>
        </>
      )}
    </main>
  )
}

/** The 후보 section: what completed revisions recorded, waiting to be reviewed in a batch
 *  (change 26). It sits ABOVE the saved list because it is the thing that has changed since the
 *  user last looked, and the saved list is reference.
 *
 *  Nothing here is learned. Each row is one instruction the user typed, recorded verbatim, and it
 *  reaches no prompt until it is approved with a scope. A load failure renders nothing rather than
 *  a second error region: the saved list above already owns the page's error state, and the
 *  candidates are an addition to this screen, not its subject. */
function CandidateSection({ ownerId }: { ownerId: string }) {
  const { t } = useTranslation('guidelines')
  const { candidates, queueFull, isError } = useGuidelineCandidates(ownerId)

  // Nothing waiting and room to record more is the ordinary state, and it needs no words.
  if (isError || (candidates.length === 0 && !queueFull)) return null

  return (
    <section aria-labelledby="guideline-candidates-heading" className="mt-8">
      <Typography variant="title" id="guideline-candidates-heading">
        {t('candidate.section')}
      </Typography>
      <Typography variant="body" as="p" className="text-content-secondary max-w-measure mt-1">
        {t('candidate.sectionHelp')}
      </Typography>
      {/* The one thing an empty result cannot tell the user: recording has stopped, and clearing
          a row is what starts it again. The bound itself is the server's and is not mirrored. */}
      {queueFull && (
        <Notice tone="warning" role="status" className="mt-3">
          {t('candidate.queueFull')}
        </Notice>
      )}
      {candidates.length > 0 && (
        <ul className="divide-divider mt-3 divide-y">
          {candidates.map((candidate) => (
            <CandidateRow key={candidate.id} ownerId={ownerId} candidate={candidate} />
          ))}
        </ul>
      )}
    </section>
  )
}

/** One recorded instruction: its text, how often it was asked for, and where it came from.
 *
 *  The occurrence count is the signal the old on-the-spot button could not give — five identical
 *  corrections read as a standing rule. The source post is a link only while it exists; a deleted
 *  post leaves the text and drops the link, so the row stays reviewable either way. */
function CandidateRow({ ownerId, candidate }: { ownerId: string; candidate: GuidelineCandidate }) {
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
