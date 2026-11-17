import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { Eye, PencilLine, Save, Sparkles } from 'lucide-react'
import type { AuthoringArtifact, AuthoringKind, AuthoringSession } from '@/entities/ai-authoring'
import {
  Badge,
  Button,
  Chat,
  ChatComposer,
  ChoiceButton,
  SegmentedControl,
  SurfaceButton,
  Typography,
  typographyStyles,
  type ChatMessage,
} from '@/shared/ui'
import { readableAuthoringProse } from '../lib/readable-prose'
import type { StudioView } from '../model/studio-flow-machine'

/** What the conversation needs from the editor: the flow's view and the session as read, the
 *  inputs' current text, and the editor's own commands. The layout owns no request: every send,
 *  choice and save goes back through the editor's flow, so the estimate dialog, the cancel and the
 *  conflict handling stay the editor's (EDIT). */
export interface AuthoringConversationProps {
  kind: AuthoringKind
  titleId: string
  name: string
  existing: boolean
  view: StudioView
  session?: AuthoringSession
  selected?: AuthoringArtifact
  purpose: string
  chat: string
  working: boolean
  workingStatus: string
  cancellable: boolean
  invalid: boolean
  composerDisabled: boolean
  historyFull: boolean
  canSave: boolean
  canDirect: boolean
  aiReady: boolean
  preview: ReactNode
  feedback: ReactNode
  failure: ReactNode
  inspection: ReactNode
  confirmedName?: string
  onPurpose: (text: string) => void
  onChat: (text: string) => void
  onSend: () => void
  onChoose: (candidateId: string) => void
  onSave: () => void
  onDirect: () => void
  onCancel: () => void
  onContinue: () => void
  onOpenExisting: () => void
}

/** Guide: making a setting by talking. The owner's words and the AI's replies run down the left as
 *  one conversation (`Chat`); the draft they are shaping is drawn on the right as the post it makes
 *  and redrawn after every reply; one composer (`ChatComposer`) at the bottom takes the next
 *  request. The heading row holds the two ways out — 직접 고치기 and 저장 — so saving is one tap
 *  from anywhere in the talk. On a phone the conversation and the preview are two panes of one
 *  switch, and a new draft offers its preview from inside the reply that announces it. */
export function AuthoringConversation(props: AuthoringConversationProps) {
  const { t } = useTranslation('authoring')
  const [pane, setPane] = useState<'chat' | 'preview'>('chat')
  const templateKind = props.kind === 'post-template' || props.kind === 'video-template'
  const starterKeys =
    props.kind === 'post-template'
      ? (['first', 'second', 'third'] as const)
      : props.kind === 'video-template'
        ? (['first', 'second'] as const)
        : []
  const messages: ChatMessage[] = []
  messages.push({
    id: 'greeting',
    sender: 'partner',
    content: (
      <div className="space-y-3">
        <p>
          {props.existing
            ? t('conversation.greeting.existing', { name: props.name })
            : templateKind
              ? t(`conversation.greeting.${props.kind as 'post-template' | 'video-template'}`)
              : t(`purposeQuestions.${props.kind}`)}
        </p>
        {props.view === 'purpose' && starterKeys.length > 0 && (
          <div className="space-y-2">
            <Typography variant="meta" as="p">
              {t('conversation.starterLabel')}
            </Typography>
            <ul className="flex flex-col gap-2">
              {starterKeys.map((key) => {
                const text = t(
                  `conversation.starters.${props.kind as 'post-template'}.${key as 'first'}`,
                )
                return (
                  <li key={key}>
                    <SurfaceButton
                      presentation="content"
                      onClick={() => props.onPurpose(text)}
                      className="border-divider text-content-primary w-full rounded-lg border px-3 py-2 whitespace-normal"
                    >
                      {text}
                    </SurfaceButton>
                  </li>
                )
              })}
            </ul>
          </div>
        )}
        {props.view === 'existing' && (
          <Button variant="secondary" size="compact" onClick={props.onOpenExisting}>
            <Sparkles aria-hidden="true" className="size-4" />
            {t('conversation.draftOpen')}
          </Button>
        )}
      </div>
    ),
  })
  if (props.purpose.trim() && props.view !== 'purpose')
    messages.push({ id: 'purpose', sender: 'owner', content: props.purpose })
  const candidates = props.session?.candidates ?? []
  if (props.view === 'choices' && candidates.length > 1)
    messages.push({
      id: 'choices',
      sender: 'partner',
      content: (
        <div className="space-y-3">
          <p>{t('conversation.choose')}</p>
          <div className="grid gap-2">
            {candidates.map((candidate) => (
              <ChoiceButton
                key={candidate.id}
                title={candidate.name}
                description={candidate.description}
                selected={props.selected?.id === candidate.id}
                disabled={props.working}
                onClick={() => props.onChoose(candidate.id)}
              />
            ))}
          </div>
        </div>
      ),
    })
  if (props.selected)
    messages.push({
      id: `draft-${props.selected.id}`,
      sender: 'partner',
      content: (
        <div className="space-y-2">
          <Typography variant="label" as="p" className="text-content-primary">
            {t('conversation.draftReady', { name: props.selected.name || props.name })}
          </Typography>
          {props.selected.description && (
            <p className="text-content-secondary">{props.selected.description}</p>
          )}
          <Button
            variant="secondary"
            size="compact"
            className="@3xl:hidden"
            onClick={() => setPane('preview')}
          >
            <Eye aria-hidden="true" className="size-4" />
            {t('conversation.draftOpen')}
          </Button>
        </div>
      ),
    })
  for (const turn of props.session?.turns ?? []) {
    messages.push({ id: `${turn.id}-request`, sender: 'owner', content: turn.request })
    if (turn.status === 'done')
      messages.push({
        id: `${turn.id}-reply`,
        sender: 'partner',
        content: readableAuthoringProse(turn.reply) ? turn.reply : t('replyReady'),
      })
  }
  if (props.working)
    messages.push({
      id: 'working',
      sender: 'partner',
      content: (
        <div className="space-y-3">
          <p className="flex items-center gap-2">
            <span aria-hidden="true" className="flex gap-1">
              {[0, 1, 2].map((dot) => (
                <span
                  key={dot}
                  className="bg-content-tertiary size-1.5 animate-pulse rounded-full"
                  style={{ animationDelay: `${dot * 180}ms` }}
                />
              ))}
            </span>
            {props.workingStatus}
          </p>
          {props.cancellable && (
            <Button variant="ghost" size="compact" onClick={props.onCancel}>
              {t('cancel')}
            </Button>
          )}
        </div>
      ),
    })
  if (props.failure) messages.push({ id: 'failure', sender: 'partner', content: props.failure })
  if (props.view === 'confirmed')
    messages.push({
      id: 'confirmed',
      sender: 'partner',
      content: (
        <div className="space-y-3">
          <p>{t('conversation.saved', { name: props.confirmedName || props.name })}</p>
          <Button variant="secondary" size="compact" onClick={props.onContinue}>
            {t('continueNamed', { kind: t(`kinds.${props.kind}`), name: props.name })}
          </Button>
        </div>
      ),
    })

  const talking = props.view === 'purpose' || props.view === 'refining'
  return (
    <div className="flex min-w-0 flex-1 flex-col">
      {/* The one heading row: what is being made, and the two ways out of the talk. */}
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
        <h2
          id={props.titleId}
          tabIndex={-1}
          className={typographyStyles({
            variant: 'title',
            className: 'min-w-0 flex-1 basis-48 break-words',
          })}
        >
          {props.selected || props.existing ? props.name : t('conversation.untitled')}
        </h2>
        <div className="ml-auto flex shrink-0 flex-wrap items-center gap-2">
          {props.canDirect && (
            <Button variant="ghost" onClick={props.onDirect}>
              <PencilLine aria-hidden="true" className="size-4" />
              {t('conversation.direct')}
            </Button>
          )}
          {props.selected && props.view !== 'confirmed' && (
            <Button variant="cta" disabled={!props.canSave} onClick={props.onSave}>
              <Save aria-hidden="true" className="size-4" />
              {t('conversation.save')}
            </Button>
          )}
        </div>
      </div>
      <SegmentedControl
        value={pane}
        onChange={setPane}
        ariaLabel={t('conversation.panes')}
        options={[
          { value: 'chat', label: t('conversation.chat') },
          { value: 'preview', label: t('previewView') },
        ]}
        className="mt-4 @3xl:hidden"
      />
      <div className="mt-4 grid flex-1 gap-8 @3xl:mt-6 @3xl:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] @3xl:items-start @3xl:gap-10">
        <div
          onFocusCapture={() => setPane('chat')}
          className={clsx(
            'flex min-h-full min-w-0 flex-col',
            pane === 'preview' && 'hidden @3xl:flex',
          )}
        >
          <Chat
            label={t('conversation.label')}
            partnerLabel={t('conversation.partner')}
            ownerLabel={t('conversation.owner')}
            messages={messages}
            className="flex-1"
            composer={
              talking || props.working ? (
                props.historyFull ? (
                  <Typography variant="body" className="bg-surface-base sticky bottom-0 pt-3">
                    {t('conversation.historyFull')}
                  </Typography>
                ) : (
                  <ChatComposer
                    value={props.view === 'purpose' ? props.purpose : props.chat}
                    onChange={props.view === 'purpose' ? props.onPurpose : props.onChat}
                    onSubmit={(submit) => {
                      submit.preventDefault()
                      props.onSend()
                    }}
                    label={t(
                      props.view === 'purpose'
                        ? 'conversation.field.purpose'
                        : 'conversation.field.refine',
                    )}
                    placeholder={
                      props.view === 'purpose'
                        ? t(`examples.${props.kind}`)
                        : t(`refineExamples.${props.kind}`)
                    }
                    sendLabel={t('conversation.send')}
                    pending={props.working}
                    disabled={props.composerDisabled || !props.aiReady}
                    invalid={props.invalid}
                    feedback={props.feedback}
                  />
                )
              ) : undefined
            }
          />
          {props.inspection}
        </div>
        <aside
          aria-label={t('previewView')}
          className={clsx(
            'min-w-0 @3xl:sticky @3xl:top-[calc(var(--spacing-chrome,4rem)+1rem)]',
            pane === 'chat' && 'hidden @3xl:block',
          )}
        >
          <div
            className={clsx(
              'bg-surface-raised ring-divider relative rounded-3xl p-5 shadow-lg ring-1 sm:p-6',
              props.working && 'opacity-70 transition-opacity',
            )}
          >
            {props.working && (
              <Badge tone="info" className="absolute top-4 right-4">
                {t('conversation.drawing')}
              </Badge>
            )}
            {props.preview ?? <EmptyPreview />}
          </div>
        </aside>
      </div>
    </div>
  )
}

/** The page before there is a draft: the outline of a post with nothing in it yet. */
function EmptyPreview() {
  const { t } = useTranslation('authoring')
  return (
    <div className="flex flex-col gap-4 py-2">
      <span
        aria-hidden="true"
        className="border-divider block h-6 w-2/3 rounded-lg border-2 border-dashed"
      />
      {[85, 70, 90].map((width, index) => (
        <span key={index} aria-hidden="true" className="flex flex-col gap-2">
          <span className="border-divider block h-4 w-1/3 rounded-md border-2 border-dashed" />
          <span
            className="border-divider block h-16 rounded-lg border-2 border-dashed"
            style={{ width: `${width}%` }}
          />
        </span>
      ))}
      <Typography variant="body" className="text-content-secondary text-center">
        {t('conversation.emptyPreview')}
      </Typography>
    </div>
  )
}
