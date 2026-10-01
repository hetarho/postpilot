import {
  createContext,
  forwardRef,
  useContext,
  useImperativeHandle,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { SendHorizontal } from 'lucide-react'
import {
  useStartGeneration,
  useStartStoryline,
  useStartStorylineRevision,
  type GenerationJob,
} from '@/entities/generation-job'
import { useSelectionSavePending } from '@/entities/model-catalog'
import { isPublished, type PostDraft } from '@/entities/post'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'
import {
  AppFailureMessage,
  Button,
  Dialog,
  FieldMessage,
  Notice,
  Textarea,
  Typography,
} from '@/shared/ui'
import { needsPicker } from '../model/reobserve'
import {
  isSetupBlocker,
  ordinaryGenerationPreconditions,
  type GenerationMode,
} from '../model/preconditions'
import { useGenerationSelections } from '../model/useBriefIssues'
import { ReobservePicker } from './ReobservePicker'

/** Mirrored from the backend like the revision instruction's: the field stops at the bound the
 *  server enforces (GEN-69). */
const STORYLINE_REQUEST_MAX_CHARS = 500

export interface StorylineActionsHandle {
  /** 다시 만들기 — also the retry of a failed storyline job ② owns. */
  remake: () => void
  /** The retry of a failed storyline request started here, with the request it carried. */
  retryRequest: () => void
}

type StorylinePost = Pick<
  PostDraft,
  | 'slug'
  | 'status'
  | 'images'
  | 'videos'
  | 'observations'
  | 'pendingExperimentId'
  | 'voice'
  | 'storyline'
  | 'content'
  | 'contentRevision'
  | 'machineBaselineRevision'
>

interface StorylineActionsProps {
  post: StorylinePost
  targetLength?: number
  activeJob?: GenerationJob
  jobPending?: boolean
  onStarted: (jobId: string) => void
  /** Saves the draft queue — the storyline edits included — before anything starts. */
  beforeStart: () => Promise<void>
  checkRequiredAnswers?: () => boolean
  /** Saves the block editor's content before a rewrite replaces it. */
  flushContent: () => Promise<unknown>
  onOpenBrief: (mode: GenerationMode) => void
  children: ReactNode
}

interface StorylineActionsValue {
  blocker: string
  disabled: boolean
  hasContent: boolean
  remakePending: boolean
  writePending: boolean
  sendPending: boolean
  request: string
  setRequest: (request: string) => void
  remake: () => void
  write: () => void
  send: () => void
  errorMessage: string
  prepareFailure: AppFailure | undefined
}

const StorylineActionsContext = createContext<StorylineActionsValue | undefined>(undefined)

function useStorylineActions(): StorylineActionsValue {
  const value = useContext(StorylineActionsContext)
  if (!value) throw new Error('storyline actions are used outside their provider')
  return value
}

/** The storyline space's actions (POST-97, POST-98): 다시 만들기, 이 스토리로 글 쓰기 / 다시 쓰기 and
 *  the AI request. They share ①'s 바로 글 쓰기 checks — the models, the voice, the published lock and
 *  a running job — and its re-observation picker, and each saves the draft queue first, so the
 *  storyline edits on screen are what the run reads.
 *
 *  A provider, because the space places them in two spots: the buttons beside its heading and the
 *  request field under it. Both read one state. */
export const StorylineActionsProvider = forwardRef<StorylineActionsHandle, StorylineActionsProps>(
  function StorylineActionsProvider(
    {
      post,
      targetLength,
      activeJob,
      jobPending = false,
      onStarted,
      beforeStart,
      checkRequiredAnswers,
      flushContent,
      onOpenBrief,
      children,
    },
    ref,
  ) {
    const { t } = useTranslation('posts')
    const selections = useGenerationSelections()
    const selectionSaving = useSelectionSavePending()
    const generation = useStartGeneration()
    const storyline = useStartStoryline()
    const revision = useStartStorylineRevision()
    const [preparing, setPreparing] = useState<'remake' | 'write' | 'send' | ''>('')
    const [prepareFailure, setPrepareFailure] = useState<AppFailure>()
    const [picking, setPicking] = useState(false)
    const [confirming, setConfirming] = useState<'remake' | 'write' | ''>('')
    const [request, setRequest] = useState('')
    // The request a started job carried, kept for its retry: the field is cleared once it starts.
    const sentRequest = useRef('')

    const { observe: observeSelection, write: writeSelection } = selections
    const ordinary = ordinaryGenerationPreconditions({
      images: post.images,
      videos: post.videos,
      published: isPublished(post),
      activeJob,
      voice: post.voice,
      observe: observeSelection,
      write: writeSelection,
    })
    const setupRefused = !ordinary.ok && isSetupBlocker(ordinary.blocker)
    const busy =
      jobPending ||
      Boolean(preparing) ||
      generation.isPending ||
      storyline.isPending ||
      revision.isPending
    const sharedDisabled =
      selections.isPending || selectionSaving || busy || Boolean(post.pendingExperimentId)
    const disabled = sharedDisabled || (!ordinary.ok && !setupRefused)
    const hasContent = Boolean(post.content)
    const observeRef = post.images.length || post.videos.length ? observeSelection?.ref : undefined

    /** Whether a press may go ahead; a setup refusal takes it to the brief instead. */
    const ready = () => {
      if (sharedDisabled) return false
      if (setupRefused) {
        onOpenBrief('generation')
        return false
      }
      if (!ordinary.ok || !writeSelection) return false
      return checkRequiredAnswers?.() !== false
    }

    const run = async (
      kind: 'remake' | 'write' | 'send',
      start: () => Promise<{ jobId: string }>,
    ) => {
      if (checkRequiredAnswers?.() === false) return false
      setPreparing(kind)
      setPrepareFailure(undefined)
      try {
        await beforeStart()
        if (kind === 'write') await flushContent()
      } catch (cause) {
        setPrepareFailure(appFailureFromConnect(cause))
        setPreparing('')
        return false
      }
      try {
        const response = await start()
        onStarted(response.jobId)
        return true
      } catch {
        // The mutation renders its own refusal under the field.
        return false
      } finally {
        setPreparing('')
      }
    }

    const enqueueRemake = (files?: readonly string[]) =>
      void run('remake', () => storyline.start(post.slug, observeRef, writeSelection!.ref, files))

    const proceedRemake = () => {
      if (needsPicker(post.images, post.observations, post.videos)) setPicking(true)
      else enqueueRemake()
    }

    const remake = () => {
      if (!ready()) return
      // The owner's own edits are what a remake replaces, so it asks once (POST-97).
      if (post.storyline?.editedByHand) setConfirming('remake')
      else proceedRemake()
    }

    const proceedWrite = () =>
      void run('write', () =>
        generation.start(post.slug, observeRef, writeSelection!.ref, targetLength, undefined, true),
      )

    const write = () => {
      if (!ready()) return
      // A post edited by hand since its machine write is what a rewrite discards (POST-98).
      if (hasContent && post.contentRevision !== post.machineBaselineRevision)
        setConfirming('write')
      else proceedWrite()
    }

    const sendRequest = async (text: string) => {
      if (!ready() || !text) return
      const started = await run('send', () => revision.start(post.slug, text, writeSelection!.ref))
      if (started) {
        sentRequest.current = text
        setRequest('')
      }
    }

    useImperativeHandle(
      ref,
      () => ({
        remake,
        retryRequest: () => void sendRequest(sentRequest.current),
      }),
      // Rebuilt every render on purpose: both read the post and the models as they stand.
    )

    const value: StorylineActionsValue = {
      blocker: ordinary.ok ? '' : ordinary.reason,
      disabled,
      hasContent,
      remakePending: preparing === 'remake' || storyline.isPending,
      writePending: preparing === 'write' || generation.isPending,
      sendPending: preparing === 'send' || revision.isPending,
      request,
      setRequest,
      remake,
      write,
      send: () => void sendRequest(request.trim()),
      errorMessage: storyline.errorMessage || generation.errorMessage || revision.errorMessage,
      prepareFailure,
    }

    return (
      <StorylineActionsContext.Provider value={value}>
        {children}
        <ReobservePicker
          open={picking}
          images={post.images}
          videos={post.videos}
          observations={post.observations}
          observeModel={observeSelection?.ref}
          pending={Boolean(preparing)}
          onConfirm={(files) => {
            setPicking(false)
            enqueueRemake(files)
          }}
          onCancel={() => setPicking(false)}
        />
        <Dialog
          open={confirming === 'remake'}
          title={t('storylineActions.remakeTitle')}
          confirmLabel={t('storylineActions.remake')}
          onClose={() => setConfirming('')}
          onConfirm={() => {
            setConfirming('')
            proceedRemake()
          }}
        >
          {t('storylineActions.remakeBody')}
        </Dialog>
        <Dialog
          open={confirming === 'write'}
          title={t('storylineActions.rewriteTitle')}
          confirmLabel={t('storylineActions.rewriteConfirm')}
          onClose={() => setConfirming('')}
          onConfirm={() => {
            setConfirming('')
            proceedWrite()
          }}
        >
          {t('storylineActions.rewriteBody')}
        </Dialog>
      </StorylineActionsContext.Provider>
    )
  },
)

/** Why the actions cannot run, above the space's heading row. */
export function StorylineActionBlocker() {
  const { blocker } = useStorylineActions()
  if (!blocker) return null
  return (
    <Typography variant="label" as="p" role="status" className="mb-2">
      {blocker}
    </Typography>
  )
}

/** 다시 만들기 and the write action, beside the space's heading. */
export function StorylineActionButtons() {
  const { t } = useTranslation('posts')
  const actions = useStorylineActions()
  return (
    <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
      <Button
        variant="secondary"
        disabled={actions.disabled}
        pending={actions.remakePending}
        onClick={actions.remake}
      >
        {t('storylineActions.remake')}
      </Button>
      <Button
        variant="cta"
        disabled={actions.disabled}
        pending={actions.writePending}
        onClick={actions.write}
      >
        {t(actions.hasContent ? 'storylineActions.rewrite' : 'storylineActions.write')}
      </Button>
    </div>
  )
}

/** The one AI request field under the heading row, with its send button (POST-97). A storyline
 *  request is about this storyline alone, so it is no guideline candidate and offers no
 *  지침으로 저장 (GUIDE-7). */
export function StorylineRequestComposer() {
  const { t } = useTranslation('posts')
  const actions = useStorylineActions()
  const trimmed = actions.request.trim()
  return (
    <form
      className="mt-2 grid gap-2"
      onSubmit={(event) => {
        event.preventDefault()
        actions.send()
      }}
    >
      <div className="flex items-end gap-2">
        <Textarea
          value={actions.request}
          rows={1}
          autoGrow
          maxLength={STORYLINE_REQUEST_MAX_CHARS}
          aria-label={t('storylineActions.request')}
          placeholder={t('storylineActions.request')}
          aria-describedby="storyline-request-count"
          disabled={actions.disabled || actions.sendPending}
          onChange={(event) => actions.setRequest(event.target.value)}
          className="max-h-field min-w-0 flex-1"
        />
        <Button
          type="submit"
          variant="secondary"
          size="icon"
          aria-label={t('storylineActions.send')}
          disabled={actions.disabled || !trimmed}
          pending={actions.sendPending}
        >
          <SendHorizontal aria-hidden="true" className="size-5" />
        </Button>
      </div>
      <Typography variant="meta" as="p" id="storyline-request-count">
        {actions.request.length}/{STORYLINE_REQUEST_MAX_CHARS}
      </Typography>
      {actions.errorMessage && <FieldMessage>{actions.errorMessage}</FieldMessage>}
      {actions.prepareFailure && (
        <Notice tone="danger" role="alert">
          <AppFailureMessage failure={actions.prepareFailure} />
        </Notice>
      )}
    </form>
  )
}
