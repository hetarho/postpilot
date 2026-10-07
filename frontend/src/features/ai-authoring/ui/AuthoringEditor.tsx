import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useMachine } from '@xstate/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, SendHorizontal, Sparkles } from 'lucide-react'
import {
  authoringScopeKey,
  completedAuthoringExchanges,
  AUTHORING_MAX_EXCHANGES,
  AUTHORING_NAME_SUMMARY_MAX_CHARS,
  type AuthoringScope,
  type AuthoringSavedRef,
  type AuthoringArtifact,
  type AuthoringCandidateCount,
} from '@/entities/ai-authoring'
import { useInitializeDefaultSelections, useStageSelection } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  TextField,
  Button,
  Checkbox,
  ChoiceButton,
  Disclosure,
  SegmentedControl,
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
  className,
  initialMakeDefault = false,
  active = true,
  onNavigationChange,
}: AuthoringEditorProps) {
  const { t } = useTranslation('authoring')
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
  const [flow, send] = useMachine(
    studioFlowMachine.provide({
      actions: {
        setChat: ({ event }) => {
          if (event.type === 'CHAT_CHANGED') controller.setText(event.text)
        },
        requestRecommend: ({ context }) => {
          controller.setText(context.purpose)
          if (write.selected) void controller.quote('recommend', write.selected, candidateCount)
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
  const guidelineKind = kind === 'post-guideline' || kind === 'video-guideline'
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
  const retry = hasFailure && (
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
          <Button variant="ghost" className="mb-2 sm:mb-6" onClick={() => event('BACK')}>
            <ArrowLeft aria-hidden="true" className="size-4" />
            {t('back')}
          </Button>
        )}
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
      {draftInvalid && !needsVoiceExample && (
        <Notice tone="danger" className="mt-4">
          {t(selected ? 'invalidSource' : 'invalidSourceNoPreview')}
        </Notice>
      )}
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
          {hasFailure ? (
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
          {hasFailure ? (
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
              {t('rerollCount', { count: candidateCount })}
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
          <div className="grid gap-8 lg:grid-cols-2 lg:items-start lg:gap-12">
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
          <div className="grid gap-8 @3xl:grid-cols-2 @3xl:items-start @3xl:gap-12">
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
                  {hasFailure ? (
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
      <Dialog
        open={state.phase === 'confirming' || state.phase === 'starting'}
        title={t(state.command?.mode === 'refine' ? 'confirmRefine' : 'confirmRecommendCount', {
          count: state.command?.candidateCount ?? candidateCount,
        })}
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
    </section>
  )
}
