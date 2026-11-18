import { ReturnIcon } from '@/shared/ui'
import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useMachine } from '@xstate/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { MessagesSquare, SendHorizontal, Sparkles } from 'lucide-react'
import {
  authoringReferenceKey,
  authoringScopeKey,
  completedAuthoringExchanges,
  AUTHORING_MAX_EXCHANGES,
  AUTHORING_NAME_SUMMARY_MAX_CHARS,
  type AuthoringScope,
  type AuthoringSavedRef,
  type AuthoringArtifact,
  type AuthoringCandidateCount,
  type AuthoringKind,
  type AuthoringMode,
  type AuthoringModelRef,
  type AuthoringReference,
} from '@/entities/ai-authoring'
import {
  sameRef,
  useInitializeDefaultSelections,
  useStageSelection,
} from '@/entities/model-catalog'
import {
  ActionMenu,
  AppFailureMessage,
  Chat,
  ChatComposer,
  TextField,
  Button,
  Checkbox,
  ChoiceButton,
  Disclosure,
  SegmentedControl,
  Sheet,
  Dialog,
  FieldMessage,
  Notice,
  Textarea,
  Typography,
  buttonStyles,
  type ChatMessage,
} from '@/shared/ui'
import { readableAuthoringProse } from '../lib/readable-prose'
import { useAuthoring } from '../model/useAuthoring'
import {
  authoringArtifactHasContent,
  studioFlowMachine,
  studioFlowView,
  type StudioAIAvailability,
} from '../model/studio-flow-machine'
import { AuthoringPreview } from './AuthoringPreview'
import { AuthoringReferences } from './AuthoringReferences'
import { AuthoringRequestFailure } from './AuthoringRequestFailure'

export interface AuthoringDirectEditorProps {
  source: AuthoringArtifact
  onChange: (source: AuthoringArtifact) => void
  disabled: boolean
}
export interface AuthoringEditorProps extends AuthoringScope {
  sessionId?: string
  initialMethod?: 'ai' | 'direct'
  startFromSaved?: boolean
  referencePost?: string
  renderDirectEditor?: (props: AuthoringDirectEditorProps) => ReactNode
  targetName?: string
  candidateCount?: AuthoringCandidateCount
  onSaved?: (ref: AuthoringSavedRef) => void
  onBusyChange?: (busy: boolean) => void
  renderPreview?: (artifact: AuthoringArtifact, lastValid?: AuthoringArtifact) => ReactNode
  renderInspection?: (context: AuthoringInspectionContext) => ReactNode
  className?: string
  initialMakeDefault?: boolean
  active?: boolean
  onNavigationChange?: (navigation: StudioNavigation | undefined) => void
  /** `conversation` lays the same flow out as a chat beside its live draft: the owner says what
   *  they want, the AI drafts it, every later message refines it, and 저장 publishes. The steps
   *  a chat has no words for — choosing the one recommended draft, reviewing it, opening
   *  refinement — are taken for the owner. Paid work still waits behind the same estimate. */
  layout?: 'steps' | 'conversation'
  /** The picture the conversation's empty draft pane shows before anything is drafted. */
  draftIllustration?: string
  /** The app's own model choice for the write stage (the one 홈's ▾ and 지금 쓰는 AI use), for
   *  a failed request's `다른 모델로 다시 시도`. It calls `onSaved` once the choice is saved;
   *  without it, a failure offers only `다시 시도`. */
  renderModelChoice?: (onSaved: () => void) => ReactNode
}
export interface AuthoringInspectionContext {
  ownerId: string
  sessionId: string
  kind: AuthoringKind
  revision: number
  mode: AuthoringMode
  prompt: string
  model?: AuthoringModelRef
  candidateCount: AuthoringCandidateCount
  operationId: string
  sourceDirty: boolean
}
export interface StudioNavigation {
  canGoBack: boolean
  goBack: () => void
}
export function AuthoringEditor(props: AuthoringEditorProps) {
  return (
    <ScopedEditor
      key={JSON.stringify([authoringScopeKey(props), props.sessionId ?? ''])}
      {...props}
    />
  )
}
function ScopedEditor({
  ownerId,
  kind,
  targetId,
  sessionId,
  initialMethod,
  startFromSaved,
  referencePost,
  renderDirectEditor,
  targetName,
  candidateCount = 8,
  onSaved,
  onBusyChange,
  renderPreview,
  renderInspection,
  className,
  initialMakeDefault = false,
  active = true,
  onNavigationChange,
  layout = 'steps',
  draftIllustration,
  renderModelChoice,
}: AuthoringEditorProps) {
  const { t } = useTranslation('authoring')
  const conversation = layout === 'conversation'
  const scope = { ownerId, kind, targetId }
  const scopeKey = authoringScopeKey(scope)
  // Navigation never mounts a second operation consumer, poller or save queue.
  const controller = useAuthoring(
    scope,
    { onSaved, onBusyChange },
    sessionId,
    referencePost,
    startFromSaved,
  )
  const { state, busy } = controller
  const write = useStageSelection('write')
  const defaults = useInitializeDefaultSelections(ownerId)
  const ai: StudioAIAvailability =
    defaults.isPending || write.isPending ? 'preparing' : write.selected ? 'ready' : 'unavailable'
  const writeLabel = write.selected
    ? write.models.find((model) => sameRef(model.ref, write.selected!))?.label
    : undefined
  // A template request can carry the owner's posts, templates or pages as its form; they ride
  // with each request until one finishes well, and are never stored (TMPL-58's reference rule).
  const referencesAllowed = kind === 'post-template'
  const [references, setReferences] = useState<AuthoringReference[]>([])
  const [seenJob, setSeenJob] = useState('')
  const activeJob = state.session?.activeJobId ?? ''
  if (activeJob && activeJob !== seenJob) setSeenJob(activeJob)
  else if (!activeJob && seenJob) {
    setSeenJob('')
    if (state.session?.phase !== 'failed') setReferences([])
  }
  const [flow, send] = useMachine(
    studioFlowMachine.provide({
      actions: {
        setChat: ({ event }) => {
          if (event.type === 'CHAT_CHANGED') controller.setText(event.text)
        },
        requestRecommend: ({ context }) => {
          controller.setText(context.purpose)
          if (write.selected)
            void controller.quote(
              'recommend',
              write.selected,
              candidateCount,
              referencesAllowed ? references : [],
            )
        },
        requestRefine: () => {
          if (write.selected)
            void controller.quote('refine', write.selected, 8, referencesAllowed ? references : [])
        },
        loadExisting: () => void controller.edit(),
        selectCandidate: ({ event }) => {
          if (event.type === 'CHOOSE') controller.select(event.candidateId)
        },
        publish: ({ context }) => controller.save(context.makeDefault),
        retry: () => void controller.retryOperation(),
        fresh: controller.fresh,
        keepDirect: () => controller.patch(),
        continueEditing: () => controller.patch(),
        resetBaseline: () => controller.resetBaseline(),
        cancel: controller.cancel,
      },
    }),
    { input: { scope, operation: state, ai, initialMakeDefault } },
  )
  useEffect(() => {
    send({ type: 'OBSERVE', scopeKey, operation: state, ai })
  }, [state, ai, scopeKey, send])
  const view = studioFlowView(flow)
  const methodStarted = useRef(false)
  useEffect(() => {
    if (
      !ownerId ||
      !initialMethod ||
      methodStarted.current ||
      !active ||
      state.phase === 'checking' ||
      busy ||
      controller.readFailure
    )
      return
    if (view === 'existing' || view === 'purpose' || view === 'review' || view === 'confirmed') {
      methodStarted.current = true
      if (view !== 'purpose' || initialMethod === 'direct')
        send({ type: initialMethod === 'direct' ? 'OPEN_DIRECT' : 'OPEN_REFINEMENT', scopeKey })
    }
  }, [
    ownerId,
    initialMethod,
    active,
    state.phase,
    busy,
    controller.readFailure,
    view,
    scopeKey,
    send,
  ])
  const [directPane, setDirectPane] = useState<'input' | 'preview'>('input')
  const [refiningPane, setRefiningPane] = useState<'input' | 'preview'>('input')
  const [paneEntry, setPaneEntry] = useState({ view, active })
  // A new editing entry opens its current input. This visual state never drives the actor.
  if (paneEntry.view !== view || paneEntry.active !== active) {
    setPaneEntry({ view, active })
    if (active && (paneEntry.view !== view || !paneEntry.active)) {
      if (view === 'direct') setDirectPane('input')
      if (view === 'refining') setRefiningPane('input')
    }
  }
  const [saveAsk, setSaveAsk] = useState(false)
  // The conversation takes the steps it has no words for. The one recommended draft is chosen, a
  // draft under review or a saved one opens straight into refinement, and a publication that
  // could not go through returns to the chat. None of these calls a model.
  useEffect(() => {
    if (!conversation || !active || busy || state.command || state.phase === 'checking') return
    if (view === 'choices') {
      const pick = state.session?.selected?.id ?? state.session?.candidates[0]?.id
      if (pick) send({ type: 'CHOOSE', scopeKey, candidateId: pick })
    } else if (view === 'review' || (view === 'existing' && !controller.readFailure))
      send({ type: 'OPEN_REFINEMENT', scopeKey })
    else if (view === 'publication' && !saveAsk) send({ type: 'BACK', scopeKey })
  }, [
    conversation,
    active,
    busy,
    state.command,
    state.phase,
    state.session?.selected?.id,
    state.session?.candidates,
    controller.readFailure,
    view,
    saveAsk,
    scopeKey,
    send,
  ])
  const guidelineKind = kind === 'post-guideline' || kind === 'video-guideline'
  const id = useId()
  const heading = useRef<HTMLDivElement>(null)
  const previousView = useRef(view)
  const previouslyActive = useRef(active)
  useEffect(() => {
    // A conversation keeps the owner's focus in its field: the steps it takes for them are not
    // new screens to announce.
    if (!conversation && active && (previousView.current !== view || !previouslyActive.current)) {
      heading.current?.querySelector('h2')?.focus()
    }
    previousView.current = view
    previouslyActive.current = active
  }, [view, active, conversation])
  const goBack = useCallback(() => send({ type: 'BACK', scopeKey }), [scopeKey, send])
  const canGoBack = flow.can({ type: 'BACK', scopeKey })
  useEffect(() => {
    onNavigationChange?.({ canGoBack, goBack })
  }, [canGoBack, goBack, onNavigationChange])
  useEffect(() => () => onNavigationChange?.(undefined), [onNavigationChange])
  const selected = state.session?.selected
  const source =
    state.directSource ??
    state.session?.workingSource ??
    selected ??
    (state.session
      ? {
          id: '',
          name: '',
          description: '',
          body: '',
          titleArea: '',
          ...(kind === 'post-template' ? { targetLength: '', tagCount: '' } : {}),
          ...(kind === 'post-guideline' || kind === 'video-guideline' ? { scope: 'global' } : {}),
        }
      : undefined)
  const summarySource = state.session?.savedBaseline ?? source ?? selected
  const summaryName =
    summarySource && readableAuthoringProse(summarySource.body)
      ? Array.from(summarySource.body.trim().replace(/\s+/g, ' '))
          .slice(0, AUTHORING_NAME_SUMMARY_MAX_CHARS)
          .join('')
      : ''
  const name =
    targetName ||
    state.session?.savedBaseline?.name ||
    source?.name ||
    selected?.name ||
    summaryName ||
    t(`kinds.${kind}`)
  const named = { kind: t(`kinds.${kind}`), name }
  const draftInvalid =
    state.session?.draftState === 'invalid' || state.session?.draftState === 'incomplete'
  // An unfinished draft is the owner's to hear about only once they have typed into it: a new
  // session starts on an empty one, which a failed request leaves exactly as it was.
  const ownerTyped =
    !!state.sourceDirty || authoringArtifactHasContent(state.session?.workingSource)
  const needsVoiceExample =
    kind === 'writing-voice' &&
    !!selected &&
    !selected.body.trim() &&
    !state.session?.hasUnpublishedChanges &&
    !state.sourceDirty
  const changeSource = (field: 'name' | 'description' | 'body' | 'titleArea', value: string) => {
    if (source) controller.setSource({ ...source, [field]: value })
  }
  const failure = state.failure
  const conflict =
    failure?.reason === 'AUTHORING_SAVE_CONFLICT' ||
    state.session?.failureReason === 'AUTHORING_SAVE_CONFLICT' ||
    failure?.reason === 'AUTHORING_REVISION_CONFLICT'
  const recoveredSave =
    state.session?.phase === 'saving' && !state.session.activeJobId && state.phase === 'active'
  const canSaveDraft = !!selected?.body.trim() && !draftInvalid && !state.sourceDirty
  const characters = Array.from(view === 'purpose' ? flow.context.purpose : state.text).length
  const blocked = busy || !!state.command
  const hasFailure = !!failure || state.session?.phase === 'failed' || conflict
  const frozenPublication = (flow.context.intent === 'save' || recoveredSave) && hasFailure
  // An AI request that did not finish — refused before its job (with its frozen command) or
  // failed as a job — is explained from its reason and offered again, on this model or another.
  // Saves, reads and conflicts keep their own retry.
  const requestFailure = conflict
    ? undefined
    : failure
      ? state.command
        ? failure
        : undefined
      : controller.requestFailure
  const aiFailure = !!requestFailure && !frozenPublication
  const otherFailure = hasFailure && !aiFailure
  const invalidNotice = draftInvalid && !needsVoiceExample && ownerTyped && !hasFailure
  const resendPending = useRef(false)
  const [modelChoiceOpen, setModelChoiceOpen] = useState(false)
  const [, setResendTick] = useState(0)
  const resend = () => {
    // The same words go again through the usual estimate. A refusal that kept its frozen
    // command is set aside first, so the new quote names the model in use now.
    if (state.command) controller.dismissQuote()
    const pending = state.session?.pendingRequest ?? ''
    if (view === 'refining') {
      if (!state.text.trim() && pending) controller.setText(pending)
    } else if (!flow.context.purpose.trim() && pending)
      send({ type: 'PURPOSE_CHANGED', scopeKey, text: pending })
    resendPending.current = true
    setResendTick((tick) => tick + 1)
  }
  // The frozen request goes again as it was — unless its references were changed since, or
  // one of them is what refused it: then the words go again with the references now attached.
  const sameReferences =
    (state.command?.references ?? []).map(authoringReferenceKey).join('\n') ===
    (referencesAllowed ? references : []).map(authoringReferenceKey).join('\n')
  const retryRequest = () =>
    state.command && sameReferences && failure?.reason !== 'AUTHORING_REFERENCE_UNAVAILABLE'
      ? event('RETRY')
      : resend()
  // Sent once the render after `resend` shows its words and, after a model change, the newly
  // saved model: the flow reads both from what it last observed.
  useEffect(() => {
    if (!resendPending.current || busy || state.command || ai !== 'ready') return
    resendPending.current = false
    if (view === 'refining') send({ type: 'REFINE', scopeKey })
    else if (view === 'purpose' || view === 'choices') send({ type: 'RECOMMEND', scopeKey })
  })
  // After a reload, a first request that failed puts its words back in the field.
  const refilled = useRef(false)
  useEffect(() => {
    const pending = state.session?.pendingRequest ?? ''
    if (refilled.current || view !== 'purpose' || state.session?.phase !== 'failed' || !pending)
      return
    refilled.current = true
    if (!flow.context.purpose.trim()) send({ type: 'PURPOSE_CHANGED', scopeKey, text: pending })
  }, [
    view,
    state.session?.phase,
    state.session?.pendingRequest,
    flow.context.purpose,
    scopeKey,
    send,
  ])
  const modelChoice = renderModelChoice && (
    <Sheet
      open={modelChoiceOpen}
      label={t('failure.otherModelTitle')}
      onClose={() => setModelChoiceOpen(false)}
      header={
        <div className="flex flex-col gap-1 px-5 pt-5 sm:px-6">
          <Typography variant="title" as="h2">
            {t('failure.otherModelTitle')}
          </Typography>
          <Typography variant="meta" as="p" className="text-pretty">
            {t('failure.otherModelHelp')}
          </Typography>
        </div>
      }
      bodyClassName="px-5 pb-5 sm:px-6"
    >
      {modelChoiceOpen && (
        <ModelChoiceSlot
          render={renderModelChoice}
          onSaved={() => {
            setModelChoiceOpen(false)
            resend()
          }}
        />
      )}
    </Sheet>
  )
  const requestFailureNotice = (className: string) =>
    requestFailure &&
    aiFailure && (
      <AuthoringRequestFailure
        failure={requestFailure}
        modelLabel={writeLabel}
        kept={
          selected || (conversation && authoringArtifactHasContent(state.session?.workingSource))
            ? 'draft'
            : (view === 'purpose' ? flow.context.purpose : state.text).trim()
              ? 'message'
              : undefined
        }
        onRetry={retryRequest}
        onOtherModel={renderModelChoice ? () => setModelChoiceOpen(true) : undefined}
        className={className}
      />
    )
  const historyFull = completedAuthoringExchanges(state.session) >= AUTHORING_MAX_EXCHANGES
  const workingStatus =
    state.phase === 'quoting'
      ? t('quoting')
      : state.phase === 'saving' || recoveredSave
        ? t('saving')
        : state.phase === 'selecting'
          ? t('selecting')
          : state.phase === 'creating'
            ? t('loadingDraft')
            : state.phase === 'cancelling'
              ? t('cancelling')
              : state.phase === 'confirming'
                ? t('quoteReady')
                : t('working')
  const event = (type: Parameters<typeof send>[0]['type']) => {
    // Payload-bearing events are sent explicitly by their inputs below.
    if (
      type !== 'OBSERVE' &&
      type !== 'PURPOSE_CHANGED' &&
      type !== 'CHAT_CHANGED' &&
      type !== 'CHOOSE' &&
      type !== 'DEFAULT_CHANGED'
    )
      send({ type, scopeKey })
  }
  const previewArtifact = view === 'direct' ? source : selected
  const preview = previewArtifact && (
    <div className="min-w-0">
      <Typography variant="fieldTitle" as="h3" className="break-words">
        {previewArtifact.name || name}
      </Typography>
      {previewArtifact.description && (
        <Typography variant="body" className="text-content-secondary mt-3 break-words">
          {previewArtifact.description}
        </Typography>
      )}
      <div className="mt-6">
        {renderPreview ? (
          renderPreview(previewArtifact, view === 'direct' ? selected : undefined)
        ) : (
          <AuthoringPreview
            kind={kind}
            artifact={previewArtifact}
            fallbackArtifact={view === 'direct' ? selected : undefined}
          />
        )}
      </div>
    </div>
  )
  const retry = otherFailure && (
    <div className="mt-8 flex flex-wrap items-center gap-4">
      <Button variant="cta" onClick={() => event('RETRY')}>
        {t(frozenPublication ? 'saveRetry' : 'retry')}
      </Button>
      {conflict && flow.can({ type: 'CONTINUE', scopeKey }) && (
        <Button variant="ghost" onClick={() => event('CONTINUE')}>
          {t('continueNamed', named)}
        </Button>
      )}
    </div>
  )
  const aiAvailability = ai !== 'ready' && (
    <div className="mt-6 space-y-3">
      <Typography variant="body" className="text-content-secondary">
        {t(ai === 'preparing' ? 'modelPreparing' : 'modelUnavailable')}
      </Typography>
      {ai === 'unavailable' && (
        <div className="flex flex-wrap gap-4">
          <Button variant="secondary" onClick={defaults.retry}>
            {t('retryAI')}
          </Button>
          <Link to="/ai-models" hash="ai-in-use" className={buttonStyles({ variant: 'ghost' })}>
            {t('settings')}
          </Link>
        </div>
      )}
    </div>
  )
  const inputFeedback = flow.context.problem && (
    <FieldMessage id={id + '-error'} className="mt-3">
      {t(`inputProblems.${flow.context.problem}`)}
    </FieldMessage>
  )
  const dialogs = (
    <>
      <Dialog
        open={state.phase === 'confirming' || state.phase === 'starting'}
        title={
          conversation
            ? t(
                state.command?.mode === 'refine'
                  ? 'conversation.confirmRevise'
                  : 'conversation.confirmDraft',
              )
            : t(state.command?.mode === 'refine' ? 'confirmRefine' : 'confirmRecommendCount', {
                count: state.command?.candidateCount ?? candidateCount,
              })
        }
        onClose={state.phase === 'starting' ? () => {} : controller.dismissQuote}
        confirmLabel={t('confirm')}
        cancelLabel={t('back')}
        pending={state.phase === 'starting'}
        onConfirm={() => void controller.confirm()}
      >
        <Typography variant="body">
          {state.estimate?.free ? t('free') : t('quote', { credits: state.estimate?.credits ?? 0 })}
        </Typography>
      </Dialog>
      <Dialog
        open={flow.context.resetOpen}
        title={t('resetNamed', named)}
        onClose={() => event('DISMISS_RESET')}
        confirmLabel={t('discard')}
        cancelLabel={t('back')}
        onConfirm={() => event('CONFIRM_RESET')}
      >
        <Typography variant="body">{t('resetWarning', named)}</Typography>
      </Dialog>
      <Dialog
        open={flow.matches({ working: 'confirmingCancellation' })}
        title={t('cancelTitle')}
        onClose={() => event('DISMISS_CANCEL')}
        confirmLabel={t('cancel')}
        cancelLabel={t('keepWorking')}
        onConfirm={() => event('CONFIRM_CANCEL')}
      >
        <Typography variant="body">{t('cancelHelp')}</Typography>
      </Dialog>
      {modelChoice}
    </>
  )
  if (conversation && view !== 'direct') {
    // A new session's working source is empty until something is drafted or typed: it is no
    // draft to show, name or save.
    const draft =
      selected ??
      (authoringArtifactHasContent(state.session?.workingSource)
        ? state.session?.workingSource
        : undefined)
    const draftName = draft?.name || state.session?.savedBaseline?.name || targetName || ''
    const working = view === 'working'
    const asking = view === 'purpose'
    const composing = asking || view === 'refining' || working
    const asked = !asking ? flow.context.purpose.trim() : ''
    const viewDraft = (
      <Button
        variant="secondary"
        size="compact"
        className="mt-3 @3xl:hidden"
        onClick={() => setRefiningPane('preview')}
      >
        {t('conversation.viewDraft')}
      </Button>
    )
    const messages: ChatMessage[] = []
    if (targetId || (state.session?.savedBaseline && !asked))
      messages.push({
        id: 'loaded',
        sender: 'partner',
        content: t('conversation.loaded', { name: draftName || name }),
      })
    else
      messages.push({
        id: 'intro',
        sender: 'partner',
        content: (
          <>
            {t(`conversation.intro.${kind}`)}
            {asking && kind === 'post-template' && (
              <span
                role="group"
                aria-label={t('conversation.examplesAria')}
                className="mt-3 flex flex-col items-start gap-2 whitespace-normal"
              >
                {(['a', 'b', 'c'] as const).map((key) => (
                  <Button
                    key={key}
                    variant="secondary"
                    size="compact"
                    className="h-auto max-w-full py-1.5 text-left"
                    disabled={blocked}
                    onClick={() =>
                      send({
                        type: 'PURPOSE_CHANGED',
                        scopeKey,
                        text: t(`conversation.examples.${key}`),
                      })
                    }
                  >
                    {t(`conversation.examples.${key}`)}
                  </Button>
                ))}
              </span>
            )}
          </>
        ),
      })
    if (asked) messages.push({ id: 'asked', sender: 'owner', content: asked })
    if (draft && !targetId && !state.session?.savedBaseline)
      messages.push({
        id: 'draft',
        sender: 'partner',
        content: (
          <>
            {t('conversation.draftReady', { name: draftName || name })}
            {!state.session?.turns.length && viewDraft}
          </>
        ),
      })
    state.session?.turns.forEach((turn, index, turns) => {
      messages.push({ id: turn.id + '-request', sender: 'owner', content: turn.request })
      if (turn.status === 'done')
        messages.push({
          id: turn.id + '-reply',
          sender: 'partner',
          content: (
            <>
              {turn.reply && readableAuthoringProse(turn.reply)
                ? turn.reply
                : t('conversation.revised')}
              {index === turns.length - 1 && viewDraft}
            </>
          ),
        })
    })
    if (working)
      messages.push({
        id: 'working',
        sender: 'partner',
        content: (
          <span className="flex flex-col items-start gap-3">
            <span className="flex items-center gap-2">
              <span aria-hidden="true" className="relative flex size-3">
                <span className="bg-promo-stroke animate-promo-glow absolute -inset-1 rounded-full blur-sm" />
                <span className="bg-button-cta-bg relative size-3 rounded-full" />
              </span>
              {state.phase === 'active' || state.phase === 'starting'
                ? t('conversation.drafting')
                : workingStatus}
            </span>
            {state.phase === 'active' && state.session?.activeJobId && (
              <Button variant="secondary" size="compact" onClick={() => event('ASK_CANCEL')}>
                {t('cancel')}
              </Button>
            )}
          </span>
        ),
      })
    const draftPane = (
      <div className="flex min-w-0 flex-col gap-3">
        {/* A post template's preview names itself (미리보기); over it, this would be a second
            heading saying the same. */}
        <Typography
          variant="eyebrow"
          as="h3"
          className={draft && kind === 'post-template' ? 'sr-only' : undefined}
        >
          {t('conversation.draft')}
        </Typography>
        {draft ? (
          <div
            key={state.session?.revision ?? 0}
            className={
              'animate-rise relative min-w-0 transition-opacity ' + (working ? 'opacity-60' : '')
            }
          >
            {working && (
              <span
                aria-hidden="true"
                className="bg-promo-stroke animate-promo-glow pointer-events-none absolute -inset-2 -z-10 rounded-3xl opacity-40 blur-2xl"
              />
            )}
            {draft.description && (
              <Typography variant="body" className="text-content-secondary mb-4 break-words">
                {draft.description}
              </Typography>
            )}
            {renderPreview ? (
              renderPreview(draft, selected === draft ? undefined : selected)
            ) : (
              <AuthoringPreview kind={kind} artifact={draft} />
            )}
          </div>
        ) : (
          // Before the first draft, the pane holds the shape of one: a blank page the conversation
          // will fill. While the AI drafts, the same page shimmers in a halo; then the draft
          // arrives in its place.
          <div className="flex flex-col items-center gap-4">
            <div className="relative w-full max-w-sm">
              {working && (
                <span
                  aria-hidden="true"
                  className="bg-promo-stroke animate-promo-glow pointer-events-none absolute inset-6 -z-10 rounded-3xl opacity-40 blur-2xl"
                />
              )}
              <div
                aria-hidden="true"
                className={
                  'bg-card-bg duration-slow flex w-full flex-col gap-4 rounded-2xl px-5 py-6 shadow-sm transition-opacity ' +
                  (working ? '' : 'opacity-60')
                }
              >
                <span
                  className={
                    'bg-surface-recessed h-5 w-3/4 rounded-md ' +
                    (working ? 'loading-skeleton' : '')
                  }
                />
                {[0, 1, 2].map((block) => (
                  <span key={block} className="flex flex-col gap-1.5">
                    <span
                      className={
                        'bg-badge-accent-bg h-3 w-1/3 rounded-full ' +
                        (working ? 'loading-skeleton' : '')
                      }
                    />
                    {['w-full', 'w-11/12', 'w-3/5'].map((width) => (
                      <span
                        key={width}
                        className={
                          'bg-surface-recessed h-2 rounded-full ' +
                          width +
                          (working ? ' loading-skeleton' : '')
                        }
                      />
                    ))}
                  </span>
                ))}
              </div>
              {!working && (
                <span className="absolute inset-0 flex items-center justify-center">
                  {draftIllustration ? (
                    <img
                      src={draftIllustration}
                      alt=""
                      aria-hidden="true"
                      className="animate-rise size-24 object-contain drop-shadow-md"
                    />
                  ) : (
                    <MessagesSquare aria-hidden="true" className="text-content-tertiary size-10" />
                  )}
                </span>
              )}
            </div>
            {/* While drafting, the chat says so; the page's shimmer is enough here. */}
            {!working && (
              <Typography variant="body" className="text-content-secondary text-center">
                {t('conversation.draftEmpty')}
              </Typography>
            )}
          </div>
        )}
      </div>
    )
    return (
      <section
        aria-labelledby={id + '-title'}
        className={'@container min-w-0 ' + (className ?? '')}
      >
        <div
          className={
            draft ? 'border-divider flex flex-wrap items-center gap-2 border-b pb-3' : 'contents'
          }
        >
          {/* A saved one is already named by the page around it; the name shows here once it is
              a new draft's, or once the conversation has renamed it. */}
          <div
            className={draft && draftName !== targetName ? 'min-w-0 flex-1 basis-40' : 'sr-only'}
          >
            <Typography variant="meta" as="p">
              {t(`kinds.${kind}`)}
            </Typography>
            <Typography variant="title" as="h2" id={id + '-title'} className="truncate">
              {draftName || t('conversation.log', { kind: t(`kinds.${kind}`) })}
            </Typography>
          </div>
          {draft && draftName === targetName && <span aria-hidden="true" className="flex-1" />}
          {draft && (
            <ActionMenu
              label={t('conversation.more')}
              triggerVariant="ghost"
              triggerLabel={t('conversation.more')}
              disabled={blocked}
              items={[
                {
                  id: 'direct',
                  label: t('conversation.direct'),
                  onSelect: () => event('OPEN_DIRECT'),
                },
                ...(state.session?.turns.length
                  ? [
                      {
                        id: 'fresh',
                        label: t('conversation.fresh'),
                        description: t('freshHelp', named),
                        onSelect: () => event('FRESH'),
                      },
                    ]
                  : []),
                ...(state.session?.savedBaseline
                  ? [
                      {
                        id: 'reset',
                        label: t('conversation.reset'),
                        onSelect: () => event('ASK_RESET'),
                      },
                    ]
                  : []),
              ]}
            />
          )}
          {draft && (
            <Button
              variant="cta"
              // A saved one opened unchanged has nothing to save until the conversation (or the
              // source) changes it.
              disabled={
                blocked ||
                !canSaveDraft ||
                (!!targetId && !!state.session && !state.session.hasUnpublishedChanges)
              }
              pending={recoveredSave || state.phase === 'saving'}
              onClick={() => setSaveAsk(true)}
            >
              {targetId ? t('conversation.save') : t('conversation.saveNew')}
            </Button>
          )}
        </div>
        {invalidNotice && (
          <Notice tone="danger" className="mt-4">
            {t(selected ? 'invalidSource' : 'invalidSourceNoPreview')}
          </Notice>
        )}
        {requestFailureNotice('mt-4')}
        {otherFailure && (
          <Notice tone="danger" role="alert" className="mt-4">
            {conflict ? (
              t('conflict')
            ) : failure ? (
              <AppFailureMessage failure={failure} />
            ) : (
              t('failed')
            )}
          </Notice>
        )}
        {otherFailure && retry}
        {controller.readFailure && (
          <Notice tone="danger" role="alert" className="mt-4">
            <AppFailureMessage failure={controller.readFailure} />
            <Button variant="ghost" className="mt-3" onClick={controller.retryRead}>
              {t('retryRead')}
            </Button>
          </Notice>
        )}
        {(draft || working) && (
          <SegmentedControl
            value={refiningPane}
            onChange={setRefiningPane}
            ariaLabel={t('conversation.panes')}
            options={[
              { value: 'input', label: t('conversation.chat') },
              { value: 'preview', label: t('conversation.draft') },
            ]}
            className="mt-4 @3xl:hidden"
          />
        )}
        <div className="mt-4 grid grid-cols-[minmax(0,1fr)] gap-8 @3xl:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] @3xl:items-start @3xl:gap-10">
          <div
            onFocusCapture={() => setRefiningPane('input')}
            className={'min-w-0 ' + (refiningPane === 'preview' ? 'hidden @3xl:block' : '')}
          >
            <Chat
              messages={messages}
              label={t('conversation.log', { kind: t(`kinds.${kind}`) })}
              partnerLabel={t('conversation.ai')}
              ownerLabel={t('conversation.you')}
              composer={
                composing && !historyFull ? (
                  <ChatComposer
                    value={asking ? flow.context.purpose : state.text}
                    onChange={(text) =>
                      send({
                        type: asking ? 'PURPOSE_CHANGED' : 'CHAT_CHANGED',
                        scopeKey,
                        text,
                      })
                    }
                    onSubmit={(submit) => {
                      submit.preventDefault()
                      event(asking ? 'RECOMMEND' : 'REFINE')
                    }}
                    label={t(
                      asking
                        ? kind === 'video-template'
                          ? 'conversation.firstPlaceholderVideo'
                          : 'conversation.firstPlaceholder'
                        : 'conversation.refinePlaceholder',
                    )}
                    placeholder={t(
                      asking
                        ? kind === 'video-template'
                          ? 'conversation.firstPlaceholderVideo'
                          : 'conversation.firstPlaceholder'
                        : 'conversation.refinePlaceholder',
                    )}
                    sendLabel={asking ? t('conversation.first') : t('send')}
                    pending={working}
                    disabled={ai !== 'ready' || blocked || frozenPublication}
                    invalid={!!flow.context.problem}
                    feedback={inputFeedback}
                    attachment={
                      referencesAllowed && (
                        <AuthoringReferences
                          ownerId={ownerId}
                          references={references}
                          onChange={setReferences}
                          // A refused request leaves its chips editable, so one that could not
                          // be read can be taken out before sending again.
                          disabled={busy || working}
                        />
                      )
                    }
                  />
                ) : historyFull ? (
                  <Typography variant="body" className="text-content-secondary">
                    {t('historyFull')}
                  </Typography>
                ) : undefined
              }
            />
            {/* Under the conversation rather than in the composer, which stays pinned to the
                screen's foot: a pinned notice would cover the chat once text is enlarged. */}
            {composing && !historyFull && aiAvailability}
            {active &&
              state.session &&
              renderInspection?.({
                ownerId,
                sessionId: state.session.id,
                kind,
                revision: state.session.revision,
                mode: asking || state.session.turns.length === 0 ? 'recommend' : 'refine',
                prompt: asking ? flow.context.purpose : state.text,
                model: write.selected ?? undefined,
                candidateCount,
                operationId: state.session.activeJobId,
                sourceDirty: Boolean(state.sourceDirty),
              })}
          </div>
          <aside
            aria-label={t('conversation.draft')}
            className={
              'lg:top-chrome min-w-0 lg:sticky ' +
              (refiningPane === 'input' ? 'hidden @3xl:block' : '')
            }
          >
            {draftPane}
          </aside>
        </div>
        <Dialog
          open={saveAsk}
          title={t('conversation.saveTitle', { name: draftName || name })}
          onClose={() => setSaveAsk(false)}
          confirmLabel={t('conversation.save')}
          cancelLabel={t('back')}
          onConfirm={() => {
            setSaveAsk(false)
            send({ type: 'FINISH_REFINEMENT', scopeKey })
            send({ type: 'OPEN_PUBLICATION', scopeKey })
            send({ type: 'PUBLISH', scopeKey })
          }}
        >
          <Typography variant="body">
            {kind === 'writing-voice'
              ? t('voiceCopy')
              : t(targetId ? `publicationPreserves.${kind}` : `publicationCreates.${kind}`)}
          </Typography>
        </Dialog>
        {dialogs}
      </section>
    )
  }
  return (
    <section aria-labelledby={id + '-title'} className={'@container min-w-0 ' + (className ?? '')}>
      <div ref={heading}>
        {canGoBack &&
          !onNavigationChange &&
          (conversation ? (
            // Inside a conversation the page already has its own way back; this one returns to
            // the chat, and says so, so the two are never mistaken for each other.
            <Button
              variant="secondary"
              size="compact"
              className="mb-4 gap-2 sm:mb-6"
              onClick={() => event('BACK')}
            >
              <MessagesSquare aria-hidden="true" className="size-4" />
              {t('conversation.backToChat')}
            </Button>
          ) : (
            <Button
              variant="secondary"
              size="icon"
              aria-label={t('back')}
              title={t('back')}
              className="mb-2 sm:mb-6"
              onClick={() => event('BACK')}
            >
              <ReturnIcon />
            </Button>
          ))}
        <Typography variant="meta" className="text-content-secondary mb-2 block sm:mb-3">
          {t('namedSetting', named)}
        </Typography>
        <Typography
          variant="stepTitle"
          as="h2"
          tabIndex={-1}
          id={id + '-title'}
          className="break-words"
        >
          {view === 'purpose'
            ? t(`purposeQuestions.${kind}`)
            : view === 'review' && state.session?.savedAvailable
              ? t('reviewNamed', named)
              : t(`steps.${view}`)}
        </Typography>
        {view !== 'working' && view !== 'restoring' && view !== 'confirmed' && (
          <Typography variant="body" className="text-content-secondary max-w-measure mt-2 sm:mt-3">
            {t(`stepHelp.${view}`)}
          </Typography>
        )}
        <Typography variant="body" role="status" aria-live="polite" className="mt-4 empty:hidden">
          {view === 'working'
            ? workingStatus
            : view === 'restoring'
              ? t('checking')
              : view === 'confirmed'
                ? t(
                    state.session?.saved?.outcome === 'created'
                      ? 'confirmedCreate'
                      : state.session?.saved?.outcome === 'updated'
                        ? 'confirmedUpdate'
                        : targetId && kind !== 'writing-voice'
                          ? 'confirmedUpdate'
                          : 'confirmedCreate',
                    {
                      kind: t(`kinds.${kind}`),
                      name: state.session?.saved?.name || name,
                    },
                  )
                : ''}
        </Typography>
      </div>
      {state.session?.savedAvailable && view !== 'confirmed' && (
        <Typography variant="body" className="text-content-secondary mt-4">
          {t('savedUsable', named)}
        </Typography>
      )}
      {(state.session?.hasUnpublishedChanges || state.sourceDirty) && (
        <Typography variant="body" role="status" className="mt-4">
          {t('unpublished', named)}
        </Typography>
      )}
      {invalidNotice && (
        <Notice tone="danger" className="mt-4">
          {t(selected ? 'invalidSource' : 'invalidSourceNoPreview')}
        </Notice>
      )}
      {requestFailureNotice('mt-6')}
      {otherFailure && (
        <Notice tone="danger" role="alert" className="mt-6">
          {conflict ? (
            t('conflict')
          ) : failure ? (
            <AppFailureMessage failure={failure} />
          ) : (
            t('failed')
          )}
        </Notice>
      )}
      {controller.readFailure && (
        <Notice tone="danger" role="alert" className="mt-6">
          <AppFailureMessage failure={controller.readFailure} />
          <Button variant="ghost" className="mt-3" onClick={controller.retryRead}>
            {t('retryRead')}
          </Button>
        </Notice>
      )}
      {active &&
        state.session &&
        renderInspection?.({
          ownerId,
          sessionId: state.session.id,
          kind,
          revision: state.session.revision,
          mode:
            view === 'purpose' ||
            view === 'choices' ||
            state.session.phase === 'generating' ||
            (view !== 'refining' &&
              view !== 'direct' &&
              state.session.phase !== 'refining' &&
              state.session.turns.length === 0)
              ? 'recommend'
              : 'refine',
          prompt: view === 'purpose' ? flow.context.purpose : state.text,
          model: write.selected ?? undefined,
          candidateCount,
          operationId: state.session.activeJobId,
          sourceDirty: Boolean(state.sourceDirty),
        })}
      {view === 'purpose' && (
        <form
          className="max-w-measure mt-8"
          onSubmit={(submit) => {
            submit.preventDefault()
            event('RECOMMEND')
          }}
        >
          <Textarea
            aria-labelledby={id + '-title'}
            aria-describedby={flow.context.problem ? id + '-error' : id + '-hint'}
            aria-invalid={!!flow.context.problem}
            rows={3}
            autoGrow
            value={flow.context.purpose}
            onChange={(change) =>
              send({ type: 'PURPOSE_CHANGED', scopeKey, text: change.target.value })
            }
            placeholder={t(`examples.${kind}`)}
            disabled={blocked}
            className="max-h-field"
          />
          <Typography variant="body" id={id + '-hint'} className="text-content-secondary mt-3">
            {t(`purposeHints.${kind}`)}
          </Typography>
          {guidelineKind && !targetId && (
            <Typography variant="body" className="text-content-secondary mt-3">
              {t('newGlobalScope')}
            </Typography>
          )}
          {characters > 0 && (
            <Typography variant="meta" className="mt-3 block">
              {t('limit', { count: characters })}
            </Typography>
          )}
          {inputFeedback}
          <Button
            variant="ghost"
            className="mt-3"
            disabled={blocked}
            onClick={() => send({ type: 'PURPOSE_CHANGED', scopeKey, text: t(`examples.${kind}`) })}
          >
            {t('useExample')}
          </Button>
          {aiAvailability}
          {otherFailure ? (
            retry
          ) : ai === 'ready' ? (
            <Button type="submit" variant="cta" className="mt-8 w-full sm:w-auto">
              <Sparkles aria-hidden="true" className="size-4" />
              {t('recommendCount', { count: candidateCount })}
            </Button>
          ) : null}
        </form>
      )}
      {view === 'existing' && (
        <div className="mt-8">
          {otherFailure ? (
            retry
          ) : (
            <Button variant="cta" onClick={() => event('LOAD_EXISTING')}>
              {t('reviewNamed', named)}
            </Button>
          )}
        </div>
      )}
      {view === 'choices' && (
        <div className="mt-8">
          <div
            aria-label={t('suggestionsCount', {
              count: state.session?.candidateCount ?? candidateCount,
            })}
            className="grid grid-cols-[minmax(0,1fr)] gap-4 @lg:grid-cols-2 @6xl:grid-cols-4"
          >
            {state.session?.candidates.map((candidate) => (
              <ChoiceButton
                key={candidate.id}
                title={candidate.name}
                description={candidate.description}
                selected={selected?.id === candidate.id}
                disabled={blocked || frozenPublication}
                onClick={() => send({ type: 'CHOOSE', scopeKey, candidateId: candidate.id })}
              />
            ))}
          </div>
          {otherFailure ? (
            retry
          ) : (
            <Button variant="ghost" className="mt-8" onClick={() => event('RECOMMEND')}>
              {t('rerollCount', { count: candidateCount })}
            </Button>
          )}
        </div>
      )}
      {view === 'review' && selected && (
        <div className="mt-8">
          {preview}
          <div className="mt-8 flex flex-wrap items-center gap-4">
            {otherFailure ? (
              retry
            ) : (
              <Button
                variant="cta"
                className="w-full sm:w-auto"
                onClick={() =>
                  event(
                    canSaveDraft
                      ? 'OPEN_PUBLICATION'
                      : draftInvalid && !needsVoiceExample
                        ? 'OPEN_DIRECT'
                        : 'OPEN_REFINEMENT',
                  )
                }
              >
                {t(
                  canSaveDraft
                    ? 'continueSave'
                    : draftInvalid && !needsVoiceExample
                      ? 'fixSource'
                      : 'prepareVoiceExample',
                )}
              </Button>
            )}
            <Button variant="ghost" disabled={blocked} onClick={() => event('OPEN_REFINEMENT')}>
              {t('aiNamed', named)}
            </Button>
            <Button variant="ghost" disabled={blocked} onClick={() => event('OPEN_DIRECT')}>
              {t('directNamed', named)}
            </Button>
            {state.session?.hasUnpublishedChanges && (
              <Button variant="ghost" onClick={() => event('OPEN_DIRECT')}>
                {t('continueNamed', named)}
              </Button>
            )}
          </div>
        </div>
      )}
      {view === 'direct' && source && (
        <div className="mt-4 sm:mt-8">
          <SegmentedControl
            value={directPane}
            onChange={setDirectPane}
            ariaLabel={t('directViews')}
            options={[
              { value: 'input', label: t('directInput') },
              { value: 'preview', label: t('previewView') },
            ]}
            className="mb-4 lg:hidden"
          />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-8 lg:grid-cols-2 lg:items-start lg:gap-12">
            <aside
              className={
                directPane === 'input'
                  ? 'lg:top-chrome hidden lg:sticky lg:block'
                  : 'lg:top-chrome lg:sticky'
              }
            >
              {preview}
            </aside>
            <form
              onFocusCapture={() => setDirectPane('input')}
              className={
                'min-w-0 space-y-4 sm:space-y-6 ' +
                (directPane === 'preview' ? 'hidden lg:block' : '')
              }
              onSubmit={(e) => {
                e.preventDefault()
                event('FINISH_DIRECT')
              }}
            >
              {renderDirectEditor ? (
                renderDirectEditor({ source, onChange: controller.setSource, disabled: blocked })
              ) : (
                <>
                  <label className="block">
                    <Typography variant="fieldTitle" as="span">
                      {t('fields.name')}
                    </Typography>
                    <TextField
                      value={source.name}
                      onChange={(e) => changeSource('name', e.target.value)}
                      maxLength={200}
                      className="mt-3"
                    />
                  </label>
                  <label className="block">
                    <Typography variant="fieldTitle" as="span">
                      {t('fields.description')}
                    </Typography>
                    <TextField
                      value={source.description}
                      onChange={(e) => changeSource('description', e.target.value)}
                      maxLength={200}
                      className="mt-3"
                    />
                  </label>
                  <label className="block">
                    <Typography variant="fieldTitle" as="span">
                      {t('fields.body')}
                    </Typography>
                    <Textarea
                      value={source.body}
                      onChange={(e) => changeSource('body', e.target.value)}
                      rows={10}
                      autoGrow
                      className="max-h-field mt-3"
                    />
                  </label>
                  {kind === 'post-template' && (
                    <label className="block">
                      <Typography variant="fieldTitle" as="span">
                        {t('fields.titleArea')}
                      </Typography>
                      <Textarea
                        value={source.titleArea}
                        onChange={(e) => changeSource('titleArea', e.target.value)}
                        rows={3}
                        autoGrow
                        className="mt-3"
                      />
                    </label>
                  )}
                </>
              )}
              <Typography variant="body" className="text-content-secondary">
                {t('directHelp')}
              </Typography>
              <div className="flex flex-wrap gap-4">
                <Button variant="cta" type="submit">
                  {t('keepDirect')}
                </Button>
                <Button variant="ghost" onClick={() => event('OPEN_REFINEMENT')}>
                  {t('aiNamed', named)}
                </Button>
              </div>
              {state.session?.savedBaseline && (
                <Button variant="ghost" onClick={() => event('ASK_RESET')}>
                  {t('resetNamed', named)}
                </Button>
              )}
            </form>
          </div>
        </div>
      )}
      {view === 'refining' && (selected || source) && (
        <div className="mt-4 sm:mt-8">
          <SegmentedControl
            value={refiningPane}
            onChange={setRefiningPane}
            ariaLabel={t('refiningViews')}
            options={[
              { value: 'input', label: t('chat') },
              { value: 'preview', label: t('previewView') },
            ]}
            className="mb-4 @3xl:hidden"
          />
          <div className="grid grid-cols-[minmax(0,1fr)] gap-8 @3xl:grid-cols-2 @3xl:items-start @3xl:gap-12">
            <aside className={refiningPane === 'input' ? 'hidden @3xl:block' : ''}>{preview}</aside>
            <div
              onFocusCapture={() => setRefiningPane('input')}
              className={'min-w-0 ' + (refiningPane === 'preview' ? 'hidden @3xl:block' : '')}
            >
              {historyFull ? (
                <div className="space-y-6">
                  <Typography variant="body">{t('historyFull')}</Typography>
                  <Button
                    variant="cta"
                    onClick={() => event(selected ? 'FINISH_REFINEMENT' : 'OPEN_DIRECT')}
                  >
                    {t(selected ? 'finishRefinement' : 'directNamed', named)}
                  </Button>
                </div>
              ) : (
                <form
                  onSubmit={(submit) => {
                    submit.preventDefault()
                    event('REFINE')
                  }}
                >
                  <Textarea
                    aria-labelledby={id + '-title'}
                    aria-describedby={flow.context.problem ? id + '-error' : undefined}
                    aria-invalid={!!flow.context.problem}
                    rows={4}
                    autoGrow
                    value={state.text}
                    onChange={(change) =>
                      send({ type: 'CHAT_CHANGED', scopeKey, text: change.target.value })
                    }
                    disabled={blocked || frozenPublication}
                    placeholder={t(`refineExamples.${kind}`)}
                    className="max-h-field"
                  />
                  {characters > 0 && (
                    <Typography variant="meta" className="mt-3 block">
                      {t('limit', { count: characters })}
                    </Typography>
                  )}
                  {inputFeedback}
                  <Button
                    variant="ghost"
                    className="mt-3"
                    disabled={blocked}
                    onClick={() =>
                      send({ type: 'CHAT_CHANGED', scopeKey, text: t(`refineExamples.${kind}`) })
                    }
                  >
                    {t('useExample')}
                  </Button>
                  {aiAvailability}
                  {otherFailure ? (
                    retry
                  ) : ai === 'ready' ? (
                    <Button type="submit" variant="cta" className="mt-4 w-full sm:mt-8 sm:w-auto">
                      <SendHorizontal aria-hidden="true" className="size-4" />
                      {t('send')}
                    </Button>
                  ) : null}
                  <div className="mt-4">
                    <Button
                      variant="ghost"
                      disabled={blocked}
                      onClick={() => event(selected ? 'FINISH_REFINEMENT' : 'OPEN_DIRECT')}
                    >
                      {t(selected ? 'finishRefinement' : 'directNamed', named)}
                    </Button>
                  </div>
                </form>
              )}
              {state.session?.turns.length ? (
                <Disclosure
                  title={t('previousConversation', { count: state.session.turns.length })}
                  headingLevel={3}
                  size="row"
                  className="mt-4"
                >
                  <ol aria-label={t('chat')} className="mt-3 space-y-6">
                    {state.session.turns.map((turn) => (
                      <li key={turn.id} className="min-w-0 space-y-3">
                        <Typography variant="body" className="break-words whitespace-pre-wrap">
                          {turn.request}
                        </Typography>
                        {turn.status === 'done' && turn.reply && (
                          <Typography
                            variant="body"
                            className="text-content-secondary break-words whitespace-pre-wrap"
                          >
                            {readableAuthoringProse(turn.reply) ? turn.reply : t('replyReady')}
                          </Typography>
                        )}
                      </li>
                    ))}
                  </ol>
                </Disclosure>
              ) : null}
              <div className="mt-4 flex flex-wrap gap-2 sm:mt-6 sm:gap-4">
                <Button variant="ghost" onClick={() => event('OPEN_DIRECT')}>
                  {t('directNamed', named)}
                </Button>
                <Button variant="ghost" onClick={() => event('FRESH')}>
                  {t('freshNamed', named)}
                </Button>
                {state.session?.savedBaseline && (
                  <Button variant="ghost" onClick={() => event('ASK_RESET')}>
                    {t('resetNamed', named)}
                  </Button>
                )}
              </div>
              <Typography variant="body" className="text-content-secondary mt-3">
                {t('freshHelp', named)}
              </Typography>
            </div>
          </div>
        </div>
      )}
      {view === 'publication' && selected && (
        <div className="max-w-measure mt-8">
          <Typography variant="fieldTitle" as="h3" className="break-words">
            {selected.name || name}
          </Typography>
          <Typography variant="body" className="text-content-secondary mt-3">
            {t(
              kind === 'writing-voice'
                ? 'voiceCopy'
                : targetId
                  ? `publicationPreserves.${kind}`
                  : `publicationCreates.${kind}`,
            )}
          </Typography>
          {guidelineKind && (
            <Typography variant="body" className="mt-4">
              {t(
                selected.scope === 'global'
                  ? 'scopeGlobal'
                  : selected.scope === 'templates'
                    ? 'scopeTemplates'
                    : selected.scope === 'fields'
                      ? 'scopeFields'
                      : targetId
                        ? 'scopeRetained'
                        : 'scopeRequired',
                {
                  count:
                    selected.scope === 'templates'
                      ? (selected.templateIds?.length ?? 0)
                      : (selected.fields?.length ?? 0),
                },
              )}
            </Typography>
          )}
          {kind === 'writing-voice' && !recoveredSave && (
            <label className="mt-6 flex min-h-11 cursor-pointer items-center gap-4 py-3">
              <Checkbox
                checked={flow.context.makeDefault}
                onChange={(change) =>
                  send({ type: 'DEFAULT_CHANGED', scopeKey, checked: change.target.checked })
                }
                disabled={frozenPublication}
              />
              <Typography variant="body" as="span">
                {t('defaultVoice')}
              </Typography>
            </label>
          )}
          {otherFailure ? (
            retry
          ) : (
            <Button
              variant="cta"
              className="mt-8 w-full sm:w-auto"
              onClick={() => event('PUBLISH')}
            >
              {t(
                recoveredSave
                  ? 'saveRetry'
                  : kind === 'writing-voice'
                    ? 'saveVoice'
                    : targetId
                      ? 'saveChangesNamed'
                      : 'saveNewNamed',
                named,
              )}
            </Button>
          )}
        </div>
      )}
      {view === 'working' && (
        <div className="mt-8">
          {state.phase === 'active' && state.session?.activeJobId && (
            <Button variant="secondary" className="mt-6" onClick={() => event('ASK_CANCEL')}>
              {t('cancel')}
            </Button>
          )}
          {selected && (
            <div className="mt-8">
              <Typography variant="body" className="text-content-secondary max-w-measure mb-6">
                {t('previousDraft', { name: selected.name })}
              </Typography>
              {preview}
            </div>
          )}
        </div>
      )}
      {view === 'restoring' && retry}
      {view === 'confirmed' && (
        <div className="mt-8">
          {state.session?.saved && (
            <Typography variant="fieldTitle" className="break-words">
              {state.session.saved.name || name}
            </Typography>
          )}
          <Button variant="cta" className="mt-8" onClick={() => event('CONTINUE')}>
            {t('continueNamed', named)}
          </Button>
        </div>
      )}
      {dialogs}
    </section>
  )
}

/** The host's model choice, drawn as a component so the save callback reaches it as a prop
 *  rather than through a call made while this editor renders. */
function ModelChoiceSlot({
  render,
  onSaved,
}: {
  render: (onSaved: () => void) => ReactNode
  onSaved: () => void
}) {
  return render(onSaved)
}
