import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { useId, useRef, useState, type ReactNode, type Ref } from 'react'
import { clsx } from 'clsx'
import { Check, Sparkles } from 'lucide-react'
import {
  DefaultGuidelineDetail,
  GuidelineScopeBadge,
  GuidelineScopeBadges,
  useGuidelineCandidates,
  useGuidelines,
  type BulkReviewOutcome,
  type DefaultGuideline,
  type Guideline,
  type GuidelineCandidate,
  type GuidelineKind,
} from '@/entities/guideline'
import { useSession } from '@/entities/session'
import { AuthoringSheet } from '@/features/ai-authoring'
import { DeleteGuidelineButton } from '@/features/delete-guideline'
import { GuidelineDirectEditor } from '@/features/edit-guideline'
import {
  useAuthoringSummaries,
  type AuthoringSummary,
  type AuthoringSavedRef,
} from '@/entities/ai-authoring'
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
  Badge,
  Button,
  Disclosure,
  FieldMessage,
  Notice,
  Sheet,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
  useListRemovalFocus,
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
  dock,
}: {
  kind: GuidelineKind
  /** Open an empty new guideline — as the screen mounts (TMPL-61's 지침 만들기) or when the
   *  section's 새 지침 button asks for one on this screen (THEME-67). */
  createOpen?: boolean
  onCreateClosed?: () => void
  /** The page's one dock: the section's creation button (THEME-67). */
  dock?: ReactNode
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { guidelines, defaults, isPending, isError, isFetching, refetch } = useGuidelines(
    ownerId,
    kind,
  )
  const page = kind === 'clip' ? 'clipPage' : 'page'
  // 원칙 always apply and are shown read-only in their own card (GUIDE-53); the list holds the
  // 기본 지침 in use and the owner's own.
  const principles = defaults.filter((guideline) => guideline.principle)
  const inUse = defaults.filter((guideline) => !guideline.principle && guideline.enabled)
  // The two kinds of row may carry the same raw id. Keep their mixed list identities distinct,
  // in the exact order already rendered, and focus the existing disclosure rather than an action
  // hidden inside a closed row.
  const rowKeys = [
    ...inUse.map((guideline) => `default:${guideline.key}`),
    ...guidelines.map((guideline) => `owned:${guideline.id}`),
  ]
  const disclosures = useRef(new Map<string, HTMLButtonElement>())
  const defaultAction = useRef<HTMLButtonElement | null>(null)
  const requestRemoval = useListRemovalFocus({
    keys: rowKeys,
    getTarget: (key) => disclosures.current.get(key),
    getFallback: () => defaultAction.current,
  })
  const rememberDisclosure = (key: string, row: HTMLLIElement | null) => {
    const button = row?.querySelector<HTMLButtonElement>('button[aria-expanded]')
    if (button) disclosures.current.set(key, button)
    else disclosures.current.delete(key)
  }
  // A refused 적용 안함 is said here, not on its row: the write is optimistic, so the row it was
  // pressed on has already left the list when the refusal arrives (GUIDE-43).
  const [stopRefusal, setStopRefusal] = useState('')
  const [authoringOpen, setAuthoringOpen] = useState(createOpen)
  const [createMethod, setCreateMethod] = useState<'ai' | 'direct'>('ai')
  const [creationSession, setCreationSession] = useState<string>()
  const [savedOutcome, setSavedOutcome] = useState<{
    ownerId: string
    ref: AuthoringSavedRef
  }>()
  const summaries = useAuthoringSummaries({
    ownerId,
    kind: kind === 'clip' ? 'video-guideline' : 'post-guideline',
  })
  const { t: authoringText } = useTranslation('authoring')
  // The 새 지침 button can ask for one while this screen is already open: a request arriving
  // after mount opens an empty AI-authored one, as one present at mount does.
  const [createRequest, setCreateRequest] = useState(createOpen)
  if (createOpen !== createRequest) {
    setCreateRequest(createOpen)
    if (createOpen) {
      setCreationSession(undefined)
      setCreateMethod('ai')
      setAuthoringOpen(true)
    }
  }

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}>
      {/* Guide: the hub's large title and the 지침 chip say where this is; the page leads with
          what the list IS — the plain sentences the AI always keeps to. */}
      <h1 className="sr-only">{t(`${page}.title`, { ns: 'guidelines' })}</h1>
      <Typography variant="display" as="p" aria-hidden="true" className="text-balance">
        {t(`${page}.always`, { ns: 'guidelines' })}
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
          {/* Guide: 후보 lead the page as a banner — the one thing here that waits on the owner —
              and open a one-at-a-time review rather than unfolding a list under the rules. */}
          <CandidateSection ownerId={ownerId} kind={kind} />
          {/* 새 지침 itself is the section's creation button (THEME-67); the 기본 지침 sheet and
              writing one directly stay on the page, above the list (GUIDE-44). */}
          <div
            ref={(actions) => {
              // The existing first action opens the default-guidelines sheet. Read its actual
              // button without adding a wrapper, translated selector or another control.
              defaultAction.current = actions?.querySelector<HTMLButtonElement>('button') ?? null
            }}
            className="mt-6 flex flex-wrap items-center gap-3"
          >
            <DefaultGuidelineSheet ownerId={ownerId} kind={kind} />
            <Button
              variant="secondary"
              onClick={() => {
                setCreationSession(undefined)
                setCreateMethod('direct')
                setAuthoringOpen(true)
              }}
            >
              {t(
                kind === 'clip'
                  ? 'authoringState.createVideoDirect'
                  : 'authoringState.createDirect',
                { ns: 'guidelines' },
              )}
            </Button>
            <AuthoringSheet
              open={authoringOpen}
              onOpenChange={(open) => {
                setAuthoringOpen(open)
                if (!open) onCreateClosed?.()
              }}
              ownerId={ownerId}
              kind={kind === 'clip' ? 'video-guideline' : 'post-guideline'}
              sessionId={creationSession}
              startFromSaved={!creationSession}
              initialMethod={createMethod}
              renderDirectEditor={(props) => (
                <GuidelineDirectEditor ownerId={ownerId} kind={kind} {...props} />
              )}
              onSaved={(saved) => {
                setSavedOutcome({ ownerId, ref: saved })
                refetch()
                setAuthoringOpen(false)
                onCreateClosed?.()
              }}
            />
          </div>
          {summaries.isPending && (
            <Typography variant="body" role="status" className="mt-6">
              {t('authoringState.checking', { ns: 'guidelines' })}
            </Typography>
          )}
          {summaries.isError && (
            <Notice tone="danger" role="alert" className="mt-6">
              {t('authoringState.failed', { ns: 'guidelines' })}
              <Button variant="ghost" onClick={() => void summaries.refetch()}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </Notice>
          )}
          <div role="status" className="mt-6 empty:hidden">
            {stopRefusal && <FieldMessage>{stopRefusal}</FieldMessage>}
            {savedOutcome?.ownerId === ownerId && savedOutcome.ref.outcome && (
              <Typography variant="body">
                {authoringText(
                  savedOutcome.ref.outcome === 'created' ? 'confirmedCreate' : 'confirmedUpdate',
                  {
                    kind: authoringText(`kinds.${savedOutcome.ref.kind}`),
                    name: savedOutcome.ref.name,
                  },
                )}
              </Typography>
            )}
          </div>
          {principles.length > 0 && <PrincipleCard principles={principles} />}
          {inUse.length === 0 && guidelines.length === 0 ? (
            <EmptyState kind={kind} />
          ) : (
            <section aria-label={t(`${page}.listAria`, { ns: 'guidelines' })} className="mt-6">
              <Typography variant="meta" as="p">
                {t(`${page}.order`, { ns: 'guidelines' })}
              </Typography>
              {/* Guide: each rule is one card holding its plain sentence; tapping it opens the
                  rule's own reading view with its scope and the way to edit it. */}
              <ul className="mt-3 grid gap-3">
                {inUse.map((guideline, index) => (
                  <DefaultGuidelineItem
                    key={guideline.key}
                    ownerId={ownerId}
                    kind={kind}
                    guideline={guideline}
                    onRefused={setStopRefusal}
                    rowRef={(row) => rememberDisclosure(rowKeys[index], row)}
                    onRemoving={() => requestRemoval(rowKeys[index], index)}
                  />
                ))}
                {guidelines.map((guideline, index) => (
                  <GuidelineRow
                    key={guideline.id}
                    ownerId={ownerId}
                    guideline={guideline}
                    rowRef={(row) => rememberDisclosure(rowKeys[inUse.length + index], row)}
                    onDeleted={() =>
                      requestRemoval(rowKeys[inUse.length + index], inUse.length + index)
                    }
                    summary={summaries.data?.find(
                      (row) =>
                        row.targetId === guideline.id || row.lastPublication?.id === guideline.id,
                    )}
                    onPublished={(saved) => {
                      void refetch()
                      setSavedOutcome({ ownerId, ref: saved })
                    }}
                  />
                ))}
              </ul>
            </section>
          )}

          {summaries.data?.some((row) => !row.targetId && !row.lastPublication) && (
            <section className="mt-8" aria-labelledby="guideline-unsaved-heading">
              <Typography variant="title" as="h2" id="guideline-unsaved-heading">
                {t('authoringState.unsavedHeading', { ns: 'guidelines' })}
              </Typography>
              <ul className="mt-3 space-y-3">
                {summaries.data
                  .filter((row) => !row.targetId && !row.lastPublication)
                  .map((row) => (
                    <li key={row.sessionId}>
                      <Button
                        variant="ghost"
                        onClick={() => {
                          setCreationSession(row.sessionId)
                          setCreateMethod('ai')
                          setAuthoringOpen(true)
                        }}
                      >
                        {authoringText('continueNamed', {
                          kind: authoringText(
                            `kinds.${kind === 'clip' ? 'video-guideline' : 'post-guideline'}`,
                          ),
                          name:
                            row.displayName ||
                            t(kind === 'clip' ? 'create.openClip' : 'create.open', {
                              ns: 'guidelines',
                            }),
                        })}
                      </Button>
                      <Typography variant="body" className="text-content-secondary">
                        {t('authoringState.unsaved', { ns: 'guidelines' })}
                      </Typography>
                      <EditingStatus summary={row} />
                    </li>
                  ))}
              </ul>
            </section>
          )}

          {dock}
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

/** Guide: a rule as a plain sentence — a check, the rule's own name above (when it has one), the
 *  sentence itself wrapping in full, and its badge. The name leads, so the row's accessible name
 *  starts with what the rule is called. */
function SentenceTitle({
  sentence,
  name,
  badge,
}: {
  sentence: string
  name?: string
  badge: ReactNode
}) {
  return (
    <span className="flex min-w-0 items-start gap-3 py-1">
      <span
        aria-hidden="true"
        className="bg-badge-accent-bg text-badge-accent-fg mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full"
      >
        <Check className="size-4" />
      </span>
      <span className="flex min-w-0 flex-col gap-1">
        {name && name !== sentence && (
          <span className={typographyStyles({ variant: 'label', className: 'truncate' })}>
            {name}
          </span>
        )}
        <span
          className={typographyStyles({
            variant: 'fieldTitle',
            className: 'text-content-primary text-pretty break-words',
          })}
        >
          {sentence}
        </span>
        <span className="flex min-w-0 flex-wrap items-center gap-2">{badge}</span>
      </span>
    </span>
  )
}

/** The card a rule row sits on. */
const ruleCard = cardStyles({ padding: 'none', className: 'px-4 shadow-sm sm:px-5' })

/** The kind's 원칙 (GUIDE-53): one closed card above the list whose summary says they always
 *  apply; open, each 원칙's name and text, with no control — a 원칙 has no switch. */
function PrincipleCard({ principles }: { principles: DefaultGuideline[] }) {
  const { t } = useTranslation('guidelines')
  return (
    <section className="mt-6">
      <Disclosure
        size="row"
        headingLevel={2}
        title={
          <RowTitle
            label={t('principles.title')}
            badge={<Badge>{t('principles.summary')}</Badge>}
          />
        }
      >
        <div className="pb-3 pl-6">
          <Typography variant="body" className="text-content-secondary">
            {t('principles.help')}
          </Typography>
          <ul aria-label={t('principles.listAria')} className="mt-3 space-y-3">
            {principles.map((principle) => (
              <li key={principle.key}>
                <Typography variant="body" as="p" className="text-content-primary">
                  {principle.name}
                </Typography>
                <DefaultGuidelineDetail guideline={principle} />
              </li>
            ))}
          </ul>
        </div>
      </Disclosure>
    </section>
  )
}

/** One 기본 지침 in use (GUIDE-47): closed, its name and 추천; open, its text, its target note and
 *  `적용 안함`. */
function DefaultGuidelineItem({
  ownerId,
  kind,
  guideline,
  onRefused,
  rowRef,
  onRemoving,
}: {
  ownerId: string
  kind: GuidelineKind
  guideline: DefaultGuideline
  onRefused: (message: string) => void
  rowRef: Ref<HTMLLIElement>
  onRemoving: () => () => void
}) {
  const { t } = useTranslation('guidelines')
  return (
    <li ref={rowRef} className={ruleCard}>
      <Disclosure
        size="row"
        headingLevel={3}
        title={
          <SentenceTitle
            sentence={guideline.name}
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
                onRemoving={onRemoving}
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
function GuidelineRow({
  ownerId,
  guideline,
  onPublished,
  summary,
  rowRef,
  onDeleted,
}: {
  ownerId: string
  guideline: Guideline
  onPublished: (ref: AuthoringSavedRef) => void
  summary?: AuthoringSummary
  rowRef: Ref<HTMLLIElement>
  onDeleted: () => void
}) {
  const { t } = useTranslation('guidelines')
  const { t: authoringText } = useTranslation('authoring')
  const [method, setMethod] = useState<'ai' | 'direct'>('ai')
  const [authoringOpen, setAuthoringOpen] = useState(false)
  const [sessionId, setSessionId] = useState<string>()
  const kind = guideline.kind === 'clip' ? 'video-guideline' : 'post-guideline'
  const named = { kind: authoringText(`kinds.${kind}`), name: guideline.title || guideline.text }
  const open = (method: 'ai' | 'direct', session?: string) => {
    setMethod(method)
    setSessionId(session)
    setAuthoringOpen(true)
  }
  return (
    <li ref={rowRef} className={ruleCard}>
      <Disclosure
        size="row"
        headingLevel={3}
        title={
          <SentenceTitle
            name={guideline.title}
            sentence={guideline.text}
            badge={<GuidelineScopeBadge guideline={guideline} />}
          />
        }
      >
        <div className="pb-3 pl-6">
          <Typography variant="body" className="text-content-primary whitespace-pre-wrap">
            {guideline.text}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-3">
            {authoringText('savedUsable', named)}
          </Typography>
          <GuidelineScopeBadges guideline={guideline} />
          <div className="mt-4 flex flex-wrap gap-3">
            <Button variant="secondary" onClick={() => open('ai')}>
              {authoringText('aiNamed', named)}
            </Button>
            <Button variant="secondary" onClick={() => open('direct')}>
              {authoringText('directNamed', named)}
            </Button>
            {summary &&
              (summary.hasUnpublishedChanges ||
                summary.activeJobId ||
                summary.publicationPending ||
                summary.targetConflict) && (
                <Button variant="ghost" onClick={() => open('ai', summary.sessionId)}>
                  {authoringText('continueNamed', named)}
                </Button>
              )}
            <DeleteGuidelineButton
              ownerId={ownerId}
              kind={guideline.kind}
              guidelineId={guideline.id}
              onDeleted={onDeleted}
            />
          </div>
          <AuthoringSheet
            open={authoringOpen}
            onOpenChange={setAuthoringOpen}
            ownerId={ownerId}
            kind={kind}
            targetId={guideline.id}
            targetName={named.name}
            sessionId={sessionId}
            startFromSaved={!sessionId}
            initialMethod={method}
            renderDirectEditor={(props) => (
              <GuidelineDirectEditor ownerId={ownerId} kind={guideline.kind} {...props} />
            )}
            onSaved={(saved) => {
              onPublished(saved)
              setAuthoringOpen(false)
            }}
          />
        </div>
      </Disclosure>
      <div className="pb-3 pl-6">
        <Typography variant="body" className="text-content-secondary">
          {t('authoringState.savedAvailable')}
        </Typography>
        <EditingStatus summary={summary} />
      </div>
    </li>
  )
}

function EditingStatus({ summary }: { summary?: AuthoringSummary }) {
  const { t } = useTranslation('guidelines')
  const { t: authoringText } = useTranslation('authoring')
  if (!summary) return null
  const named = {
    kind: authoringText(`kinds.${summary.kind}`),
    name: summary.displayName || authoringText(`kinds.${summary.kind}`),
  }
  return (
    <div role="status" className="mt-2 space-y-2">
      {summary.lastPublication?.outcome && (
        <Typography variant="body">
          {t('authoringState.lastPublication', {
            result: authoringText(
              summary.lastPublication.outcome === 'created' ? 'confirmedCreate' : 'confirmedUpdate',
              { kind: named.kind, name: summary.lastPublication.name },
            ),
          })}
        </Typography>
      )}
      {summary.hasUnpublishedChanges && (
        <Typography variant="body">{authoringText('unpublished', named)}</Typography>
      )}
      {summary.activeJobId && (
        <Typography variant="body">{t('authoringState.active', named)}</Typography>
      )}
      {summary.publicationPending && (
        <Typography variant="body">{t('authoringState.pending', named)}</Typography>
      )}
      {summary.targetConflict && (
        <Typography variant="body">{t('authoringState.conflict', named)}</Typography>
      )}
    </div>
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
  const [reviewing, setReviewing] = useState(false)

  // Nothing waiting, room to record more and nothing just done is the ordinary state, and it
  // needs no words.
  if (isError || (!waiting && !outcome && !reviewing)) return null

  const summary =
    kind === 'clip'
      ? t(queueFull ? 'candidate.summaryClipFull' : 'candidate.summaryClip', {
          count: candidates.length,
        })
      : t(queueFull ? 'candidate.summaryFull' : 'candidate.summary', {
          count: candidates.length,
        })
  return (
    <div className="mt-6">
      <Typography variant="meta" as="p" role="status" className="empty:hidden">
        {outcome ? bulkReport(t, outcome) : ''}
      </Typography>
      {/* Guide: the banner — the count, and ONE big way into the review. A full queue says so
          here, before anything is opened (GUIDE-22). */}
      {waiting && (
        <section
          aria-labelledby="guideline-candidates-banner"
          className={cardStyles({
            padding: 'md',
            className:
              'bg-badge-accent-bg animate-step-in mt-2 flex flex-col gap-4 sm:flex-row sm:items-center',
          })}
        >
          <span className="flex min-w-0 flex-1 items-center gap-3">
            <span
              aria-hidden="true"
              className="bg-button-cta-bg text-button-cta-fg flex size-12 shrink-0 items-center justify-center rounded-lg"
            >
              <Sparkles className="size-6" />
            </span>
            <Typography variant="title" as="h2" id="guideline-candidates-banner">
              {summary}
            </Typography>
          </span>
          <Button
            variant="cta"
            size="hero"
            className="w-full sm:w-auto"
            onClick={() => setReviewing(true)}
          >
            {t('candidate.reviewStart')}
          </Button>
        </section>
      )}
      <CandidateReview
        open={reviewing}
        onClose={() => setReviewing(false)}
        ownerId={ownerId}
        kind={kind}
        candidates={candidates}
        queueFull={queueFull}
        failures={live}
        onFailures={setFailures}
        onFinished={setOutcome}
      />
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

/** Guide: the review — ONE candidate per screen, large, with a progress count and the two big
 *  answers in the thumb zone: 저장 (which asks for the scope, as approval always has) and 넘기기
 *  (look at the next one; this one stays waiting). 무시 and the bulk answers stay one row below,
 *  quieter. Saving or ignoring takes the candidate out of the queue, so the same position then
 *  shows the next one; skipping past the last wraps to the first. */
function CandidateReview({
  open,
  onClose,
  ownerId,
  kind,
  candidates,
  queueFull,
  failures,
  onFailures,
  onFinished,
}: {
  open: boolean
  onClose: () => void
  ownerId: string
  kind: GuidelineKind
  candidates: readonly GuidelineCandidate[]
  queueFull: boolean
  failures: BulkReviewOutcome['failures']
  onFailures: (failures: BulkReviewOutcome['failures']) => void
  onFinished: (outcome: BulkOutcome) => void
}) {
  const { t } = useTranslation('guidelines')
  const headingId = useId()
  const [position, setPosition] = useState(0)
  const total = candidates.length
  const index = total === 0 ? 0 : Math.min(position, total - 1)
  const candidate = candidates[index]
  return (
    <Sheet
      open={open}
      labelledBy={headingId}
      onClose={onClose}
      size="wide"
      header={
        <div className="flex flex-col gap-3 px-5 pt-5 sm:px-6">
          <span className="flex items-baseline justify-between gap-3">
            <Typography variant="title" as="h2" id={headingId}>
              {t(kind === 'clip' ? 'candidate.reviewTitleClip' : 'candidate.reviewTitle')}
            </Typography>
            {total > 0 && (
              <span className={typographyStyles({ variant: 'label' })}>
                {t('candidate.reviewProgress', { current: index + 1, total })}
              </span>
            )}
          </span>
          {total > 0 && (
            <span aria-hidden="true" className="flex gap-1">
              {candidates.map((row, at) => (
                <span
                  key={row.id}
                  className={clsx(
                    'h-1.5 flex-1 rounded-full',
                    at <= index ? 'bg-progress-step-done' : 'bg-progress-step-todo',
                  )}
                />
              ))}
            </span>
          )}
        </div>
      }
      bodyClassName="px-5 pb-4 sm:px-6"
      footer={
        <div className="flex flex-col gap-2 px-5 pt-2 pb-5 sm:px-6">
          {candidate ? (
            <div className="flex gap-2">
              <Button
                variant="secondary"
                size="hero"
                className="flex-1"
                disabled={total < 2}
                onClick={() => setPosition((index + 1) % total)}
              >
                {t('candidate.skip')}
              </Button>
              <ApproveGuidelineCandidateButton
                key={candidate.id}
                ownerId={ownerId}
                candidate={candidate}
                size="hero"
                className="flex-1"
              />
            </div>
          ) : (
            <Button variant="cta" size="hero" className="w-full" onClick={onClose}>
              {t('candidate.close')}
            </Button>
          )}
        </div>
      }
    >
      {/* Named by what it is, not by a count, so the region a screen reader lands in keeps its
          name as the queue empties. */}
      <section aria-label={t('candidate.section')} className="flex flex-col gap-4">
        {/* The one thing an empty result cannot tell the user: recording has stopped, and
            clearing a row is what starts it again. */}
        {queueFull && (
          <Notice tone="warning" role="status">
            {t('candidate.queueFull')}
          </Notice>
        )}
        {candidate ? (
          <ul key={candidate.id} className="animate-step-in">
            <CandidateRow
              ownerId={ownerId}
              candidate={candidate}
              failure={failures.find((failure) => failure.id === candidate.id)?.message}
            />
          </ul>
        ) : (
          <Typography variant="title" as="p" className="py-8 text-center">
            {t('candidate.reviewDone')}
          </Typography>
        )}
        <BulkGuidelineCandidateActions
          ownerId={ownerId}
          kind={kind}
          candidates={candidates}
          onFailures={onFailures}
          onFinished={onFinished}
        />
      </section>
    </Sheet>
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
    <li className={cardStyles({ padding: 'lg', className: 'shadow-sm' })}>
      <Typography
        variant="title"
        as="p"
        className="text-content-primary break-words whitespace-pre-wrap"
      >
        {candidate.text}
      </Typography>
      <div className="mt-3 flex flex-wrap items-center gap-2">
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
      {/* 저장 is the review's big button; 무시 stays here, quiet, on the candidate it ignores. */}
      <div className="mt-4 flex flex-wrap items-center gap-2">
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
