import { useCallback, useEffect, useId, useRef, type ReactNode } from 'react'
import { useMachine } from '@xstate/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, SendHorizontal, Sparkles } from 'lucide-react'
import {
  authoringScopeKey,
  completedAuthoringExchanges,
  AUTHORING_MAX_EXCHANGES,
  type AuthoringScope,
  type AuthoringSavedRef,
  type AuthoringArtifact,
} from '@/entities/ai-authoring'
import { useInitializeDefaultSelections, useStageSelection } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Button,
  Checkbox,
  ChoiceButton,
  Dialog,
  FieldMessage,
  Notice,
  Textarea,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { readableAuthoringProse } from '../lib/readable-prose'
import { useAuthoring } from '../model/useAuthoring'
import {
  studioFlowMachine,
  studioFlowView,
  type StudioAIAvailability,
} from '../model/studio-flow-machine'
import { AuthoringPreview } from './AuthoringPreview'

export interface AuthoringEditorProps extends AuthoringScope {
  onSaved?: (ref: AuthoringSavedRef) => void
  onBusyChange?: (busy: boolean) => void
  renderPreview?: (artifact: AuthoringArtifact) => ReactNode
  className?: string
  initialMakeDefault?: boolean
  active?: boolean
  onNavigationChange?: (navigation: StudioNavigation | undefined) => void
}
export interface StudioNavigation {
  canGoBack: boolean
  goBack: () => void
}
export function AuthoringEditor(props: AuthoringEditorProps) {
  return <ScopedEditor key={authoringScopeKey(props)} {...props} />
}
function ScopedEditor({
  ownerId,
  kind,
  targetId,
  onSaved,
  onBusyChange,
  renderPreview,
  className,
  initialMakeDefault = false,
  active = true,
  onNavigationChange,
}: AuthoringEditorProps) {
  const { t } = useTranslation('authoring')
  const scope = { ownerId, kind, targetId }
  const scopeKey = authoringScopeKey(scope)
  // Navigation never mounts a second operation consumer, poller or save queue.
  const controller = useAuthoring(scope, { onSaved, onBusyChange })
  const { state, busy } = controller
  const write = useStageSelection('write')
  const defaults = useInitializeDefaultSelections(ownerId)
  const ai: StudioAIAvailability =
    defaults.isPending || write.isPending ? 'preparing' : write.selected ? 'ready' : 'unavailable'
  const [flow, send] = useMachine(
    studioFlowMachine.provide({
      actions: {
        setChat: ({ event }) => {
          if (event.type === 'CHAT_CHANGED') controller.setText(event.text)
        },
        requestRecommend: ({ context }) => {
          controller.setText(context.purpose)
          if (write.selected) void controller.quote('recommend', write.selected)
        },
        requestRefine: () => {
          if (write.selected) void controller.quote('refine', write.selected)
        },
        loadExisting: () => void controller.edit(),
        selectCandidate: ({ event }) => {
          if (event.type === 'CHOOSE') controller.select(event.candidateId)
        },
        publish: ({ context }) => controller.save(context.makeDefault),
        retry: () => void controller.retryOperation(),
        fresh: controller.fresh,
        cancel: controller.cancel,
      },
    }),
    { input: { scope, operation: state, ai, initialMakeDefault } },
  )
  useEffect(() => {
    send({ type: 'OBSERVE', scopeKey, operation: state, ai })
  }, [state, ai, scopeKey, send])
  const view = studioFlowView(flow)
  const id = useId()
  const heading = useRef<HTMLDivElement>(null)
  const previousView = useRef(view)
  const previouslyActive = useRef(active)
  useEffect(() => {
    if (active && (previousView.current !== view || !previouslyActive.current)) {
      heading.current?.querySelector('h2')?.focus()
    }
    previousView.current = view
    previouslyActive.current = active
  }, [view, active])
  const goBack = useCallback(() => send({ type: 'BACK', scopeKey }), [scopeKey, send])
  const canGoBack = flow.can({ type: 'BACK', scopeKey })
  useEffect(() => {
    onNavigationChange?.({ canGoBack, goBack })
  }, [canGoBack, goBack, onNavigationChange])
  useEffect(() => () => onNavigationChange?.(undefined), [onNavigationChange])
  const selected = state.session?.selected
  const failure = state.failure
  const conflict =
    failure?.reason === 'AUTHORING_SAVE_CONFLICT' ||
    failure?.reason === 'AUTHORING_REVISION_CONFLICT'
  const recoveredSave =
    state.session?.phase === 'saving' && !state.session.activeJobId && state.phase === 'active'
  const canSaveDraft = !!selected?.body.trim()
  const characters = Array.from(view === 'purpose' ? flow.context.purpose : state.text).length
  const blocked = busy || !!state.command
  const hasFailure = !!failure || state.session?.phase === 'failed'
  const frozenPublication = flow.context.intent === 'save' && !!failure
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
  const preview = selected && (
    <div className="min-w-0">
      <Typography variant="fieldTitle" as="h3" className="break-words">
        {selected.name}
      </Typography>
      {selected.description && (
        <Typography variant="body" className="text-content-secondary mt-3 break-words">
          {selected.description}
        </Typography>
      )}
      <div className="mt-6">
        {renderPreview ? (
          renderPreview(selected)
        ) : (
          <AuthoringPreview kind={kind} artifact={selected} />
        )}
      </div>
    </div>
  )
  const retry = hasFailure && (
    <div className="mt-8 flex flex-wrap items-center gap-4">
      <Button variant="cta" onClick={() => event('RETRY')}>
        {t(frozenPublication ? 'saveRetry' : 'retry')}
      </Button>
      {conflict && (
        <Button variant="ghost" onClick={() => event('FRESH')}>
          {t('fresh')}
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
          <Link to="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
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
  return (
    <section aria-labelledby={id + '-title'} className={'@container min-w-0 ' + (className ?? '')}>
      <div ref={heading}>
        {canGoBack && !onNavigationChange && (
          <Button variant="ghost" className="mb-6" onClick={() => event('BACK')}>
            <ArrowLeft aria-hidden="true" className="size-4" />
            {t('back')}
          </Button>
        )}
        <Typography variant="meta" className="text-content-secondary mb-3 block">
          {t(`kinds.${kind}`)}
        </Typography>
        <Typography
          variant="stepTitle"
          as="h2"
          tabIndex={-1}
          id={id + '-title'}
          className="break-words"
        >
          {view === 'purpose' ? t(`purposeQuestions.${kind}`) : t(`steps.${view}`)}
        </Typography>
        {view !== 'working' && view !== 'restoring' && view !== 'confirmed' && (
          <Typography variant="body" className="text-content-secondary max-w-measure mt-3">
            {t(`stepHelp.${view}`)}
          </Typography>
        )}
        <Typography variant="body" role="status" aria-live="polite" className="mt-4 empty:hidden">
          {view === 'working'
            ? workingStatus
            : view === 'restoring'
              ? t('checking')
              : view === 'confirmed'
                ? t('saved')
                : ''}
        </Typography>
      </div>
      {hasFailure && (
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
          {hasFailure ? (
            retry
          ) : ai === 'ready' ? (
            <Button type="submit" variant="cta" className="mt-8 w-full sm:w-auto">
              <Sparkles aria-hidden="true" className="size-4" />
              {t('recommend')}
            </Button>
          ) : null}
        </form>
      )}
      {view === 'existing' && (
        <div className="mt-8">
          {hasFailure ? (
            retry
          ) : (
            <Button variant="cta" onClick={() => event('LOAD_EXISTING')}>
              {t(selected ? 'loadedDraft' : 'editStart')}
            </Button>
          )}
        </div>
      )}
      {view === 'choices' && (
        <div className="mt-8">
          <div
            aria-label={t('suggestions')}
            className="grid gap-4 @lg:grid-cols-2 @6xl:grid-cols-4"
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
          {hasFailure ? (
            retry
          ) : (
            <Button variant="ghost" className="mt-8" onClick={() => event('RECOMMEND')}>
              {t('reroll')}
            </Button>
          )}
        </div>
      )}
      {view === 'review' && selected && (
        <div className="mt-8">
          {preview}
          <div className="mt-8 flex flex-wrap items-center gap-4">
            {hasFailure ? (
              retry
            ) : (
              <Button
                variant="cta"
                className="w-full sm:w-auto"
                onClick={() => event(canSaveDraft ? 'OPEN_PUBLICATION' : 'OPEN_REFINEMENT')}
              >
                {t(canSaveDraft ? 'continueSave' : 'prepareVoiceExample')}
              </Button>
            )}
            {canSaveDraft && (
              <Button variant="ghost" onClick={() => event('OPEN_REFINEMENT')}>
                {t('openRefinement')}
              </Button>
            )}
          </div>
        </div>
      )}
      {view === 'refining' && selected && (
        <div className="mt-8 grid gap-8 @3xl:grid-cols-2 @3xl:items-start @3xl:gap-12">
          {preview}
          <div className="min-w-0">
            {state.session?.turns.length ? (
              <ol aria-label={t('chat')} className="mb-6 space-y-6">
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
            ) : null}
            {historyFull ? (
              <div className="space-y-6">
                <Typography variant="body">{t('historyFull')}</Typography>
                <Button variant="cta" onClick={() => event('FINISH_REFINEMENT')}>
                  {t('finishRefinement')}
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
                {hasFailure ? (
                  retry
                ) : ai === 'ready' ? (
                  <Button type="submit" variant="cta" className="mt-8 w-full sm:w-auto">
                    <SendHorizontal aria-hidden="true" className="size-4" />
                    {t('send')}
                  </Button>
                ) : null}
                <div className="mt-4">
                  <Button
                    variant="ghost"
                    disabled={blocked}
                    onClick={() => event('FINISH_REFINEMENT')}
                  >
                    {t('finishRefinement')}
                  </Button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
      {view === 'publication' && selected && (
        <div className="max-w-measure mt-8">
          <Typography variant="fieldTitle" as="h3" className="break-words">
            {selected.name}
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
          {hasFailure ? (
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
                      ? 'apply'
                      : 'save',
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
              {state.session.saved.name}
            </Typography>
          )}
          <Button variant="cta" className="mt-8" onClick={() => event('FRESH')}>
            {t('fresh')}
          </Button>
        </div>
      )}
      <Dialog
        open={state.phase === 'confirming' || state.phase === 'starting'}
        title={t(state.command?.mode === 'refine' ? 'confirmRefine' : 'confirmRecommend')}
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
        open={flow.matches({ working: 'confirmingCancellation' })}
        title={t('cancelTitle')}
        onClose={() => event('DISMISS_CANCEL')}
        confirmLabel={t('cancel')}
        cancelLabel={t('keepWorking')}
        onConfirm={() => event('CONFIRM_CANCEL')}
      >
        <Typography variant="body">{t('cancelHelp')}</Typography>
      </Dialog>
    </section>
  )
}
