import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { SendHorizontal, Check, Sparkles } from 'lucide-react'
import {
  authoringScopeKey,
  completedAuthoringExchanges,
  AUTHORING_MESSAGE_MAX_CHARS,
  AUTHORING_MAX_EXCHANGES,
  type AuthoringScope,
  type AuthoringSavedRef,
  type AuthoringArtifact,
} from '@/entities/ai-authoring'
import { useInitializeDefaultSelections, useStageSelection } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Badge,
  Button,
  Checkbox,
  Dialog,
  FieldLabel,
  Notice,
  Textarea,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import { readableAuthoringProse } from '../lib/readable-prose'
import { useAuthoring } from '../model/useAuthoring'
import { AuthoringPreview } from './AuthoringPreview'

export interface AuthoringEditorProps extends AuthoringScope {
  onSaved?: (ref: AuthoringSavedRef) => void
  onBusyChange?: (busy: boolean) => void
  renderPreview?: (artifact: AuthoringArtifact) => ReactNode
  className?: string
  initialMakeDefault?: boolean
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
}: AuthoringEditorProps) {
  const { t } = useTranslation('authoring')
  const scope = { ownerId, kind, targetId }
  const controller = useAuthoring(scope, { onSaved, onBusyChange })
  const { state, busy } = controller
  const write = useStageSelection('write')
  const defaults = useInitializeDefaultSelections(ownerId)
  const id = useId()
  const [makeDefault, setMakeDefault] = useState(kind === 'writing-voice' && initialMakeDefault)
  const [cancelOpen, setCancelOpen] = useState(false)
  const [candidatesOpen, setCandidatesOpen] = useState(false)
  const selectionToFocus = useRef<string | undefined>(undefined)
  const heading = useRef<HTMLDivElement>(null)
  const selected = state.session?.selected
  const selectedId = selected?.id
  useEffect(() => {
    if (selectedId && selectionToFocus.current === selectedId) {
      selectionToFocus.current = undefined
      heading.current?.querySelector('h3')?.focus()
    }
  }, [selectedId])
  const characters = Array.from(state.text).length
  const completed = completedAuthoringExchanges(state.session)
  const canRequest =
    !!write.selected &&
    !write.isPending &&
    !defaults.isPending &&
    !busy &&
    !state.command &&
    state.phase !== 'checking' &&
    state.phase !== 'saved' &&
    characters <= AUTHORING_MESSAGE_MAX_CHARS
  const canRefine =
    canRequest && !!selected && state.text.trim() !== '' && completed < AUTHORING_MAX_EXCHANGES
  const recoveredSave =
    state.session?.phase === 'saving' && !state.session.activeJobId && state.phase === 'active'
  const failure = state.failure
  const conflict =
    failure?.reason === 'AUTHORING_SAVE_CONFLICT' ||
    failure?.reason === 'AUTHORING_REVISION_CONFLICT'
  const status =
    state.phase === 'checking'
      ? t('checking')
      : state.phase === 'quoting'
        ? t('quoting')
        : state.phase === 'saving' || recoveredSave
          ? t('saving')
          : state.phase === 'active' ||
              state.phase === 'starting' ||
              state.phase === 'creating' ||
              state.phase === 'selecting' ||
              state.phase === 'cancelling'
            ? t('working')
            : state.phase === 'saved'
              ? t('saved')
              : ''
  const request = (mode: 'recommend' | 'refine') => {
    if (write.selected) void controller.quote(mode, write.selected)
  }
  return (
    <section aria-labelledby={`${id}-title`} className={`@container min-w-0 ${className ?? ''}`}>
      <Typography variant="title" id={`${id}-title`}>
        {t('title', { kind: t(`kinds.${kind}`) })}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-3">
        {t(targetId ? 'existing' : 'intro')}
      </Typography>
      <Typography variant="body" role="status" aria-live="polite" className="mt-4 empty:hidden">
        {status}
      </Typography>
      {(failure || state.session?.phase === 'failed') && (
        <Notice tone="danger" role="alert" className="mt-4">
          {conflict ? (
            t('conflict')
          ) : failure ? (
            <AppFailureMessage failure={failure} />
          ) : (
            t('failed')
          )}
          <Button
            variant="ghost"
            className="mt-2"
            disabled={busy}
            onClick={() => void controller.retryOperation()}
          >
            {t('retry')}
          </Button>
          {conflict && (
            <Button variant="secondary" className="mt-2" disabled={busy} onClick={controller.fresh}>
              {t('fresh')}
            </Button>
          )}
        </Notice>
      )}
      {controller.readFailure && (
        <Notice tone="danger" role="alert" className="mt-4">
          <AppFailureMessage failure={controller.readFailure} />
          <Button variant="ghost" className="mt-2" onClick={controller.retryRead}>
            {t('retryRead')}
          </Button>
        </Notice>
      )}
      {(defaults.isPending || write.isPending || !write.selected) && (
        <div className="mt-4">
          <Typography variant="body" className="text-content-secondary">
            {t(defaults.isPending || write.isPending ? 'modelPreparing' : 'modelUnavailable')}
          </Typography>
          {!defaults.isPending && !write.isPending && (
            <div className="mt-2 flex flex-wrap gap-3">
              <Button variant="ghost" onClick={defaults.retry}>
                {t('retryAI')}
              </Button>
              <Link to="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
                {t('settings')}
              </Link>
            </div>
          )}
        </div>
      )}
      {!selected &&
        !state.session?.candidates.length &&
        state.phase !== 'checking' &&
        state.phase !== 'saved' && (
          <div className="mt-8">
            {targetId ? (
              <Button
                variant="cta"
                disabled={busy}
                pending={state.phase === 'creating'}
                onClick={() => void controller.edit()}
              >
                {t('editStart')}
              </Button>
            ) : (
              <>
                <FieldLabel htmlFor={`${id}-purpose`}>{t('purpose')}</FieldLabel>
                <Textarea
                  id={`${id}-purpose`}
                  rows={2}
                  autoGrow
                  value={state.text}
                  disabled={busy || !!state.command}
                  onChange={(event) => controller.setText(event.target.value)}
                  placeholder={t('purposePlaceholder')}
                  className="max-h-field mt-2"
                />
                <Typography variant="meta" className="mt-2 block">
                  {t('limit', { count: characters })}
                </Typography>
                <Button
                  variant="secondary"
                  className="mt-3"
                  disabled={busy || !!state.command}
                  onClick={() => controller.setText(t(`examples.${kind}`))}
                >
                  {t(`examples.${kind}`)}
                </Button>
                <Button
                  variant="cta"
                  className="mt-4 w-full sm:w-auto"
                  disabled={!canRequest}
                  onClick={() => request('recommend')}
                >
                  <Sparkles aria-hidden="true" className="size-4" />
                  {t('recommend')}
                </Button>
              </>
            )}
          </div>
        )}
      {!!state.session?.candidates.length && selected && (
        <div className="mt-6 flex min-w-0 flex-wrap items-center justify-between gap-3">
          <Typography variant="body" className="min-w-0 break-words">
            {t('selectedChoice', { name: selected.name })}
          </Typography>
          <Button
            variant="ghost"
            disabled={busy}
            aria-expanded={candidatesOpen}
            aria-controls={`${id}-candidates`}
            onClick={() => setCandidatesOpen(!candidatesOpen)}
          >
            {t(candidatesOpen ? 'hideChoices' : 'changeChoice')}
          </Button>
        </div>
      )}
      {!!state.session?.candidates.length && (
        <section
          id={`${id}-candidates`}
          hidden={!!selected && !candidatesOpen}
          aria-labelledby={`${id}-suggestions`}
          className="mt-8"
        >
          <div className="flex flex-wrap items-center justify-between gap-3">
            <Typography variant="title" as="h3" id={`${id}-suggestions`}>
              {t('suggestions')}
            </Typography>
            <Button variant="ghost" disabled={!canRequest} onClick={() => request('recommend')}>
              {t('reroll')}
            </Button>
          </div>
          <div className="mt-4 grid gap-4 @lg:grid-cols-2 @6xl:grid-cols-4">
            {state.session!.candidates.map((candidate) => (
              <article key={candidate.id} className="bg-surface-raised min-w-0 rounded-lg p-4">
                <Typography variant="fieldTitle" as="h4" className="break-words">
                  {candidate.name}
                </Typography>
                <Typography variant="body" className="text-content-secondary mt-2 break-words">
                  {candidate.description}
                </Typography>
                {selected?.id === candidate.id && (
                  <Badge className="mt-3">
                    <Check aria-hidden="true" className="mr-1 inline size-3" />
                    {t('selected')}
                  </Badge>
                )}
                <Button
                  variant="secondary"
                  className="mt-4 w-full"
                  disabled={busy || !!state.command || state.phase === 'saved'}
                  aria-pressed={selected?.id === candidate.id}
                  onClick={() => {
                    selectionToFocus.current = candidate.id
                    setCandidatesOpen(false)
                    controller.select(candidate.id)
                  }}
                >
                  {t('choose')}
                </Button>
              </article>
            ))}
          </div>
        </section>
      )}
      {selected && (
        <div className="mt-10 grid gap-8 @3xl:grid-cols-2 @3xl:items-start @3xl:gap-12">
          <div ref={heading} className="min-w-0">
            <Typography variant="title" as="h3" tabIndex={-1} className="break-words">
              {selected.name}
            </Typography>
            <Typography variant="body" className="text-content-secondary mt-3 break-words">
              {selected.description}
            </Typography>
            <div className="mt-6">
              {renderPreview ? (
                renderPreview(selected)
              ) : (
                <AuthoringPreview kind={kind} artifact={selected} />
              )}
            </div>
          </div>
          <section aria-labelledby={`${id}-chat`} className="min-w-0">
            <Typography variant="title" as="h3" id={`${id}-chat`}>
              {t('chat')}
            </Typography>
            {state.session!.turns.length ? (
              <ol className="mt-4 space-y-6">
                {state.session!.turns.map((turn) => (
                  <li key={turn.id} className="min-w-0 space-y-3">
                    <Typography
                      variant="body"
                      className="bg-surface-recessed rounded-lg p-4 break-words whitespace-pre-wrap"
                    >
                      {turn.request}
                    </Typography>
                    {turn.status === 'done' && turn.reply && (
                      <Typography variant="body" className="break-words whitespace-pre-wrap">
                        {readableAuthoringProse(turn.reply) ? turn.reply : t('replyReady')}
                      </Typography>
                    )}
                  </li>
                ))}
              </ol>
            ) : (
              <Typography variant="body" className="text-content-secondary mt-4">
                {t('historyEmpty')}
              </Typography>
            )}
            {completed >= AUTHORING_MAX_EXCHANGES && (
              <Notice tone="info" className="mt-6">
                {t('historyFull')}
              </Notice>
            )}
            <form
              className="mt-6"
              onSubmit={(event) => {
                event.preventDefault()
                if (canRefine) request('refine')
              }}
            >
              <FieldLabel htmlFor={`${id}-message`}>{t('chatLabel')}</FieldLabel>
              <Textarea
                id={`${id}-message`}
                rows={3}
                autoGrow
                value={state.text}
                onChange={(event) => controller.setText(event.target.value)}
                disabled={busy || !!state.command || state.phase === 'saved'}
                placeholder={t('chatPlaceholder')}
                className="max-h-field mt-2"
              />
              <Typography variant="meta" className="mt-2 block">
                {t('limit', { count: characters })}
              </Typography>
              <div className="mt-3 flex flex-wrap gap-2">
                {(['concise', 'friendly', 'simple'] as const).map((chip) => (
                  <Button
                    key={chip}
                    variant="ghost"
                    disabled={busy || !!state.command || state.phase === 'saved'}
                    onClick={() => controller.setText(t(`chips.${chip}`))}
                  >
                    {t(`chips.${chip}`)}
                  </Button>
                ))}
              </div>
              <Button
                type="submit"
                variant="secondary"
                className="mt-3 w-full sm:w-auto"
                disabled={!canRefine}
              >
                <SendHorizontal aria-hidden="true" className="size-4" />
                {t('send')}
              </Button>
            </form>
            {kind === 'writing-voice' && (
              <div className="mt-6 space-y-3">
                <Typography variant="body" className="text-content-secondary">
                  {t('voiceCopy')}
                </Typography>
                <label className="flex min-h-11 cursor-pointer items-center gap-4 px-3">
                  <Checkbox
                    checked={makeDefault}
                    onChange={(event) => setMakeDefault(event.target.checked)}
                    disabled={busy || state.phase === 'saved'}
                  />
                  <Typography variant="body" as="span">
                    {t('defaultVoice')}
                  </Typography>
                </label>
              </div>
            )}
            {state.phase !== 'saved' && (
              <Button
                variant="cta"
                className="mt-6 w-full sm:w-auto"
                pending={state.phase === 'saving'}
                disabled={(busy && !recoveredSave) || !!state.command}
                onClick={() => controller.save(makeDefault)}
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
          </section>
        </div>
      )}
      {state.phase === 'active' && state.session?.activeJobId && (
        <Button variant="secondary" className="mt-6" onClick={() => setCancelOpen(true)}>
          {t('cancel')}
        </Button>
      )}
      {state.phase === 'saved' && (
        <Button variant="secondary" className="mt-6" onClick={controller.fresh}>
          {t('fresh')}
        </Button>
      )}
      <Dialog
        open={state.phase === 'confirming' || state.phase === 'starting'}
        title={t('confirmTitle')}
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
        open={cancelOpen}
        title={t('cancelTitle')}
        onClose={() => setCancelOpen(false)}
        confirmLabel={t('cancel')}
        cancelLabel={t('keepWorking')}
        onConfirm={() => {
          setCancelOpen(false)
          controller.cancel()
        }}
      >
        <Typography variant="body">{t('cancelHelp')}</Typography>
      </Dialog>
    </section>
  )
}
