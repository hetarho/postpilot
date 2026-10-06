import { assign, getInitialSnapshot, getNextSnapshot, setup, type SnapshotFrom } from 'xstate'
import { VOICE_INITIAL_QUESTION_COUNT, type VoicePromptPart } from '@/entities/voice'

export const QUESTIONNAIRE_BATCH_SIZE = VOICE_INITIAL_QUESTION_COUNT

export interface QuestionnaireQuestion {
  key: string
  photo: boolean
  starter?: boolean
  part?: VoicePromptPart
}
export interface QuestionnaireState {
  ownerId: string
  voiceId: string
  phase: 'loading' | 'answering' | 'saving' | 'failed' | 'complete'
  mode: 'starter' | 'more'
  catalog: readonly QuestionnaireQuestion[]
  saved: readonly string[]
  coveredParts: readonly VoicePromptPart[]
  baselineAnswered: number
  sessionSaved: readonly string[]
  skipped: readonly string[]
  history: readonly string[]
  cursor: number
  drafts: Readonly<Record<string, string>>
  operation: number
  completion: 'ten' | 'exhausted' | null
  reviewing: boolean
}

type Identity = { ownerId: string; voiceId: string }
export type QuestionnaireEvent = Identity &
  (
    | {
        type: 'hydrate'
        catalog: readonly QuestionnaireQuestion[]
        saved: readonly string[]
        made: boolean
        answeredQuestions?: number
        missingParts?: readonly VoicePromptPart[]
      }
    | { type: 'draft'; key: string; body: string }
    | { type: 'begin'; key: string }
    | { type: 'success' | 'failure'; key: string; operation: number }
    | { type: 'back' | 'skip' | 'new-session' }
    | { type: 'review'; key: string }
  )

function initialQuestionnaireStateData(ownerId: string, voiceId: string): QuestionnaireContext {
  return {
    ownerId,
    voiceId,
    mode: 'starter',
    catalog: [],
    saved: [],
    coveredParts: [],
    baselineAnswered: 0,
    sessionSaved: [],
    skipped: [],
    history: [],
    cursor: 0,
    drafts: {},
    operation: 0,
    completion: null,
    reviewing: false,
  }
}

export function questionnaireSavedCount(state: QuestionnaireContext): number {
  return Math.min(QUESTIONNAIRE_BATCH_SIZE, state.baselineAnswered + state.sessionSaved.length)
}
export function questionnaireCurrentKey(state: QuestionnaireContext): string {
  return state.history[state.cursor] ?? ''
}
function nextKey(state: QuestionnaireContext, part?: VoicePromptPart): string | undefined {
  const available = state.catalog.filter(
    (question) =>
      (state.mode !== 'starter' || !question.photo) &&
      (part === undefined || question.part === part) &&
      !state.saved.includes(question.key) &&
      !state.skipped.includes(question.key) &&
      !state.history.includes(question.key),
  )
  if (state.mode !== 'starter') return available[0]?.key
  const missing = (['opening', 'description', 'closing'] as const).filter(
    (candidate) => !state.coveredParts.includes(candidate),
  )
  const coversMissing = (question: QuestionnaireQuestion) =>
    question.part !== undefined && missing.includes(question.part)
  return (
    available.find((question) => question.starter && coversMissing(question))?.key ??
    available.find(coversMissing)?.key ??
    available.find((question) => question.starter)?.key ??
    available[0]?.key
  )
}

type QuestionnaireContext = Omit<QuestionnaireState, 'phase'>
function owns(context: QuestionnaireContext, event: QuestionnaireEvent) {
  return (
    !!context.ownerId &&
    !!context.voiceId &&
    context.ownerId === event.ownerId &&
    context.voiceId === event.voiceId
  )
}
function confirmed(context: QuestionnaireContext): QuestionnaireContext {
  const key = questionnaireCurrentKey(context)
  const wasSaved = context.saved.includes(key)
  const part = context.catalog.find((question) => question.key === key)?.part
  return {
    ...context,
    saved: wasSaved ? context.saved : [...context.saved, key],
    sessionSaved: wasSaved ? context.sessionSaved : [...context.sessionSaved, key],
    coveredParts:
      part && !context.coveredParts.includes(part)
        ? [...context.coveredParts, part]
        : context.coveredParts,
  }
}
function replacement(context: QuestionnaireContext) {
  const key = questionnaireCurrentKey(context)
  return nextKey(
    { ...context, skipped: [...context.skipped, key] },
    context.catalog.find((q) => q.key === key)?.part,
  )
}
const editableQuestion = {
  draft: { guard: 'currentQuestion', actions: 'draft' },
  begin: { guard: 'currentQuestion', target: 'saving', actions: 'begin' },
  skip: [
    { guard: 'replaceable', target: 'answering', actions: 'replace' },
    { guard: 'skippable', target: 'complete', actions: ['skip', 'exhausted'] },
  ],
  back: [
    { guard: 'reviewing', target: 'complete', actions: 'stopReview' },
    { guard: 'previous', target: 'answering', actions: 'previous' },
  ],
} as const
export const questionnaireMachine = setup({
  types: {
    context: {} as QuestionnaireContext,
    events: {} as QuestionnaireEvent,
    input: {} as Identity,
  },
  guards: {
    owned: ({ context, event }) => owns(context, event),
    currentQuestion: ({ context, event }) =>
      owns(context, event) && 'key' in event && event.key === questionnaireCurrentKey(context),
    matchingOperation: ({ context, event }) =>
      owns(context, event) &&
      'operation' in event &&
      'key' in event &&
      event.operation === context.operation &&
      event.key === questionnaireCurrentKey(context),
    reviewingSuccess: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'success' &&
      event.operation === context.operation &&
      event.key === questionnaireCurrentKey(context) &&
      context.reviewing,
    tenAfterSuccess: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'success' &&
      event.operation === context.operation &&
      event.key === questionnaireCurrentKey(context) &&
      questionnaireSavedCount(confirmed(context)) >= QUESTIONNAIRE_BATCH_SIZE,
    nextAfterSuccess: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'success' &&
      event.operation === context.operation &&
      event.key === questionnaireCurrentKey(context) &&
      context.cursor < context.history.length - 1,
    reviewing: ({ context, event }) => owns(context, event) && context.reviewing,
    previous: ({ context, event }) => owns(context, event) && context.cursor > 0,
    hasHistory: ({ context, event }) => owns(context, event) && context.history.length > 0,
    canReview: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'review' &&
      context.saved.includes(event.key) &&
      context.catalog.some((q) => q.key === event.key),
    skippable: ({ context, event }) =>
      owns(context, event) &&
      !context.saved.includes(questionnaireCurrentKey(context)) &&
      !context.reviewing,
    replaceable: ({ context, event }) =>
      owns(context, event) &&
      !context.saved.includes(questionnaireCurrentKey(context)) &&
      !context.reviewing &&
      !!replacement(context),
    ten: ({ context }) => questionnaireSavedCount(context) >= QUESTIONNAIRE_BATCH_SIZE,
    hasNext: ({ context }) => !!nextKey(context),
  },
  actions: {
    hydrate: assign(({ event }) => {
      if (event.type !== 'hydrate') return {}
      const catalog = event.catalog.filter(
        (q, index, all) => q.key && all.findIndex((item) => item.key === q.key) === index,
      )
      const saved = [...new Set(event.saved.filter(Boolean))]
      const count =
        typeof event.answeredQuestions === 'number' &&
        Number.isFinite(event.answeredQuestions) &&
        event.answeredQuestions >= 0
          ? Math.floor(event.answeredQuestions)
          : saved.length
      return {
        catalog,
        saved,
        coveredParts: event.missingParts
          ? (['opening', 'description', 'closing'] as const).filter(
              (part) => !event.missingParts!.includes(part),
            )
          : [
              ...new Set(
                catalog
                  .filter((q) => saved.includes(q.key) && q.part !== undefined)
                  .map((q) => q.part!),
              ),
            ],
        mode: event.made ? ('more' as const) : ('starter' as const),
        baselineAnswered: event.made ? 0 : Math.min(QUESTIONNAIRE_BATCH_SIZE, count),
      }
    }),
    next: assign(({ context }) => ({
      history: [...context.history, nextKey(context)!],
      cursor: context.history.length,
      completion: null,
    })),
    ten: assign({ completion: 'ten' as const }),
    exhausted: assign({ completion: 'exhausted' as const }),
    confirmed: assign(({ context }) => confirmed(context)),
    stopReview: assign({ reviewing: false }),
    previous: assign(({ context }) => ({ cursor: context.cursor - 1 })),
    nextExisting: assign(({ context }) => ({ cursor: context.cursor + 1 })),
    last: assign(({ context }) => ({ cursor: context.history.length - 1 })),
    draft: assign(({ context, event }) =>
      event.type === 'draft' ? { drafts: { ...context.drafts, [event.key]: event.body } } : {},
    ),
    begin: assign(({ context }) => ({ operation: context.operation + 1 })),
    skip: assign(({ context }) => ({
      skipped: [...context.skipped, questionnaireCurrentKey(context)],
    })),
    replace: assign(({ context }) => ({
      skipped: [...context.skipped, questionnaireCurrentKey(context)],
      history: context.history.map((key, index) =>
        index === context.cursor ? replacement(context)! : key,
      ),
    })),
    review: assign(({ context, event }) =>
      event.type === 'review'
        ? { history: [event.key], cursor: 0, reviewing: true, operation: context.operation + 1 }
        : {},
    ),
    restart: assign(({ context }) => ({
      mode: 'more' as const,
      baselineAnswered: 0,
      sessionSaved: [],
      skipped: [],
      history: [],
      cursor: 0,
      operation: context.operation + 1,
      completion: null,
      reviewing: false,
    })),
  },
}).createMachine({
  id: 'voiceQuestionnaire',
  initial: 'loading',
  context: ({ input }) => initialQuestionnaireStateData(input.ownerId, input.voiceId),
  states: {
    loading: { on: { hydrate: { guard: 'owned', target: 'routing', actions: 'hydrate' } } },
    routing: {
      always: [
        { guard: 'ten', target: 'complete', actions: 'ten' },
        { guard: 'hasNext', target: 'answering', actions: 'next' },
        { target: 'complete', actions: 'exhausted' },
      ],
    },
    answering: { on: editableQuestion },
    failed: { on: editableQuestion },
    saving: {
      on: {
        failure: { guard: 'matchingOperation', target: 'failed' },
        success: [
          { guard: 'reviewingSuccess', target: 'complete', actions: ['confirmed', 'stopReview'] },
          { guard: 'tenAfterSuccess', target: 'complete', actions: ['confirmed', 'ten'] },
          {
            guard: 'nextAfterSuccess',
            target: 'answering',
            actions: ['confirmed', 'nextExisting'],
          },
          { guard: 'matchingOperation', target: 'routing', actions: 'confirmed' },
        ],
      },
    },
    complete: {
      on: {
        'new-session': { guard: 'owned', target: 'routing', actions: 'restart' },
        review: { guard: 'canReview', target: 'answering', actions: 'review' },
        back: { guard: 'hasHistory', target: 'answering', actions: 'last' },
      },
    },
  },
})
const projections = new WeakMap<object, QuestionnaireState>()
export function questionnaireStateOf(
  snapshot: SnapshotFrom<typeof questionnaireMachine>,
): QuestionnaireState {
  let state = projections.get(snapshot)
  if (!state) {
    state = { ...snapshot.context, phase: snapshot.value as QuestionnaireState['phase'] }
    projections.set(snapshot, state)
  }
  return state
}
export function initialQuestionnaireState(ownerId: string, voiceId: string): QuestionnaireState {
  return questionnaireStateOf(getInitialSnapshot(questionnaireMachine, { ownerId, voiceId }))
}
export function questionnaireTransition(
  state: QuestionnaireState,
  event: QuestionnaireEvent,
): QuestionnaireState {
  const { phase, ...context } = state
  const snapshot = questionnaireMachine.resolveState({ value: phase, context })
  const next = getNextSnapshot(questionnaireMachine, snapshot, event)
  return next.context === snapshot.context && next.value === snapshot.value
    ? state
    : questionnaireStateOf(next)
}
