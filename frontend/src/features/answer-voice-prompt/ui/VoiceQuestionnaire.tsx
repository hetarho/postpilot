import { useCallback, useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import {
  useVoicePrompts,
  useVoiceSample,
  VoiceReadinessMeter,
  type VoiceProfile,
  type VoiceSample,
} from '@/entities/voice'
import { Button, FieldLabel, FieldMessage, Listbox, ProgressBar, Typography } from '@/shared/ui'
import {
  initialQuestionnaireState,
  questionnaireCurrentKey,
  questionnaireSavedCount,
  questionnaireTransition,
  QUESTIONNAIRE_BATCH_SIZE,
  type QuestionnaireEvent,
} from '../model/questionnaire-machine'
import { AnswerForm } from './AnswerForm'

export interface VoiceQuestionnaireProps {
  ownerId: string
  voiceId: string
  samples: readonly VoiceSample[]
  profile: VoiceProfile
  renderMakeVoice: (close: () => void) => ReactNode
  onClose?: () => void
  onBusyChange?: (busy: boolean) => void
}

/** One mounted form for one scene; saved keys, rather than catalog size, define completion. */
export function VoiceQuestionnaire(props: VoiceQuestionnaireProps) {
  return <AccountQuestionnaire key={`${props.ownerId}:${props.voiceId}`} {...props} />
}

function AccountQuestionnaire({
  ownerId,
  voiceId,
  samples,
  profile,
  renderMakeVoice,
  onClose,
  onBusyChange,
}: VoiceQuestionnaireProps) {
  const { t } = useTranslation('voices')
  const titleId = useId()
  const heading = useRef<HTMLElement>(null)
  const query = useVoicePrompts()
  const [state, setState] = useState(() => initialQuestionnaireState(ownerId, voiceId))
  const stateRef = useRef(state)
  const [savedMessage, setSavedMessage] = useState(false)
  const mounted = useRef(true)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  // Keep the ref synchronous: two presses in the same browser frame still reserve one save.
  const send = useCallback(
    (event: QuestionnaireEvent) => {
      const current = stateRef.current
      if (!mounted.current) return current
      const next = questionnaireTransition(current, event)
      stateRef.current = next
      if (next !== current) {
        onBusyChange?.(next.phase === 'saving')
        setState(next)
      }
      return next
    },
    [onBusyChange],
  )
  useEffect(() => {
    if (query.isPending || query.isError || stateRef.current.phase !== 'loading') return
    send({
      ownerId,
      voiceId,
      type: 'hydrate',
      catalog: query.prompts,
      saved: samples.filter((sample) => sample.kind === 'answer').map((sample) => sample.promptKey),
      made: profile.made,
      answeredQuestions: profile.readiness.answeredQuestions,
      missingParts: profile.readiness.missingParts,
    })
  }, [
    query.prompts,
    query.isPending,
    query.isError,
    samples,
    profile.made,
    profile.readiness.answeredQuestions,
    profile.readiness.missingParts,
    ownerId,
    voiceId,
    send,
  ])
  const key = questionnaireCurrentKey(state)
  const prompt = query.prompts.find((question) => question.key === key)
  const previousKey = useRef('')
  useEffect(() => {
    if (state.phase !== 'loading' && key !== previousKey.current) {
      heading.current?.querySelector('h2')?.focus({ preventScroll: true })
      previousKey.current = key
    }
  }, [key, state.phase])
  const identity = { ownerId, voiceId }
  const count = questionnaireSavedCount(state)
  const busy = state.phase === 'saving'
  const submission = useRef<{ key: string; operation: number } | null>(null)
  const begin = () => {
    const previous = stateRef.current
    const next = send({ ...identity, type: 'begin', key })
    if (next === previous || next.phase !== 'saving') return false
    submission.current = { key, operation: next.operation }
    setSavedMessage(false)
    return true
  }
  const complete = (success: boolean) => {
    const pending = submission.current
    if (!pending || !mounted.current) return
    const previous = stateRef.current
    const next = send({ ...identity, ...pending, type: success ? 'success' : 'failure' })
    submission.current = null
    if (success && next !== previous) setSavedMessage(true)
  }
  const close = () => {
    if (!busy) onClose?.()
  }
  const changeBody = (body: string) => {
    send({ ...identity, type: 'draft', key, body })
    setSavedMessage(false)
  }
  const canContinue = state.catalog.some((question) => !state.saved.includes(question.key))
  const answeredSample = samples.find(
    (sample) => sample.kind === 'answer' && sample.promptKey === key,
  )
  const formProps = {
    ownerId,
    voiceId,
    prompt: prompt!,
    bodyValue: state.drafts[key],
    onBodyChange: changeBody,
    onEdit: () => setSavedMessage(false),
    disabled: busy,
    onSubmitStart: begin,
    onFailure: () => complete(false),
    onDone: () => complete(true),
    onBack: () => send({ ...identity, type: 'back' }),
    backLabel: t('prompts.previous'),
    backDisabled: state.cursor === 0 && !state.reviewing,
    secondaryActions: !state.saved.includes(key) && (
      <Button
        variant="ghost"
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
  return (
    <section ref={heading} aria-labelledby={titleId} className="min-w-0">
      <Typography variant="fieldTitle" as="h2" id={titleId} tabIndex={-1}>
        {state.reviewing ? t('prompts.reviewing') : t('prompts.sessionTitle')}
      </Typography>
      {!prompt?.hint && state.phase !== 'complete' && (
        <Typography variant="body" className="text-content-secondary mt-2">
          {t('prompts.sessionHelp')}
        </Typography>
      )}
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <Typography variant="label">
          {t('prompts.count', { current: count, total: QUESTIONNAIRE_BATCH_SIZE })}
        </Typography>
        <Typography variant="meta" role="status">
          {savedMessage ? t('prompts.saved') : ''}
        </Typography>
      </div>
      <ProgressBar
        label={t('prompts.progress')}
        done={count}
        total={QUESTIONNAIRE_BATCH_SIZE}
        className="mt-3"
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
        <Typography variant="body" role="status" className="mt-6">
          {t('prompts.loading')}
        </Typography>
      ) : state.phase === 'complete' ? (
        <div className="mt-6 space-y-4">
          <Typography variant="title" as="h3">
            {t(state.completion === 'ten' ? 'prompts.batchComplete' : 'prompts.complete')}
          </Typography>
          <Typography variant="body" className="text-content-secondary">
            {t(state.completion === 'ten' ? 'prompts.batchHelp' : 'prompts.exhaustedHelp')}
          </Typography>
          {profile.readiness.percent >= 100 && <div>{renderMakeVoice(close)}</div>}
          {canContinue && (
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
          {state.saved.length > 0 && (
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
                  ...query.prompts
                    .filter((question) => state.saved.includes(question.key))
                    .map((question) => ({ value: question.key, label: question.text })),
                ]}
                onChange={(reviewKey) => {
                  if (reviewKey) send({ ...identity, type: 'review', key: reviewKey })
                  setSavedMessage(false)
                }}
              />
            </div>
          )}
        </div>
      ) : prompt ? (
        <QuestionAnswerForm
          key={key}
          {...formProps}
          sampleId={answeredSample?.id ?? ''}
          saved={state.saved.includes(key)}
        />
      ) : null}
      {onClose && (
        <Button variant="ghost" disabled={busy} className="mt-6" onClick={close}>
          {t('prompts.close')}
        </Button>
      )}
    </section>
  )
}

function QuestionAnswerForm({
  sampleId,
  saved,
  ...props
}: Parameters<typeof AnswerForm>[0] & { sampleId: string; saved: boolean }) {
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
    return (
      <Typography variant="body" role="status" className="mt-4">
        {t('prompts.loading')}
      </Typography>
    )
  return (
    <AnswerForm
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
