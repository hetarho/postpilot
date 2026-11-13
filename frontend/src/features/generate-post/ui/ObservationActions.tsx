import { useId, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useStartObservation, type GenerationJob } from '@/entities/generation-job'
import { useSelectionSavePending } from '@/entities/model-catalog'
import { isPublished, type PostDraft } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  FieldMessage,
  Notice,
  StepBarActions,
  StepBarCta,
  Typography,
} from '@/shared/ui'
import {
  isSetupBlocker,
  observationPreconditions,
  type GenerationMode,
} from '../model/preconditions'
import { lacksObservation } from '../model/reobserve'
import { useGenerationSelections } from '../model/useBriefIssues'
import { ReobservePicker } from './ReobservePicker'

type ObservationPost = Pick<PostDraft, 'slug' | 'status' | 'images' | 'videos' | 'observations'>

interface ObservationStartArgs {
  post: ObservationPost
  activeJob?: GenerationJob
  jobPending?: boolean
  /** Flushes the draft before the job reads it. */
  beforeStart: () => Promise<void>
  onStarted: (jobId: string) => void
  /** Opens the writing brief, where the observe model is chosen, for a refused setup. */
  onOpenBrief: (mode: GenerationMode) => void
}

/** One observation start, shared by 관찰하기 and 다시 관찰하기 (GEN-89). */
function useObservationStart({
  post,
  activeJob,
  jobPending = false,
  beforeStart,
  onStarted,
  onOpenBrief,
}: ObservationStartArgs) {
  const selections = useGenerationSelections()
  const selectionSaving = useSelectionSavePending()
  const observation = useStartObservation()
  const [preparing, setPreparing] = useState(false)
  const [prepareFailure, setPrepareFailure] = useState<AppFailure>()
  const reasonId = useId()
  const gate = observationPreconditions({
    images: post.images,
    videos: post.videos,
    published: isPublished(post),
    activeJob,
    observe: selections.observe,
  })
  const setupRefused = !gate.ok && isSetupBlocker(gate.blocker)
  const busy =
    jobPending || preparing || observation.isPending || selections.isPending || selectionSaving
  const disabled = busy || (!gate.ok && !setupRefused)
  /** Whether a press may go ahead; a setup refusal takes it to the brief instead. */
  const ready = () => {
    if (busy) return false
    if (setupRefused) {
      onOpenBrief('generation')
      return false
    }
    return gate.ok && Boolean(selections.observe)
  }
  const start = async (files: readonly string[]) => {
    if (!ready() || !selections.observe) return
    setPreparing(true)
    setPrepareFailure(undefined)
    try {
      await beforeStart()
    } catch (cause) {
      setPrepareFailure(appFailureFromConnect(cause))
      setPreparing(false)
      return
    }
    try {
      const response = await observation.start(post.slug, selections.observe.ref, files)
      onStarted(response.jobId)
    } catch {
      // The mutation renders its refusal above the row.
    } finally {
      setPreparing(false)
    }
  }
  const failure = (
    <>
      {setupRefused && (
        <Typography variant="label" as="p" id={reasonId} className="min-w-0">
          {gate.reason}
        </Typography>
      )}
      {observation.isError && <FieldMessage>{observation.errorMessage}</FieldMessage>}
      {prepareFailure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={prepareFailure} />
        </Notice>
      )}
    </>
  )
  return {
    gate,
    reasonId: setupRefused ? reasonId : undefined,
    disabled,
    pending: preparing || observation.isPending,
    observeModel: selections.observe?.ref,
    ready,
    start,
    failure,
  }
}

/** ① 확인's big action (POST-123): `관찰하기` while an attached photo or video has no observation,
 *  starting the observation job with an EMPTY selection — observe only what lacks one — and
 *  carrying the owner to ②; otherwise `다음`, moving to ② without a call. A published post only
 *  moves (POST-86). An observe-model refusal stands above the row. */
export function ObserveActions({
  back,
  onNext,
  observeLabel,
  nextLabel,
  ...args
}: ObservationStartArgs & {
  back: ReactNode
  /** Moves to ② — after a started observation, or in place of one. */
  onNext: () => void
  observeLabel: string
  nextLabel: string
}) {
  const observation = useObservationStart({
    ...args,
    onStarted: (jobId) => {
      args.onStarted(jobId)
      onNext()
    },
  })
  const observe =
    !isPublished(args.post) &&
    lacksObservation(args.post.images, args.post.observations, args.post.videos)
  return (
    <>
      {observe && observation.failure}
      <StepBarActions back={back}>
        {observe ? (
          <StepBarCta
            forward
            disabled={observation.disabled}
            pending={observation.pending}
            aria-describedby={observation.reasonId}
            onClick={() => void observation.start(NO_REOBSERVE)}
          >
            {observeLabel}
          </StepBarCta>
        ) : (
          <StepBarCta forward onClick={onNext}>
            {nextLabel}
          </StepBarCta>
        )}
      </StepBarActions>
    </>
  )
}

/** ②'s heading-row action (POST-125): `다시 관찰하기` opens the re-observation picker, and its
 *  answer starts the observation job with that selection (GEN-8). */
export function ReobserveButton(args: ObservationStartArgs) {
  const { t } = useTranslation('posts')
  const observation = useObservationStart(args)
  const [picking, setPicking] = useState(false)
  const { post } = args
  if (post.images.length + post.videos.length === 0 || isPublished(post)) return null
  return (
    <div className="flex flex-col items-end gap-1">
      <Button
        variant="secondary"
        disabled={observation.disabled}
        pending={observation.pending}
        onClick={() => {
          if (observation.ready()) setPicking(true)
        }}
      >
        {t('generation.reobserve.open')}
      </Button>
      {observation.failure}
      <ReobservePicker
        open={picking}
        images={post.images}
        videos={post.videos}
        observations={post.observations}
        observeModel={observation.observeModel}
        pending={observation.pending}
        onConfirm={(files) => {
          setPicking(false)
          void observation.start(files)
        }}
        onCancel={() => setPicking(false)}
      />
    </div>
  )
}

/** Observe only what lacks an observation (GEN-89). */
const NO_REOBSERVE: readonly string[] = []
