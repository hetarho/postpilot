import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  promptText,
  useStartVoiceExperience,
  useVoiceExperiences,
  useVoicePrompts,
  useVoiceSample,
  VoiceReadinessMeter,
  type VoiceProfile,
  type VoiceSample,
} from '@/entities/voice'
import {
  Badge,
  Button,
  Chat,
  Disclosure,
  FieldLabel,
  FieldMessage,
  Listbox,
  LoadingState,
  GuideProgress,
  Typography,
  type ChatMessage,
} from '@/shared/ui'
import {
  answerTurnKey,
  parseTurnKey,
  questionnaireCanSkip,
  questionnaireCurrentKey,
  questionnaireHasSituations,
  questionnaireSavedCount,
  QUESTIONNAIRE_BATCH_SIZE,
  type QuestionnaireEvent,
} from '../model/questionnaire-machine'
import { useQuestionnaire } from '../model/useQuestionnaire'
import { ExperiencePicker } from './ExperiencePicker'
import { VoiceAnswerComposer } from './VoiceAnswerComposer'
import { CHAT_REACTION_KEYS } from '../config'

export interface VoiceLearningChatProps {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  profile: VoiceProfile
  renderMakeVoice: (close: () => void) => ReactNode
  onClose?: () => void
  onBusyChange?: (busy: boolean) => void
  onReview?: () => void
  active?: boolean
  onNavigationChange?: (
    navigation: { canGoBack: boolean; goBack: () => void; startNewBatch: () => void } | undefined,
  ) => void
}

/** A saved answer's identity: its question, and for a thread answer its experience (VOICE-65). */
function sampleTurnKey(sample: VoiceSample) {
  return answerTurnKey(sample.experienceId, sample.promptKey)
}

/** One mounted form for one scene; saved keys, rather than catalog size, define completion. */
export function VoiceLearningChat(props: VoiceLearningChatProps) {
  return <AccountLearningChat key={`${props.ownerId}:${props.voiceId}`} {...props} />
}

function AccountLearningChat({
  ownerId,
  voiceId,
  samples,
  profile,
  renderMakeVoice,
  onClose,
  onBusyChange,
  onReview,
  active = true,
  onNavigationChange,
}: VoiceLearningChatProps) {
  const { t } = useTranslation('voices')
  const titleId = useId()
  const heading = useRef<HTMLElement>(null)
  const query = useVoicePrompts()
  const experiencesQuery = useVoiceExperiences(ownerId, voiceId)
  const startExperience = useStartVoiceExperience(ownerId, voiceId)
  const { state, send: actorSend, getSnapshot } = useQuestionnaire(ownerId, voiceId)
  const [moreOpen, setMoreOpen] = useState(false)
  const [savedMessage, setSavedMessage] = useState(false)
  const [confirmedReplies, setConfirmedReplies] = useState<Record<string, string>>({})
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const send = useCallback(
    (event: QuestionnaireEvent) => {
      const before = getSnapshot()
      if (!mounted.current) return before
      const next = actorSend(event)
      if (next !== before) onBusyChange?.(next.phase === 'saving' || next.phase === 'starting')
      return next
    },
    [getSnapshot, actorSend, onBusyChange],
  )
  useEffect(() => {
    if (
      query.isPending ||
      query.isError ||
      experiencesQuery.isPending ||
      experiencesQuery.isError ||
      getSnapshot().phase !== 'loading'
    )
      return
    send({
      ownerId,
      voiceId,
      type: 'hydrate',
      catalog: query.prompts,
      saved: samples.filter((sample) => sample.kind === 'answer').map(sampleTurnKey),
      experiences: experiencesQuery.experiences.map(({ id, kind }) => ({ id, kind })),
      made: profile.made,
      answeredQuestions: profile.readiness.answeredQuestions,
      missingParts: profile.readiness.missingParts,
    })
  }, [
    query.prompts,
    query.isPending,
    query.isError,
    experiencesQuery.experiences,
    experiencesQuery.isPending,
    experiencesQuery.isError,
    samples,
    profile.made,
    profile.readiness.answeredQuestions,
    profile.readiness.missingParts,
    ownerId,
    voiceId,
    send,
    getSnapshot,
  ])
  const key = questionnaireCurrentKey(state)
  const prompt = query.prompts.find((question) => question.key === parseTurnKey(key).promptKey)
  /** A turn's question text, with its experience named when it is a thread question. */
  const turnText = (turnKey: string) => {
    const { experienceId, promptKey } = parseTurnKey(turnKey)
    const question = query.prompts.find((candidate) => candidate.key === promptKey)
    if (!question) return ''
    return promptText(
      question,
      query.kinds,
      experiencesQuery.experiences.find((experience) => experience.id === experienceId),
    )
  }
  const previousKey = useRef('')
  useEffect(() => {
    if (active && state.phase !== 'loading' && key !== previousKey.current) {
      heading.current?.querySelector('h2')?.focus({ preventScroll: true })
      previousKey.current = key
    }
  }, [key, state.phase, active])
  const identity = { ownerId, voiceId }
  const count = questionnaireSavedCount(state)
  const busy = state.phase === 'saving' || state.phase === 'starting'
  useEffect(() => {
    onNavigationChange?.(
      active
        ? {
            canGoBack: !busy && (state.cursor > 0 || state.reviewing),
            goBack: () => send({ ownerId, voiceId, type: 'back' }),
            startNewBatch: () => send({ ownerId, voiceId, type: 'new-session' }),
          }
        : undefined,
    )
    return () => onNavigationChange?.(undefined)
  }, [onNavigationChange, active, busy, state.cursor, state.reviewing, ownerId, voiceId, send])

  const submission = useRef<{ key: string; operation: number } | null>(null)
  const begin = () => {
    if (!active) return false
    const previous = getSnapshot()
    const next = send({ ...identity, type: 'begin', key })
    if (next === previous || next.phase !== 'saving') return false
    submission.current = { key, operation: next.operation }
    setSavedMessage(false)
    return true
  }
  const complete = (success: boolean, body?: string) => {
    const pending = submission.current
    if (!pending || !mounted.current) return
    const previous = getSnapshot()
    const next = send({ ...identity, ...pending, type: success ? 'success' : 'failure' })
    submission.current = null
    if (success && next !== previous) {
      setSavedMessage(true)
      if (body !== undefined)
        setConfirmedReplies((replies) => ({ ...replies, [pending.key]: body }))
    }
  }
  const close = () => {
    if (!busy) onClose?.()
  }
  const changeBody = (body: string) => {
    if (!active) return
    send({ ...identity, type: 'draft', key, body })
    setSavedMessage(false)
  }
  const answeredSample = samples.find(
    (sample) => sample.kind === 'answer' && sampleTurnKey(sample) === key,
  )
  const beginExperience = async (
    kind: Parameters<typeof startExperience.start>[0],
    name: string,
  ) => {
    if (!active) return
    const previous = getSnapshot()
    const next = send({ ...identity, type: 'start-experience' })
    if (next === previous || next.phase !== 'starting') return
    try {
      const experience = await startExperience.start(kind, name)
      send({
        ...identity,
        type: 'experience-started',
        experience: { id: experience.id, kind: experience.kind },
        operation: next.operation,
      })
    } catch {
      send({ ...identity, type: 'experience-failed', operation: next.operation })
    }
  }
  const formProps = {
    ownerId,
    voiceId,
    prompt: prompt!,
    experienceId: parseTurnKey(key).experienceId || undefined,
    bodyValue: state.drafts[key],
    onBodyChange: changeBody,
    onEdit: () => setSavedMessage(false),
    disabled: busy,
    onSubmitStart: begin,
    onFailure: () => complete(false),
    onDone: (body: string) => complete(true, body),
    onBack: () => send({ ...identity, type: 'back' }),
    backLabel: t('prompts.previous'),
    hideBack: !!onNavigationChange,
    backDisabled: state.cursor === 0 && !state.reviewing,
    secondaryActions: !state.saved.includes(key) &&
      (!parseTurnKey(key).experienceId || questionnaireCanSkip(state)) && (
        <Button
          variant="ghost"
          className="min-h-11 min-w-11"
          disabled={busy}
          onClick={() => {
            send({ ...identity, type: 'skip' })
            setSavedMessage(false)
          }}
        >
          {t('prompts.skip')}
        </Button>
      ),
  }
  const messages: ChatMessage[] = [
    {
      id: 'welcome',
      sender: 'partner',
      content: t('prompts.chat.welcome'),
    },
  ]
  const answering =
    state.phase === 'answering' || state.phase === 'saving' || state.phase === 'failed'
  const visibleKeys = answering ? state.history.slice(0, state.cursor + 1) : state.history
  const askedExperiences = new Set<string>()
  const askExperience = (experienceId: string, id: string) => {
    const first = state.experiences.findIndex((experience) => experience.id === experienceId) <= 0
    messages.push({
      id: `${id}-ask`,
      sender: 'partner',
      content: t(first ? 'prompts.chat.experience.ask' : 'prompts.chat.experience.askAnother'),
    })
  }
  for (const turnKey of visibleKeys) {
    const { experienceId, promptKey } = parseTurnKey(turnKey)
    const questionIndex = query.prompts.findIndex((question) => question.key === promptKey)
    const question = query.prompts[questionIndex]
    if (!question) continue
    if (experienceId && !askedExperiences.has(experienceId)) {
      askedExperiences.add(experienceId)
      const experience = experiencesQuery.experiences.find(
        (candidate) => candidate.id === experienceId,
      )
      askExperience(experienceId, experienceId)
      messages.push({
        id: `${experienceId}-pick`,
        sender: 'owner',
        content:
          experience?.name.trim() ||
          query.kinds.find((info) => info.kind === (experience?.kind ?? question.experienceKind))
            ?.label ||
          '',
      })
    }
    messages.push({
      id: `${turnKey}-situation`,
      sender: 'partner',
      content: (
        <div className="space-y-2">
          {question.scene && <p>{question.scene}</p>}
          <p>{turnText(turnKey)}</p>
          {question.hint && <p className="text-content-secondary">{question.hint}</p>}
          {state.phase !== 'complete' && turnKey === key && question.examples.length > 0 && (
            <Disclosure
              key={turnKey}
              size="row"
              headingLevel={3}
              title={t('prompts.chat.examples.title')}
            >
              <ul className="mt-2 space-y-2">
                {question.examples.map((example) => (
                  <li key={example.register} className="bg-card-well rounded-md px-3 py-2">
                    <Badge tone="neutral">
                      {t(`prompts.chat.examples.register.${example.register}`)}
                    </Badge>
                    <p className="mt-1">{example.text}</p>
                  </li>
                ))}
              </ul>
            </Disclosure>
          )}
        </div>
      ),
    })
    if (!state.saved.includes(turnKey)) continue
    const sample = samples.find(
      (sample) => sample.kind === 'answer' && sampleTurnKey(sample) === turnKey,
    )
    messages.push({
      id: `${turnKey}-answer`,
      sender: 'owner',
      content: (
        <>
          {confirmedReplies[turnKey] !== undefined ? (
            confirmedReplies[turnKey]
          ) : (
            <SavedReply ownerId={ownerId} voiceId={voiceId} sampleId={sample?.id ?? ''} />
          )}
          {question.photo && (
            <p className="text-content-secondary mt-2">{t('prompts.chat.photoSaved')}</p>
          )}
        </>
      ),
    })
    const reaction = CHAT_REACTION_KEYS[questionIndex % CHAT_REACTION_KEYS.length]
    messages.push({
      id: `${turnKey}-reaction`,
      sender: 'partner',
      content: t(`prompts.chat.reactions.${reaction}`),
    })
  }
  if (state.phase === 'choosing' || state.phase === 'starting')
    messages.push({
      id: `ask-${state.experiences.length}`,
      sender: 'partner',
      content: t(
        state.experiences.length === 0
          ? 'prompts.chat.experience.ask'
          : 'prompts.chat.experience.askAnother',
      ),
    })
  if (state.phase === 'offer')
    messages.push({
      id: `offer-${state.history.length}`,
      sender: 'partner',
      content: t('prompts.chat.experience.offerLead'),
    })
  return (
    <section ref={heading} aria-labelledby={titleId} className="min-w-0">
      <Typography
        variant="fieldTitle"
        as="h2"
        id={titleId}
        tabIndex={-1}
        className={prompt && state.phase !== 'complete' ? 'sr-only md:not-sr-only' : undefined}
      >
        {state.reviewing ? t('prompts.reviewing') : t('prompts.sessionTitle')}
      </Typography>
      <div className="flex flex-wrap items-center justify-between gap-2 md:mt-4 md:gap-3">
        <Typography variant="label">
          {t('prompts.count', { current: count, total: QUESTIONNAIRE_BATCH_SIZE })}
        </Typography>
        <Typography variant="meta" role="status">
          {savedMessage ? t('prompts.saved') : ''}
        </Typography>
      </div>
      {/* One segment per answer the chat asks for, drawn like every other sequence in the app;
          the one being answered now is the current segment. */}
      <GuideProgress
        steps={Array.from({ length: QUESTIONNAIRE_BATCH_SIZE }, (_, at) => ({ value: String(at) }))}
        current={String(Math.min(count, QUESTIONNAIRE_BATCH_SIZE - 1))}
        ariaLabel={t('prompts.progress')}
        className="mt-2 md:mt-3"
      />
      {state.phase === 'complete' && !profile.made && profile.readiness.percent < 100 && (
        <div className="mt-6">
          <VoiceReadinessMeter readiness={profile.readiness} />
        </div>
      )}
      {query.isError ? (
        <div className="mt-6">
          <FieldMessage>{t('prompts.loadFailed')}</FieldMessage>
          <Button variant="secondary" className="mt-3" onClick={query.refetch}>
            {t('prompts.retry')}
          </Button>
        </div>
      ) : state.phase === 'loading' ? (
        <LoadingState variant="detail" rows={2} label={t('prompts.loading')} className="mt-6" />
      ) : (
        <>
          <Chat
            label={t('prompts.chat.label')}
            partnerLabel={t('prompts.chat.partner')}
            ownerLabel={t('prompts.chat.owner')}
            messages={messages}
            className="mt-4"
            composer={
              state.phase === 'choosing' || state.phase === 'starting' ? (
                <ExperiencePicker
                  kinds={query.kinds}
                  pending={state.phase === 'starting'}
                  errorMessage={
                    startExperience.isError
                      ? startExperience.errorMessage || t('prompts.chat.experience.startFailed')
                      : undefined
                  }
                  onStart={(kind, name) => void beginExperience(kind, name)}
                  onNothing={
                    state.experiences.length === 0 && state.history.length === 0
                      ? () => send({ ...identity, type: 'situations' })
                      : undefined
                  }
                />
              ) : state.phase === 'offer' ? (
                <div className="bg-surface-base pb-safe-b sticky bottom-0 z-10 flex flex-wrap justify-end gap-2 pt-3">
                  {questionnaireHasSituations(state) && (
                    <Button
                      variant="secondary"
                      onClick={() => send({ ...identity, type: 'situations' })}
                    >
                      {t('prompts.chat.experience.situations')}
                    </Button>
                  )}
                  {state.saved.length > 0 && (
                    <Button
                      variant="ghost"
                      onClick={() => send({ ...identity, type: 'review-saved' })}
                    >
                      {t('prompts.chat.experience.review')}
                    </Button>
                  )}
                  <Button
                    variant="cta"
                    onClick={() => send({ ...identity, type: 'another-experience' })}
                  >
                    {t('prompts.chat.experience.another')}
                  </Button>
                </div>
              ) : state.phase !== 'complete' && prompt ? (
                <QuestionAnswerComposer
                  key={key}
                  {...formProps}
                  sampleId={answeredSample?.id ?? ''}
                  saved={state.saved.includes(key)}
                />
              ) : undefined
            }
          />
          {state.phase === 'complete' && (
            <div className="mt-6 space-y-4">
              <Typography variant="title" as="h3">
                {t(
                  state.completion === 'ten'
                    ? 'prompts.batchComplete'
                    : state.completion === 'saved'
                      ? 'prompts.savedTitle'
                      : 'prompts.complete',
                )}
              </Typography>
              <Typography variant="body" className="text-content-secondary">
                {t(state.completion === 'ten' ? 'prompts.batchHelp' : 'prompts.exhaustedHelp')}
              </Typography>
              {onReview ? (
                <Button variant="cta" onClick={onReview}>
                  {t('prompts.reviewNext')}
                </Button>
              ) : (
                profile.readiness.percent >= 100 && <div>{renderMakeVoice(close)}</div>
              )}
              <Button
                variant="ghost"
                aria-expanded={moreOpen}
                onClick={() => setMoreOpen(!moreOpen)}
              >
                {t('prompts.moreOptions')}
              </Button>
              {moreOpen && (
                <Button
                  variant="secondary"
                  onClick={() => {
                    send({ ...identity, type: 'new-session' })
                    setSavedMessage(false)
                  }}
                >
                  {t('prompts.anotherTen')}
                </Button>
              )}
              {moreOpen && state.saved.length > 0 && (
                <div>
                  <FieldLabel id={`${titleId}-review-label`} htmlFor={`${titleId}-review`}>
                    {t('prompts.review')}
                  </FieldLabel>
                  <Listbox
                    id={`${titleId}-review`}
                    aria-labelledby={`${titleId}-review-label`}
                    className="mt-2"
                    value=""
                    options={[
                      { value: '', label: t('prompts.chooseReview') },
                      ...state.saved
                        .filter((turnKey) => turnText(turnKey) !== '')
                        .map((turnKey) => ({ value: turnKey, label: turnText(turnKey) })),
                    ]}
                    onChange={(reviewKey) => {
                      if (reviewKey) send({ ...identity, type: 'review', key: reviewKey })
                      setSavedMessage(false)
                    }}
                  />
                </div>
              )}
            </div>
          )}
        </>
      )}
      {onClose && (
        <Button variant="ghost" disabled={busy} className="mt-6" onClick={close}>
          {t('prompts.close')}
        </Button>
      )}
    </section>
  )
}

function QuestionAnswerComposer({
  sampleId,
  saved,
  ...props
}: Parameters<typeof VoiceAnswerComposer>[0] & { sampleId: string; saved: boolean }) {
  const { t } = useTranslation('voices')
  const { detail, isError, refetch } = useVoiceSample(props.ownerId, props.voiceId, sampleId)
  const needsSavedAnswer = sampleId !== '' && (props.bodyValue === undefined || props.prompt.photo)
  if (needsSavedAnswer && isError && !detail)
    return (
      <div className="mt-4">
        <FieldMessage>{t('prompts.answerLoadFailed')}</FieldMessage>
        <Button variant="secondary" onClick={refetch}>
          {t('prompts.retry')}
        </Button>
      </div>
    )
  if (needsSavedAnswer && !detail)
    return <LoadingState variant="form" rows={1} label={t('prompts.loading')} className="mt-4" />
  return (
    <VoiceAnswerComposer
      {...props}
      initialBody={saved ? (props.bodyValue ?? detail?.body ?? '') : ''}
      currentPhoto={
        detail?.photoUrl
          ? { url: detail.photoUrl, width: detail.photoWidth, height: detail.photoHeight }
          : undefined
      }
    />
  )
}

function SavedReply({
  ownerId,
  voiceId,
  sampleId,
}: {
  ownerId: string
  voiceId: string
  sampleId: string
}) {
  const { t } = useTranslation('voices')
  const { detail, isError, refetch } = useVoiceSample(ownerId, voiceId, sampleId)
  if (detail) return <>{detail.body}</>
  if (isError)
    return (
      <Button variant="ghost" onClick={refetch}>
        {t('prompts.retry')}
      </Button>
    )
  return <LoadingState variant="inline" label={t('prompts.loading')} />
}
