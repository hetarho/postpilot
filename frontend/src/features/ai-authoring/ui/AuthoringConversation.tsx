import { useEffect, useRef, useState, type FormEvent, type ReactNode, type Ref } from 'react'
import type { SnapshotFrom } from 'xstate'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Ellipsis, Eye, Sparkles } from 'lucide-react'
import type { AuthoringKind } from '@/entities/ai-authoring'
import {
  ActionMenu,
  Button,
  Chat,
  ChatComposer,
  ChoiceButton,
  SegmentedControl,
  Skeleton,
  Typography,
  type ActionMenuItem,
  type ChatMessage,
} from '@/shared/ui'
import { readableAuthoringProse } from '../lib/readable-prose'
import type { useAuthoring } from '../model/useAuthoring'
import type {
  StudioAIAvailability,
  StudioFlowEvent,
  StudioView,
  studioFlowMachine,
} from '../model/studio-flow-machine'

type Flow = SnapshotFrom<typeof studioFlowMachine>
type Controller = ReturnType<typeof useAuthoring>

/** Guide: making a setting by talking to the AI. The owner says what they post; the AI drafts
 *  ONE version, drawn beside the talk as the real thing (the same preview the saved one shows);
 *  every later message changes it; 저장 in the heading row keeps it. There is no wizard and nothing
 *  to drag: the order of a post is something the owner SAYS ("가격은 맨 아래로").
 *
 *  This is the `conversation` layout of `AuthoringEditor`, driven by the same flow machine and
 *  operation actor — the estimate dialog before every paid request, the publication guards, the
 *  stop and the reset are the editor's own. A message typed while the flow cannot take one yet
 *  (an existing setting, or a reviewed draft) is held, the refinement is opened, and the held
 *  message is sent once it can be. Direct text editing stays one choice away in the ⋯ menu. */
export function AuthoringConversation({
  kind,
  targetId,
  name,
  named,
  view,
  flow,
  send,
  scopeKey,
  controller,
  blocked,
  ai,
  hasFailure,
  frozenPublication,
  canSaveDraft,
  historyFull,
  workingStatus,
  preview,
  savedPreview,
  retry,
  aiAvailability,
  inputFeedback,
  inspection,
  notices,
  directEditor,
  headingRef,
  titleId,
  className,
  dialogs,
}: {
  kind: AuthoringKind
  targetId?: string
  name: string
  named: { kind: string; name: string }
  view: StudioView
  flow: Flow
  send: (event: StudioFlowEvent) => void
  scopeKey: string
  controller: Controller
  blocked: boolean
  busy: boolean
  ai: StudioAIAvailability
  hasFailure: boolean
  frozenPublication: boolean
  canSaveDraft: boolean
  historyFull: boolean
  workingStatus: string
  preview: ReactNode
  savedPreview?: ReactNode
  retry: ReactNode
  aiAvailability: ReactNode
  inputFeedback: ReactNode
  inspection: ReactNode
  notices: ReactNode
  directEditor: ReactNode
  headingRef: Ref<HTMLDivElement>
  titleId: string
  className?: string
  dialogs: ReactNode
}) {
  const { t } = useTranslation('authoring')
  const { state } = controller
  const session = state.session
  const selected = session?.selected
  const candidates = session?.candidates ?? []
  const firstCandidate = candidates.length === 1 ? candidates[0].id : ''
  const [pane, setPane] = useState<'chat' | 'draft'>('chat')
  const [held, setHeld] = useState('')
  const queued = useRef<{ text: string; typed: boolean } | null>(null)
  const operationText = flow.context.operation.text

  // ONE draft: the single candidate is taken at once — a free selection — so the talk goes on.
  useEffect(() => {
    if (view === 'choices' && !selected && firstCandidate && !blocked && !hasFailure)
      send({ type: 'CHOOSE', scopeKey, candidateId: firstCandidate })
  }, [view, selected, firstCandidate, blocked, hasFailure, send, scopeKey])

  // A held message: typed into the refinement once it is open, then sent once the flow has seen
  // the text (the guard reads the observed operation, not the field).
  useEffect(() => {
    const message = queued.current
    if (!message || view !== 'refining' || blocked) return
    if (!message.typed) {
      message.typed = true
      send({ type: 'CHAT_CHANGED', scopeKey, text: message.text })
      return
    }
    if (operationText === message.text) {
      queued.current = null
      setHeld('')
      send({ type: 'REFINE', scopeKey })
    }
  }, [view, blocked, operationText, send, scopeKey])

  const composing: 'purpose' | 'refine' | 'hold' | 'off' =
    view === 'purpose'
      ? 'purpose'
      : view === 'refining'
        ? historyFull
          ? 'off'
          : 'refine'
        : view === 'review' || view === 'existing'
          ? 'hold'
          : 'off'
  const value =
    composing === 'purpose' ? flow.context.purpose : composing === 'refine' ? state.text : held
  const change = (text: string) => {
    if (composing === 'purpose') send({ type: 'PURPOSE_CHANGED', scopeKey, text })
    else if (composing === 'refine') send({ type: 'CHAT_CHANGED', scopeKey, text })
    else setHeld(text)
  }
  const submit = (submitted: FormEvent<HTMLFormElement>) => {
    submitted.preventDefault()
    if (composing === 'purpose') send({ type: 'RECOMMEND', scopeKey })
    else if (composing === 'refine') send({ type: 'REFINE', scopeKey })
    else if (composing === 'hold' && held.trim()) {
      queued.current = { text: held, typed: false }
      send({ type: 'OPEN_REFINEMENT', scopeKey })
    }
  }
  const save = () => {
    if (view === 'refining') send({ type: 'FINISH_REFINEMENT', scopeKey })
    if (view !== 'publication') send({ type: 'OPEN_PUBLICATION', scopeKey })
    send({ type: 'PUBLISH', scopeKey })
  }
  const canSave =
    canSaveDraft &&
    !blocked &&
    !frozenPublication &&
    !hasFailure &&
    (view === 'review' || view === 'refining' || view === 'publication')

  const drafting = view === 'working' && flow.context.intent === 'recommend'
  const refining = view === 'working' && flow.context.intent === 'refine'
  const brief = flow.context.purpose.trim()
  const turns = session?.turns ?? []
  const inFlight = turns.some((turn) => turn.status !== 'done')
  const messages: ChatMessage[] = [
    {
      id: 'opener',
      sender: 'partner',
      content: targetId
        ? t('conversation.openerExisting', { name: name || named.name })
        : t(`conversation.opener.${kind}`),
    },
  ]
  if (brief && (drafting || candidates.length > 0))
    messages.push({ id: 'brief', sender: 'owner', content: brief })
  // A resumed draft that came from no suggestion (typed, or reopened) is named once, so the talk
  // says what it is about.
  if (candidates.length === 0 && selected && !targetId)
    messages.push({
      id: 'current',
      sender: 'partner',
      content: t('conversation.current', { name: selected.name || named.name }),
    })
  if (candidates.length > 0)
    messages.push({
      id: 'draft',
      sender: 'partner',
      content:
        candidates.length > 1 && !selected ? (
          <span className="block">
            {t('conversation.pick', { count: candidates.length })}
            <span className="mt-3 grid gap-2">
              {candidates.map((candidate) => (
                <ChoiceButton
                  key={candidate.id}
                  title={candidate.name}
                  description={candidate.description}
                  disabled={blocked}
                  onClick={() => send({ type: 'CHOOSE', scopeKey, candidateId: candidate.id })}
                />
              ))}
            </span>
          </span>
        ) : (
          <span className="block">
            {t('conversation.drafted', { name: (selected ?? candidates[0]).name })}
            <Button
              variant="ghost"
              className="mt-2 -ml-3 @3xl:hidden"
              onClick={() => setPane('draft')}
            >
              <Eye aria-hidden="true" className="size-4" />
              {t('conversation.showDraft')}
            </Button>
          </span>
        ),
    })
  for (const turn of turns) {
    messages.push({ id: `${turn.id}-ask`, sender: 'owner', content: turn.request })
    if (turn.status === 'done')
      messages.push({
        id: `${turn.id}-reply`,
        sender: 'partner',
        content: readableAuthoringProse(turn.reply) ? turn.reply : t('replyReady'),
      })
  }
  const asking = session?.pendingRequest || state.text
  if (refining && !inFlight && asking.trim())
    messages.push({ id: 'asking', sender: 'owner', content: asking })
  if (view === 'working')
    messages.push({
      id: 'working',
      sender: 'partner',
      content: (
        <span className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <span className="inline-flex items-center gap-2">
            <Sparkles aria-hidden="true" className="text-link-fg size-4 shrink-0" />
            {drafting
              ? t('conversation.drafting')
              : refining
                ? t('conversation.refining')
                : workingStatus}
          </span>
          {state.phase === 'active' && session?.activeJobId && (
            <Button
              variant="ghost"
              className="-my-2"
              onClick={() => send({ type: 'ASK_CANCEL', scopeKey })}
            >
              {t('cancel')}
            </Button>
          )}
        </span>
      ),
    })
  if (view === 'confirmed')
    messages.push({
      id: 'saved',
      sender: 'partner',
      content: (
        <span className="block">
          {t(
            session?.saved?.outcome === 'updated' || targetId
              ? 'confirmedUpdate'
              : 'confirmedCreate',
            {
              kind: named.kind,
              name: session?.saved?.name || name || named.name,
            },
          )}
          <Button
            variant="ghost"
            className="mt-2 -ml-3"
            onClick={() => send({ type: 'OPEN_REFINEMENT', scopeKey })}
          >
            {t('conversation.keepTalking')}
          </Button>
        </span>
      ),
    })

  const menu: ActionMenuItem[] = [
    ...(view !== 'direct' && flow.can({ type: 'OPEN_DIRECT', scopeKey })
      ? [
          {
            id: 'direct',
            label: t('conversation.direct'),
            onSelect: () => send({ type: 'OPEN_DIRECT', scopeKey }),
          },
        ]
      : []),
    ...(session &&
    (turns.length > 0 || candidates.length > 0) &&
    flow.can({ type: 'FRESH', scopeKey })
      ? [
          {
            id: 'fresh',
            label: t('conversation.fresh'),
            onSelect: () => send({ type: 'FRESH', scopeKey }),
          },
        ]
      : []),
    ...(session?.savedBaseline && flow.can({ type: 'ASK_RESET', scopeKey })
      ? [
          {
            id: 'reset',
            label: t('conversation.reset'),
            onSelect: () => send({ type: 'ASK_RESET', scopeKey }),
          },
        ]
      : []),
  ]
  const hasDraft = !!selected || view === 'direct'
  const showsDraft = hasDraft || !!savedPreview
  const statusLine =
    view === 'confirmed'
      ? t('conversation.status.saved')
      : session?.hasUnpublishedChanges || state.sourceDirty || (!targetId && hasDraft)
        ? t('conversation.status.unsaved')
        : hasDraft || savedPreview
          ? t('conversation.status.saved')
          : t('conversation.status.empty')

  const composer =
    view === 'direct' ? null : (
      <ChatComposer
        value={value}
        onChange={change}
        onSubmit={submit}
        label={t('conversation.composer')}
        placeholder={t(
          composing === 'purpose' && !targetId
            ? `conversation.placeholderFirst.${kind}`
            : `conversation.placeholderNext.${kind}`,
        )}
        sendLabel={t(composing === 'purpose' ? 'conversation.start' : 'conversation.send')}
        pending={view === 'working'}
        disabled={composing === 'off' || ai !== 'ready' || frozenPublication}
        invalid={!!flow.context.problem}
        attachment={ai !== 'ready' ? aiAvailability : undefined}
        feedback={
          historyFull ? (
            <Typography variant="body" className="text-content-secondary">
              {t('historyFull')}
            </Typography>
          ) : (
            inputFeedback
          )
        }
      />
    )

  return (
    <section
      aria-labelledby={titleId}
      className={clsx('@container flex min-w-0 flex-1 flex-col', className)}
    >
      {/* One heading row: where the draft stands, then the menu and 저장. */}
      <div ref={headingRef} className="flex min-h-11 items-center gap-2">
        <h2 id={titleId} className="sr-only">
          {t('conversation.label', named)}
        </h2>
        <Typography variant="label" as="p" role="status" className="min-w-0 flex-1 truncate">
          {statusLine}
        </Typography>
        {menu.length > 0 && (
          <ActionMenu
            label={t('conversation.more')}
            triggerLabel={<Ellipsis aria-hidden="true" className="size-5" />}
            triggerVariant="ghost"
            items={menu}
          />
        )}
        <Button variant="cta" disabled={!canSave} onClick={save}>
          {t(targetId ? 'conversation.saveChanges' : 'conversation.save')}
        </Button>
      </div>
      {notices}
      {showsDraft && (
        <SegmentedControl
          value={pane}
          onChange={setPane}
          ariaLabel={t('conversation.pane.aria')}
          options={[
            {
              value: 'chat',
              label: t(view === 'direct' ? 'conversation.pane.direct' : 'conversation.pane.chat'),
            },
            { value: 'draft', label: t('conversation.pane.draft') },
          ]}
          className="mt-3 @3xl:hidden"
        />
      )}
      <div className="mt-4 grid flex-1 gap-8 @3xl:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] @3xl:items-start @3xl:gap-10">
        <div
          onFocusCapture={() => setPane('chat')}
          className={clsx('flex min-w-0 flex-col', pane === 'draft' && 'hidden @3xl:flex')}
        >
          {view === 'direct' ? (
            directEditor
          ) : (
            <Chat
              messages={messages}
              label={t('conversation.label', named)}
              partnerLabel={t('conversation.partner')}
              ownerLabel={t('conversation.owner')}
              composer={composer}
              className="flex-1 justify-between"
            />
          )}
          {retry}
          {inspection}
        </div>
        <aside
          aria-label={t('conversation.pane.draft')}
          className={clsx(
            '@3xl:top-chrome min-w-0 @3xl:sticky',
            pane === 'chat' && 'hidden @3xl:block',
          )}
        >
          <DraftSheet
            preview={hasDraft ? preview : drafting ? null : savedPreview}
            drawing={drafting}
            revision={`${selected?.id ?? ''}:${session?.revision ?? 0}`}
          />
        </aside>
      </div>
      {dialogs}
    </section>
  )
}

/** The draft as the post it will shape — or, before there is one, the empty sheet it will be
 *  drawn on. While the first draft is being written the sheet breathes a halo; a new draft
 *  arrives once (rise), never by a light travelling round its edge. */
function DraftSheet({
  preview,
  drawing,
  revision,
}: {
  preview: ReactNode
  drawing: boolean
  revision: string
}) {
  const { t } = useTranslation('authoring')
  if (preview)
    return (
      <div key={revision} className="bg-card-bg animate-rise rounded-2xl p-5 shadow-md sm:p-6">
        {preview}
      </div>
    )
  return (
    <div className="relative">
      {drawing && (
        <span
          aria-hidden="true"
          className="bg-promo-stroke animate-promo-glow absolute -inset-2 rounded-3xl opacity-50 blur-xl"
        />
      )}
      <div
        aria-busy={drawing || undefined}
        className={clsx(
          'relative flex min-h-80 flex-col gap-4 rounded-2xl p-6',
          drawing ? 'bg-card-bg shadow-md' : 'border-content-tertiary/40 border-2 border-dashed',
        )}
      >
        <Typography variant="label" as="p">
          {t(drawing ? 'conversation.draftWorking' : 'conversation.draftEmpty')}
        </Typography>
        <div aria-hidden="true" className={clsx('flex flex-col gap-3', !drawing && 'opacity-50')}>
          {[64, 0, 92, 100, 78, 0, 88, 100, 54].map((width, index) =>
            width === 0 ? (
              <Skeleton key={index} className="mt-2 h-4 w-2/5" />
            ) : (
              <span key={index} className="block" style={{ width: `${width}%` }}>
                <Skeleton className="h-2.5" />
              </span>
            ),
          )}
        </div>
      </div>
    </div>
  )
}
