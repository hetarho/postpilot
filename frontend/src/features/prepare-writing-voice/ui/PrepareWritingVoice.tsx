import { useCallback, useEffect, useId, useMemo, useRef, useState, type ReactNode } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { FileText, MessageCircle, Sparkles } from 'lucide-react'
import {
  useCreateVoice,
  useAnalyzeVoice,
  useSetDefaultVoice,
  useVoiceProfile,
  useVoiceAnalysisQueryKey,
  toVoice,
  VOICE_NAME_MAX_CHARS,
  VoiceReadinessMeter,
  type Voice,
  type VoiceProfile,
} from '@/entities/voice'
import { useJob, isTerminal } from '@/entities/generation-job'
import { useInitializeDefaultSelections, useStageSelection } from '@/entities/model-catalog'
import {
  AppFailureMessage,
  Button,
  ChoiceButton,
  Notice,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import {
  learningMachine,
  type LearningMethod,
  type LearningEvent,
  type LearningServices,
} from '../model/learning-machine'

export interface VoiceLearningNavigation {
  back: () => boolean
  busy: boolean
}
export interface LearningSlotProps {
  ownerId: string
  voiceId: string
  profile: VoiceProfile
  active: boolean
  onBusyChange: (busy: boolean) => void
  onReview: () => void
  onConfirmed: (voiceId: string) => void
  pasteDraft: { label: string; body: string }
  onPasteDraft: (draft: { label: string; body: string }) => void
  onNavigationChange: (
    navigation: { canGoBack: boolean; goBack: () => void; startNewBatch?: () => void } | undefined,
  ) => void
}
export interface PrepareWritingVoiceProps {
  ownerId: string
  initialVoiceId?: string
  initialMethod?: LearningMethod
  voices?: readonly Voice[]
  renderQuestions: (props: LearningSlotProps) => ReactNode
  renderPaste: (props: LearningSlotProps) => ReactNode
  renderAI?: (
    props: Pick<
      LearningSlotProps,
      'ownerId' | 'active' | 'onBusyChange' | 'onConfirmed' | 'onNavigationChange'
    >,
  ) => ReactNode
  renderLegacy?: PrepareWritingVoiceProps['renderAI']
  renderMaterials?: (props: LearningSlotProps) => ReactNode
  onBusyChange?: (busy: boolean) => void
  onComplete?: (voice: Voice) => void
  onNavigationChange?: (navigation: VoiceLearningNavigation | null) => void
}
export function PrepareWritingVoice(props: PrepareWritingVoiceProps) {
  return (
    <ScopedPreparation
      key={`${props.ownerId}:${props.initialVoiceId ?? ''}:${props.initialMethod ?? 'choose'}`}
      {...props}
    />
  )
}
function ScopedPreparation(props: PrepareWritingVoiceProps) {
  const { t } = useTranslation('voicePreparation')
  const ownerId = props.ownerId
  const create = useCreateVoice(ownerId)
  const initialAPI = useRef<LearningServices>({
    create: async (name: string) => {
      const result = await create.create({ name })
      if (!result.voice) throw new Error('Unconfirmed writing voice')
      return toVoice(result.voice)
    },
    analyze: async (): Promise<string> => {
      throw new Error('Analysis not ready')
    },
    read: async (): Promise<VoiceProfile> => {
      throw new Error('Profile not ready')
    },
    confirm: async (): Promise<Voice> => {
      throw new Error('Default not ready')
    },
  })
  const names = new Set(props.voices?.map((voice) => voice.name) ?? [])
  const baseName = t('autoName')
  let name = baseName,
    suffix = 2
  while (names.has(name)) {
    const tail = ` ${suffix++}`
    name =
      Array.from(baseName)
        .slice(0, VOICE_NAME_MAX_CHARS - tail.length)
        .join('') + tail
  }
  const actor = useActorRef(learningMachine, {
    input: {
      ownerId,
      initialVoiceId: props.initialVoiceId,
      initialMethod: props.initialMethod,
      name,
      runtime: initialAPI,
    },
  })
  const snapshot = useSelector(actor, (state) => state)
  const context = snapshot.context
  const query = useVoiceProfile(ownerId, context.voiceId)
  const analyze = useAnalyzeVoice(ownerId, context.voiceId)
  const defaults = useSetDefaultVoice(ownerId)
  const refreshProfile = query.refresh
  const modelDefaults = useInitializeDefaultSelections(ownerId)
  const model = useStageSelection('analyze')
  useEffect(() => {
    const voiceId = context.voiceId
    initialAPI.current = {
      create: async (name) => {
        const result = await create.create({ name })
        if (!result.voice) throw new Error('Unconfirmed writing voice')
        return toVoice(result.voice)
      },
      analyze: async (requested, selected) => {
        if (requested !== voiceId) throw new Error('Obsolete voice analysis')
        return (await analyze.analyze(selected)).jobId
      },
      read: async (requested) => {
        if (requested !== voiceId) throw new Error('Obsolete voice read')
        return refreshProfile()
      },
      confirm: async (requested) => {
        if (requested !== voiceId) throw new Error('Obsolete voice selection')
        const result = await defaults.setDefault(requested)
        const found = result.voices.find(
          (voice) => voice.id === requested && voice.made && voice.isDefault && !voice.deleted,
        )
        if (!found) throw new Error('Unconfirmed default writing voice')
        return toVoice(found)
      },
    }
  }, [create, analyze, defaults, refreshProfile, context.voiceId])
  const send = useCallback(
    (event: Omit<LearningEvent, 'ownerId'>) => actor.send({ ...event, ownerId } as LearningEvent),
    [actor, ownerId],
  )
  const profileKey = useVoiceAnalysisQueryKey(ownerId, context.voiceId)
  const invalidation = useMemo(() => [profileKey], [profileKey])
  const job = useJob(context.jobId || query.profile?.activeJobId || '', invalidation)
  useEffect(() => {
    if (query.profile)
      send({ type: 'PROFILE', profile: query.profile } as Omit<LearningEvent, 'ownerId'>)
    else if (query.isError)
      send({ type: 'PROFILE_FAILED', failure: { reason: 'UNKNOWN_FAILURE', params: {} } } as Omit<
        LearningEvent,
        'ownerId'
      >)
  }, [query.profile, query.isError, send])
  const terminalRead = useRef('')
  const watching = snapshot.matches({ personal: { analyzing: 'watching' } })
  useEffect(() => {
    if (!watching || !job.job || !isTerminal(job.job)) return
    if (job.job.status === 'done') {
      if (terminalRead.current === job.job.id) return
      terminalRead.current = job.job.id
      send({ type: 'JOB_COMPLETED', jobId: job.job.id } as Omit<LearningEvent, 'ownerId'>)
    } else
      send({ type: 'JOB_FAILED', failure: job.job.failure, jobId: job.job.id } as Omit<
        LearningEvent,
        'ownerId'
      >)
  }, [job.job, watching, send])
  const busy = snapshot.hasTag('busy') || context.childBusy
  const busyCallback = useRef(props.onBusyChange)
  useEffect(() => {
    busyCallback.current = props.onBusyChange
  }, [props.onBusyChange])
  useEffect(() => {
    busyCallback.current?.(busy)
  }, [busy])
  useEffect(() => () => busyCallback.current?.(false), [])
  const completeCallback = props.onComplete
  const navigationCallback = props.onNavigationChange
  const notified = useRef(false)
  useEffect(() => {
    if (snapshot.matches('done') && context.confirmed && !notified.current) {
      notified.current = true
      completeCallback?.(context.confirmed)
    }
  }, [snapshot, context.confirmed, completeCallback])
  const [questionNavigation, setQuestionNavigation] = useState<
    { canGoBack: boolean; goBack: () => void; startNewBatch?: () => void } | undefined
  >(undefined)
  const [aiNavigation, setAINavigation] = useState<
    { canGoBack: boolean; goBack: () => void } | undefined
  >(undefined)
  const recordQuestionNavigation = useCallback(
    (
      navigation:
        { canGoBack: boolean; goBack: () => void; startNewBatch?: () => void } | undefined,
    ) => {
      if (navigation) setQuestionNavigation(navigation)
    },
    [],
  )
  const recordAINavigation = useCallback(
    (navigation: { canGoBack: boolean; goBack: () => void } | undefined) =>
      setAINavigation(navigation),
    [],
  )
  const questionBusy = useCallback(
    (busy: boolean) => {
      if (actor.getSnapshot().matches({ personal: { collecting: 'questions' } }))
        actor.send({ ownerId, type: 'CHILD_BUSY', busy })
    },
    [actor, ownerId],
  )
  const pasteBusy = useCallback(
    (busy: boolean) => {
      if (actor.getSnapshot().matches({ personal: { collecting: 'paste' } }))
        actor.send({ ownerId, type: 'CHILD_BUSY', busy })
    },
    [actor, ownerId],
  )
  const aiBusy = useCallback(
    (busy: boolean) => {
      if (actor.getSnapshot().matches('ai')) actor.send({ ownerId, type: 'CHILD_BUSY', busy })
    },
    [actor, ownerId],
  )
  const legacyBusy = useCallback(
    (busy: boolean) => {
      if (actor.getSnapshot().matches('legacy')) actor.send({ ownerId, type: 'CHILD_BUSY', busy })
    },
    [actor, ownerId],
  )
  const back = useCallback(() => {
    const current = actor.getSnapshot()
    if (current.hasTag('busy') || current.context.childBusy) return true
    if (current.matches('choose') || current.matches('done')) return false
    const inner = current.matches('ai')
      ? aiNavigation
      : current.matches({ personal: { collecting: 'questions' } })
        ? questionNavigation
        : undefined
    if (inner?.canGoBack) {
      inner.goBack()
      return true
    }
    actor.send({ type: 'BACK', ownerId })
    return true
  }, [actor, ownerId, aiNavigation, questionNavigation])
  useEffect(() => {
    navigationCallback?.({ back, busy })
    return () => navigationCallback?.(null)
  }, [navigationCallback, back, busy])
  const view = snapshot.matches('choose')
    ? 'choose'
    : snapshot.matches('preparingPersonal')
      ? 'preparing'
      : snapshot.matches({ personal: { collecting: 'paste' } })
        ? 'paste'
        : snapshot.matches({ personal: { collecting: 'questions' } })
          ? 'questions'
          : snapshot.matches({ personal: 'review' })
            ? 'review'
            : snapshot.matches({ personal: 'analyzing' })
              ? 'analyzing'
              : snapshot.matches({ personal: 'confirmed' }) ||
                  snapshot.matches({ personal: 'confirming' })
                ? 'confirm'
                : snapshot.matches('ai')
                  ? 'ai'
                  : snapshot.matches('legacy')
                    ? 'legacy'
                    : snapshot.matches('failure')
                      ? 'failure'
                      : snapshot.matches('done')
                        ? 'done'
                        : 'checking'
  const heading = useRef<HTMLDivElement>(null)
  useEffect(() => {
    heading.current?.querySelector('h2')?.focus({ preventScroll: true })
  }, [view])
  const titleId = useId()
  const profile = context.profile ?? query.profile
  const common = profile
    ? {
        ownerId,
        voiceId: context.voiceId,
        profile,
        onBusyChange: (busy: boolean) =>
          send({ type: 'CHILD_BUSY', busy } as Omit<LearningEvent, 'ownerId'>),
        onReview: () => send({ type: 'REVIEW' }),
        onConfirmed: (voiceId: string) =>
          send({ type: 'EXTERNAL_SAVED', voiceId } as Omit<LearningEvent, 'ownerId'>),
        pasteDraft: context.pasteDraft,
        onNavigationChange: recordQuestionNavigation,
        onPasteDraft: (draft: { label: string; body: string }) =>
          send({ type: 'PASTE_DRAFT', ...draft } as Omit<LearningEvent, 'ownerId'>),
      }
    : undefined
  const externalProps = {
    ownerId,
    active: view === 'ai',
    onNavigationChange: recordAINavigation,
    onBusyChange: aiBusy,
    onConfirmed: (voiceId: string) => {
      if (actor.getSnapshot().matches('ai'))
        send({ type: 'EXTERNAL_SAVED', voiceId } as Omit<LearningEvent, 'ownerId'>)
    },
  }
  const title =
    view === 'choose'
      ? t('choose')
      : view === 'paste'
        ? t('collectingPaste')
        : view === 'questions'
          ? t('collectingQuestions')
          : view === 'review'
            ? t('review')
            : view === 'analyzing'
              ? t('analyzing')
              : view === 'confirm'
                ? t('confirm')
                : view === 'failure'
                  ? t('failed')
                  : view === 'done'
                    ? t(context.confirmed?.isDefault ? 'used' : 'savedStyle')
                    : t('preparing')
  return (
    <section aria-labelledby={titleId} className="min-w-0" ref={heading}>
      {!['ai', 'legacy'].includes(view) && (
        <Typography variant="stepTitle" as="h2" id={titleId} tabIndex={-1}>
          {title}
        </Typography>
      )}
      {view === 'choose' && (
        <>
          <Typography variant="body" className="text-content-secondary mt-3">
            {t('chooseHelp')}
          </Typography>
          <div className="mt-6 space-y-3">
            <ChoiceButton
              title={t('questions')}
              description={t('questionsHelp')}
              icon={<MessageCircle aria-hidden className="size-5" />}
              onClick={() =>
                send({ type: 'CHOOSE', method: 'questions' } as Omit<LearningEvent, 'ownerId'>)
              }
            />
            <ChoiceButton
              title={t('paste')}
              description={t('pasteHelp')}
              icon={<FileText aria-hidden className="size-5" />}
              onClick={() =>
                send({ type: 'CHOOSE', method: 'paste' } as Omit<LearningEvent, 'ownerId'>)
              }
            />
            {props.renderAI && (
              <ChoiceButton
                title={t('ai')}
                description={t('aiHelp')}
                icon={<Sparkles aria-hidden className="size-5" />}
                onClick={() =>
                  send({ type: 'CHOOSE', method: 'ai' } as Omit<LearningEvent, 'ownerId'>)
                }
              />
            )}
          </div>
        </>
      )}
      {view === 'paste' && (
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('collectingPasteHelp')}
        </Typography>
      )}
      {view === 'questions' && (
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('collectingQuestionsHelp')}
        </Typography>
      )}
      {common && context.visitedPaste && (
        <div hidden={view !== 'paste'} className="mt-6">
          {props.renderPaste({
            ...common,
            active: view === 'paste',
            onPasteDraft: (draft) => {
              if (actor.getSnapshot().matches({ personal: { collecting: 'paste' } }))
                common.onPasteDraft(draft)
            },
            onBusyChange: pasteBusy,
            onReview: () => {
              if (actor.getSnapshot().matches({ personal: { collecting: 'paste' } }))
                common.onReview()
            },
          })}
        </div>
      )}
      {common && context.visitedQuestions && (
        <div hidden={view !== 'questions'} className="mt-6">
          {props.renderQuestions({
            ...common,
            active: view === 'questions',
            onBusyChange: questionBusy,
            onReview: () => {
              if (actor.getSnapshot().matches({ personal: { collecting: 'questions' } }))
                common.onReview()
            },
          })}
        </div>
      )}
      {context.visitedAI && props.renderAI && (
        <div hidden={view !== 'ai'}>{props.renderAI(externalProps)}</div>
      )}
      {context.visitedLegacy && props.renderLegacy && (
        <div hidden={view !== 'legacy'}>
          {props.renderLegacy({
            ...externalProps,
            active: view === 'legacy',
            onBusyChange: legacyBusy,
            onConfirmed: (voiceId) => {
              if (actor.getSnapshot().matches('legacy'))
                send({ type: 'EXTERNAL_SAVED', voiceId } as Omit<LearningEvent, 'ownerId'>)
            },
          })}
        </div>
      )}
      {view === 'review' && profile && (
        <div className="mt-4 space-y-5">
          <Typography variant="body" className="text-content-secondary">
            {t('reviewHelp')}
          </Typography>
          <Typography variant="body">
            {t('savedCount', { count: profile.samples.length })}
          </Typography>
          <VoiceReadinessMeter readiness={profile.readiness} />
          {profile.readiness.percent >= 100 ? (
            model.selected && !model.isPending && !modelDefaults.isPending ? (
              <Button
                variant="cta"
                onClick={() =>
                  send({ type: 'ANALYZE', model: { ...model.selected! } } as Omit<
                    LearningEvent,
                    'ownerId'
                  >)
                }
              >
                {t('analyze')}
              </Button>
            ) : (
              <div className="space-y-3">
                <Typography variant="body" role="status">
                  {t(model.isPending || modelDefaults.isPending ? 'aiPreparing' : 'aiUnavailable')}
                </Typography>
                {!model.isPending && !modelDefaults.isPending && (
                  <>
                    <Button variant="secondary" onClick={modelDefaults.retry}>
                      {t('retryAI')}
                    </Button>
                    <Link to="/ai-models" className={buttonStyles({ variant: 'ghost' })}>
                      {t('checkSettings')}
                    </Link>
                  </>
                )}
              </div>
            )
          ) : (
            <>
              <Typography variant="body">{t('notReady')}</Typography>
              <Button
                variant="cta"
                onClick={() => {
                  if (context.method === 'questions') questionNavigation?.startNewBatch?.()
                  send({ type: 'ADD_MORE' })
                }}
              >
                {t('addMore')}
              </Button>
            </>
          )}
          {profile.readiness.percent >= 100 && (
            <Button
              variant="ghost"
              onClick={() => {
                if (context.method === 'questions') questionNavigation?.startNewBatch?.()
                send({ type: 'ADD_MORE' })
              }}
            >
              {t('addMore')}
            </Button>
          )}
          {props.renderMaterials && common && (
            <details>
              <summary className="text-link-fg min-h-11 cursor-pointer py-3">
                {t('showMaterials')}
              </summary>
              {props.renderMaterials({ ...common, active: true })}
            </details>
          )}
        </div>
      )}
      {view === 'analyzing' && (
        <div className="mt-4 space-y-4">
          <Typography variant="body" role="status">
            {t('analyzingHelp')}
          </Typography>
          {(job.isError || job.job?.status === 'done') && (
            <Button
              variant="secondary"
              onClick={() => {
                if (job.job?.status === 'done')
                  send({ type: 'RECHECK' } as Omit<LearningEvent, 'ownerId'>)
                else job.refetch()
              }}
            >
              {t('retry')}
            </Button>
          )}
        </div>
      )}
      {view === 'confirm' && profile && (
        <div className="mt-4 space-y-5">
          <Typography variant="body" className="text-content-secondary">
            {t('confirmHelp')}
          </Typography>
          <Typography variant="fieldTitle" as="h3">
            {profile.voice.name}
          </Typography>
          {profile.analysis?.ai.impression && (
            <Typography variant="body" className="break-words">
              {profile.analysis.ai.impression}
            </Typography>
          )}
          <Button
            variant="cta"
            pending={busy}
            disabled={busy}
            onClick={() => send({ type: 'USE' })}
          >
            {t('use')}
          </Button>
        </div>
      )}
      {view === 'failure' && (
        <Notice tone="danger" role="alert" className="mt-4">
          {context.failure && <AppFailureMessage failure={context.failure} />}
          <Button
            variant="secondary"
            className="mt-4"
            onClick={() => {
              if (context.failureStep === 'load') query.refetch()
              send({ type: 'RETRY' })
            }}
          >
            {t('retry')}
          </Button>
        </Notice>
      )}
      {!props.onNavigationChange && view !== 'choose' && view !== 'done' && (
        <Button variant="ghost" disabled={busy} className="mt-6" onClick={back}>
          {t(
            view === 'paste' || view === 'questions' || view === 'ai' || view === 'legacy'
              ? 'anotherMethod'
              : 'back',
          )}
        </Button>
      )}
    </section>
  )
}
