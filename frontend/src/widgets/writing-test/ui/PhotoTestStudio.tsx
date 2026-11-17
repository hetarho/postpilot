import { useCallback, useEffect, useId, useMemo, useState, type ReactNode } from 'react'
import { useModels, filterForStage, modelChoiceIssue, refKey } from '@/entities/model-catalog'
import type { PostImage } from '@/entities/image'
import {
  useTestPhotoHandshake,
  useWritingTestClient,
  type TestCount,
  type WritingTestPlan,
} from '@/entities/writing-test'
import {
  ModelContestantList,
  useWritingTestFlow,
  useWritingTestTranslation,
} from '@/features/writing-test'
import { PhotoAttachments, useKeyedPhotoUploads } from '@/features/upload-photos'
import {
  AppFailureMessage,
  Button,
  ContextualReturn,
  FieldMessage,
  Listbox,
  Notice,
  ReturnIcon,
  StepBar,
  StepBarActions,
  StepBarCta,
  StepBarReason,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { WritingFieldLabel } from './WritingTestTemplateFacts'

const COUNTS: readonly TestCount[] = [2, 4, 8, 16]
const CONTESTANTS_ID = 'photo-test-contestants'
const PHOTOS_ID = 'photo-test-photos'
const photosKey = (ownerId: string, seedKey: string) =>
  `postpilot:photo-test-photos:${JSON.stringify([ownerId, seedKey])}`

/** The confirmed photos' tiles survive a reload like the draft's ids do (MODEL-91); a browser that
 *  keeps nothing still has the ids, and the strip then shows only what this page confirmed. */
function restorePhotos(key: string): PostImage[] {
  try {
    const raw = globalThis.sessionStorage?.getItem(key)
    const value: unknown = raw ? JSON.parse(raw) : []
    return Array.isArray(value)
      ? value.filter(
          (photo): photo is PostImage =>
            !!photo &&
            typeof photo === 'object' &&
            typeof (photo as PostImage).id === 'string' &&
            typeof (photo as PostImage).filename === 'string',
        )
      : []
  } catch {
    return []
  }
}
function storePhotos(key: string, photos: readonly PostImage[]): void {
  try {
    if (photos.length) globalThis.sessionStorage?.setItem(key, JSON.stringify(photos))
    else globalThis.sessionStorage?.removeItem(key)
  } catch {
    /* The draft's ids still hold the test's photos. */
  }
}

export interface PhotoTestStudioProps {
  /** The page's heading, set beside ← on the studio's top row. */
  heading?: ReactNode
  ownerId: string
  seedKey: string
  initialPlan: WritingTestPlan
  /** Called once the test exists, so the page can open it at /tests/$testId. */
  onStarted: (testId: string) => void
}

/** The photo observation test's setup (MODEL-97): observe-stage models, then the test's own
 *  photos, then the cost. It drives the writing test's actor with an observation plan, so
 *  estimate, start, reload recovery and leaving behave as a writing test's do (MODEL-91). */
export function PhotoTestStudio(props: PhotoTestStudioProps) {
  return <OwnedPhotoStudio key={JSON.stringify([props.ownerId, props.seedKey])} {...props} />
}

function OwnedPhotoStudio({
  heading,
  ownerId,
  seedKey,
  initialPlan,
  onStarted,
}: PhotoTestStudioProps) {
  const { t } = useWritingTestTranslation()
  const models = useModels()
  const client = useWritingTestClient()
  const handshake = useTestPhotoHandshake()
  const flow = useWritingTestFlow({ ownerId, seedKey, initialDraft: initialPlan, client })
  const plan = flow.context.draft ?? initialPlan
  const draftPhotoIds = plan.context.photoIds
  const photoIds = useMemo(() => draftPhotoIds ?? [], [draftPhotoIds])
  const headingId = useId()
  const [attempted, setAttempted] = useState(false)
  const [focusRequest, setFocusRequest] = useState<{ id: string } | null>(null)
  const storageKey = photosKey(ownerId, seedKey)
  const [confirmed, setConfirmed] = useState<PostImage[]>(() => restorePhotos(storageKey))
  // The strip shows the draft's photos in the draft's order: the ids are the truth, the tiles
  // follow them.
  const photos = useMemo(
    () =>
      photoIds.flatMap((id) => {
        const photo = confirmed.find((each) => each.id === id)
        return photo ? [photo] : []
      }),
    [photoIds, confirmed],
  )
  useEffect(() => storePhotos(storageKey, photos), [storageKey, photos])
  const { send, sendPresentation, actorRef } = flow
  const draft = useCallback((next: WritingTestPlan) => send({ type: 'DRAFT', draft: next }), [send])
  const setPhotoIds = useCallback(
    (ids: string[]) => {
      const current = actorRef.getSnapshot().context.draft ?? initialPlan
      draft({ ...current, context: { ...current.context, photoIds: ids } })
    },
    [actorRef, initialPlan, draft],
  )

  // There is no factor to choose: the setup opens on its candidates.
  useEffect(() => {
    if (flow.view === 'factor') sendPresentation({ type: 'NEXT' })
  }, [flow.view, sendPresentation])
  const testId = flow.context.test?.id
  useEffect(() => {
    if (testId) onStarted(testId)
  }, [testId, onStarted])
  useEffect(() => {
    if (!focusRequest) return
    const frame = window.requestAnimationFrame(() => {
      const field = document.getElementById(focusRequest.id)
      field?.focus({ preventScroll: true })
      field?.scrollIntoView?.({ block: 'center' })
    })
    return () => window.cancelAnimationFrame(frame)
  }, [focusRequest])

  // A confirm lands after the plan may have moved on, so it appends to the newest draft.
  const onConfirmed = useCallback(
    (image: PostImage) => {
      const current = actorRef.getSnapshot().context.draft ?? initialPlan
      const ids = current.context.photoIds ?? []
      if (ids.includes(image.id)) return
      setConfirmed((previous) => [...previous, image])
      draft({ ...current, context: { ...current.context, photoIds: [...ids, image.id] } })
    },
    [actorRef, initialPlan, draft],
  )
  const uploads = useKeyedPhotoUploads({
    batchKey: `test-photos:${seedKey}`,
    handshake,
    taken: photos.map((photo) => photo.filename),
    held: photoIds.length,
    onConfirmed,
  })
  // Before the test starts a photo is only named by the draft: taking it out of the draft is the
  // whole delete, and the test photo sweep reclaims the unattached upload (→POST-40).
  const removePhoto = (image: PostImage) => setPhotoIds(photoIds.filter((id) => id !== image.id))
  const movePhoto = (from: number, to: number) => {
    const ids = photos.map((photo) => photo.id)
    const [moved] = ids.splice(from, 1)
    if (!moved) return
    ids.splice(to, 0, moved)
    setPhotoIds(ids)
  }
  const uploading = uploads.items.some(
    (item) => item.status !== 'skipped' && item.status !== 'failed',
  )

  const issue = (model: Parameters<typeof modelChoiceIssue>[0]) =>
    modelChoiceIssue({ ...model, affordable: true }, 'observe')
  const selected = plan.entrants.flatMap((entrant) =>
    entrant.type === 'model' && entrant.model.modelId ? [entrant.model] : [],
  )
  const candidateProblem =
    selected.length !== plan.count ||
    new Set(selected.map(refKey)).size !== selected.length ||
    selected.some(
      (ref) =>
        !filterForStage(models.models, 'observe').some(
          (model) => refKey(model.ref) === refKey(ref) && !issue(model),
        ),
    )
  const candidatesReason =
    attempted && candidateProblem ? t('requiredChoices', { count: plan.count }) : ''
  const photosMessage = attempted && !photoIds.length ? t('photoTest.photosRequired') : ''
  const photosReason = !attempted
    ? ''
    : uploading
      ? t('photoTest.uploading')
      : photosMessage
        ? t('fixOne', { first: t('photoTest.photos') })
        : ''

  const next = () => {
    setAttempted(true)
    if (candidateProblem) {
      setFocusRequest({ id: CONTESTANTS_ID })
      return
    }
    setAttempted(false)
    sendPresentation({ type: 'NEXT' })
  }
  const estimate = () => {
    setAttempted(true)
    if (uploading) return
    if (!photoIds.length) {
      setFocusRequest({ id: PHOTOS_ID })
      return
    }
    send({ type: 'ESTIMATE' })
  }
  const back = (
    <Button
      variant="secondary"
      size="icon"
      aria-label={t('back')}
      title={t('back')}
      onClick={() => {
        setAttempted(false)
        sendPresentation({ type: 'BACK' })
      }}
    >
      <ReturnIcon />
    </Button>
  )
  const title =
    flow.view === 'material'
      ? t('photoTest.photosTitle')
      : flow.view === 'estimate'
        ? t('photoTest.estimateTitle', { count: plan.count })
        : flow.view === 'factor' || flow.view === 'candidates'
          ? t('photoTest.candidatesTitle')
          : t('photoTestTitle')

  return (
    <div className="flex min-w-0 flex-1 flex-col">
      <div className="flex min-h-11 flex-wrap items-center gap-2">
        <ContextualReturn />
        {heading && <div className="min-w-0 flex-1">{heading}</div>}
      </div>
      <Typography
        variant="stepTitle"
        id={headingId}
        as="h2"
        tabIndex={-1}
        className="mt-3 break-words sm:mt-6"
      >
        {title}
      </Typography>
      {flow.context.failure && flow.view !== 'material' && (
        <Notice tone="danger" role="alert" className="mt-4">
          <AppFailureMessage failure={flow.context.failure} />
        </Notice>
      )}
      {(flow.view === 'factor' || flow.view === 'candidates') && (
        <section className="mt-5 flex flex-1 flex-col gap-6">
          <Typography variant="body" className="max-w-measure">
            {t('photoTest.candidatesHelp')}
          </Typography>
          <div className="space-y-2">
            <WritingFieldLabel id={`${headingId}-format-label`} htmlFor={`${headingId}-format`}>
              {t('format')}
            </WritingFieldLabel>
            <Listbox
              id={`${headingId}-format`}
              aria-labelledby={`${headingId}-format-label`}
              value={String(plan.count)}
              options={COUNTS.map((count) => ({
                value: String(count),
                label: t(`formatLabel.${count}`),
              }))}
              onChange={(value) =>
                draft({
                  ...plan,
                  count: Number(value) as TestCount,
                  entrants: plan.entrants.slice(0, Number(value)),
                })
              }
            />
          </div>
          {models.isError && (
            <Notice tone="danger">
              <Button variant="ghost" onClick={() => void models.refetch()}>
                {t('refresh')}
              </Button>
            </Notice>
          )}
          <ModelContestantList
            id={CONTESTANTS_ID}
            stage="observe"
            count={plan.count}
            models={models.models}
            selected={selected}
            issue={issue}
            onChange={(refs) =>
              draft({ ...plan, entrants: refs.map((model) => ({ type: 'model', model })) })
            }
          />
          {!models.isPending && !filterForStage(models.models, 'observe').length && (
            <Typography variant="body">
              {t('noModels')}{' '}
              <a href="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
                {t('settings')}
              </a>
            </Typography>
          )}
          <StepBar ariaLabel={title}>
            <StepBarProblem id={`${headingId}-candidates-reason`} text={candidatesReason} />
            <StepBarActions>
              <StepBarCta
                forward
                aria-describedby={candidatesReason ? `${headingId}-candidates-reason` : undefined}
                onClick={next}
              >
                {t('next')}
              </StepBarCta>
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'material' && (
        <section className="mt-5 flex flex-1 flex-col gap-6">
          <Typography variant="body" className="max-w-measure">
            {t('photoTest.photosHelp')}
          </Typography>
          <div
            id={PHOTOS_ID}
            role="group"
            tabIndex={-1}
            aria-labelledby={`${PHOTOS_ID}-label`}
            aria-invalid={photosMessage ? true : undefined}
            aria-describedby={photosMessage ? `${PHOTOS_ID}-message` : undefined}
            className="space-y-3"
          >
            <WritingFieldLabel id={`${PHOTOS_ID}-label`} htmlFor={PHOTOS_ID} required>
              {t('photoTest.photos')}
            </WritingFieldLabel>
            <PhotoAttachments
              photosOnly
              images={photos}
              upload={uploads}
              onDelete={removePhoto}
              onMove={movePhoto}
            />
            {photosMessage && (
              <FieldMessage id={`${PHOTOS_ID}-message`}>{photosMessage}</FieldMessage>
            )}
          </div>
          <StepBar ariaLabel={title}>
            {flow.context.failure && (
              <Notice tone="danger" role="alert">
                <AppFailureMessage failure={flow.context.failure} />
              </Notice>
            )}
            <StepBarProblem id={`${headingId}-photos-reason`} text={photosReason} />
            <StepBarActions>
              {back}
              <Button
                variant="cta"
                size="hero"
                className="min-w-0 flex-1"
                pending={flow.phase === 'quoting'}
                aria-describedby={photosReason ? `${headingId}-photos-reason` : undefined}
                onClick={estimate}
              >
                {t('photoTest.estimate')}
              </Button>
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {flow.view === 'estimate' && (
        <section className="mt-5 flex flex-1 flex-col gap-5">
          <Typography variant="body">{t('photoTest.onceOnly')}</Typography>
          <Typography variant="label" as="p">
            {t('photoTest.observeCost', { count: plan.count })}
          </Typography>
          <Typography variant="title" as="p">
            {flow.context.quote?.free
              ? t('free')
              : t('credits', { count: flow.context.quote?.credits ?? 0 })}
          </Typography>
          <StepBar ariaLabel={title}>
            <StepBarActions>
              {back}
              <Button
                variant="cta"
                size="hero"
                className="min-w-0 flex-1"
                pending={flow.phase === 'starting'}
                onClick={() => send({ type: 'CONFIRM' })}
              >
                {t('photoTest.start', { count: plan.count })}
              </Button>
            </StepBarActions>
          </StepBar>
        </section>
      )}
      {!['factor', 'candidates', 'material', 'estimate'].includes(flow.view) && (
        <Typography variant="body" role="status" className="mt-5">
          {t('loading')}
        </Typography>
      )}
    </div>
  )
}

function StepBarProblem({ id, text }: { id: string; text: string }) {
  return (
    <div aria-live="polite" className="empty:hidden">
      {text ? <StepBarReason id={id}>{text}</StepBarReason> : null}
    </div>
  )
}
