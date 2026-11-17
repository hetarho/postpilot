import { and, assign, getInitialSnapshot, getNextSnapshot, setup, type SnapshotFrom } from 'xstate'
import {
  VOICE_EXPERIENCE_REPLIES,
  VOICE_EXPERIENCE_PART_QUOTA,
  VOICE_INITIAL_QUESTION_COUNT,
  type VoiceExperienceKindName,
  type VoicePromptPart,
} from '@/entities/voice'

export const QUESTIONNAIRE_BATCH_SIZE = VOICE_INITIAL_QUESTION_COUNT

export interface QuestionnaireQuestion {
  key: string
  photo: boolean
  starter?: boolean
  part?: VoicePromptPart
  /** Set on a thread question; a situation carries none. */
  experienceKind?: VoiceExperienceKindName
}
export interface QuestionnaireExperience {
  id: string
  kind: VoiceExperienceKindName
}
export interface QuestionnaireState {
  ownerId: string
  voiceId: string
  phase:
    'loading' | 'choosing' | 'starting' | 'offer' | 'answering' | 'saving' | 'failed' | 'complete'
  mode: 'starter' | 'more'
  /** Which kind of question the chat asks now: the owner's experience, imagined situations, or
   *  undecided until the first route is taken. */
  track: 'none' | 'experience' | 'situations'
  experiences: readonly QuestionnaireExperience[]
  experienceId: string
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
  completion: 'ten' | 'exhausted' | 'saved' | null
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
        experiences?: readonly QuestionnaireExperience[]
      }
    | { type: 'start-experience' }
    | { type: 'experience-started'; experience: QuestionnaireExperience; operation: number }
    | { type: 'experience-failed'; operation: number }
    | { type: 'situations' | 'another-experience' | 'review-saved' }
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
    track: 'none',
    experiences: [],
    experienceId: '',
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

const TURN_SEPARATOR = '::'
/** A thread answer is identified by its experience and question (VOICE-65); a situation by its
 *  question alone. */
export function answerTurnKey(experienceId: string | undefined, promptKey: string): string {
  return experienceId ? `${experienceId}${TURN_SEPARATOR}${promptKey}` : promptKey
}
export function parseTurnKey(turnKey: string): { experienceId: string; promptKey: string } {
  const at = turnKey.indexOf(TURN_SEPARATOR)
  return at < 0
    ? { experienceId: '', promptKey: turnKey }
    : {
        experienceId: turnKey.slice(0, at),
        promptKey: turnKey.slice(at + TURN_SEPARATOR.length),
      }
}
function questionAt(context: QuestionnaireContext, turnKey: string) {
  const { promptKey } = parseTurnKey(turnKey)
  return context.catalog.find((question) => question.key === promptKey)
}
function threadOf(context: QuestionnaireContext, experienceId: string) {
  const kind = context.experiences.find((experience) => experience.id === experienceId)?.kind
  return kind ? context.catalog.filter((question) => question.experienceKind === kind) : []
}
/** How many replies this experience has saved. */
export function experienceReplies(context: QuestionnaireContext, experienceId: string): number {
  return context.saved.filter((key) => parseTurnKey(key).experienceId === experienceId).length
}
function threadNextKey(
  context: QuestionnaireContext,
  experienceId: string,
  part?: VoicePromptPart,
): string | undefined {
  const taken = (turnKey: string) =>
    context.saved.includes(turnKey) ||
    context.skipped.includes(turnKey) ||
    context.history.includes(turnKey)
  const assigned = (candidate: VoicePromptPart | undefined) =>
    new Set(
      [...context.saved, ...context.history].filter(
        (turnKey) =>
          parseTurnKey(turnKey).experienceId === experienceId &&
          !context.skipped.includes(turnKey) &&
          questionAt(context, turnKey)?.part === candidate,
      ),
    ).size
  return threadOf(context, experienceId)
    .map((question) => ({ question, turnKey: answerTurnKey(experienceId, question.key) }))
    .find(
      ({ question, turnKey }) =>
        !taken(turnKey) &&
        (part !== undefined
          ? question.part === part
          : question.part !== undefined &&
            assigned(question.part) < VOICE_EXPERIENCE_PART_QUOTA[question.part]),
    )?.turnKey
}
function unfinishedExperience(context: QuestionnaireContext): string {
  const open = [...context.experiences]
    .reverse()
    .find((experience) => experienceReplies(context, experience.id) < VOICE_EXPERIENCE_REPLIES)
  return open && threadNextKey(context, open.id) ? open.id : ''
}
/** The question that follows now on the chosen track. */
function nextTurnKey(context: QuestionnaireContext, part?: VoicePromptPart) {
  return context.track === 'experience'
    ? threadNextKey(context, context.experienceId, part)
    : nextKey(context, part)
}
function nextKey(state: QuestionnaireContext, part?: VoicePromptPart): string | undefined {
  const available = state.catalog.filter(
    (question) =>
      !question.experienceKind &&
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
  const part = questionAt(context, key)?.part
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
  const swapped = { ...context, skipped: [...context.skipped, key] }
  const { experienceId } = parseTurnKey(key)
  const part = questionAt(context, key)?.part
  return experienceId ? threadNextKey(swapped, experienceId, part) : nextKey(swapped, part)
}
/** Whether any imagined situation is still left to ask. */
export function questionnaireHasSituations(state: QuestionnaireContext): boolean {
  return !!nextKey(state)
}
/** Whether `다른 얘기 할래요` has another question to offer for the one on screen. */
export function questionnaireCanSkip(state: QuestionnaireContext): boolean {
  const key = questionnaireCurrentKey(state)
  return !state.saved.includes(key) && !state.reviewing && !!replacement(state)
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
    hasSaved: ({ context, event }) => owns(context, event) && context.saved.length > 0,
    hasHistory: ({ context, event }) => owns(context, event) && context.history.length > 0,
    canReview: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'review' &&
      context.saved.includes(event.key) &&
      !!questionAt(context, event.key),
    skippable: ({ context, event }) =>
      owns(context, event) &&
      !parseTurnKey(questionnaireCurrentKey(context)).experienceId &&
      !context.saved.includes(questionnaireCurrentKey(context)) &&
      !context.reviewing,
    startsExperience: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'experience-started' &&
      event.operation === context.operation,
    failsExperience: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'experience-failed' &&
      event.operation === context.operation,
    onExperience: ({ context }) => context.track === 'experience',
    onSituations: ({ context }) => context.track === 'situations',
    resumesExperience: ({ context }) => context.track === 'none' && !!unfinishedExperience(context),
    continuesSituations: ({ context }) =>
      context.track === 'none' &&
      context.mode === 'starter' &&
      context.experiences.length === 0 &&
      context.saved.length > 0 &&
      !!nextKey(context),
    asksFirstExperience: ({ context }) =>
      context.track === 'none' &&
      context.mode === 'starter' &&
      context.experiences.length === 0 &&
      context.saved.length === 0,
    hasThreadNext: ({ context }) => !!threadNextKey(context, context.experienceId),
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
        experiences: event.experiences ?? [],
        coveredParts: event.missingParts
          ? (['opening', 'description', 'closing'] as const).filter(
              (part) => !event.missingParts!.includes(part),
            )
          : [
              ...new Set(
                saved.flatMap((turnKey) => {
                  const part = catalog.find((q) => q.key === parseTurnKey(turnKey).promptKey)?.part
                  return part ? [part] : []
                }),
              ),
            ],
        mode: event.made ? ('more' as const) : ('starter' as const),
        baselineAnswered: event.made ? 0 : Math.min(QUESTIONNAIRE_BATCH_SIZE, count),
      }
    }),
    resume: assign(({ context }) => ({
      track: 'experience' as const,
      experienceId: unfinishedExperience(context),
    })),
    chooseSituations: assign({ track: 'situations' as const }),
    startedExperience: assign(({ context, event }) =>
      event.type === 'experience-started'
        ? {
            track: 'experience' as const,
            experienceId: event.experience.id,
            experiences: [
              ...context.experiences.filter((experience) => experience.id !== event.experience.id),
              event.experience,
            ],
          }
        : {},
    ),
    next: assign(({ context }) => ({
      history: [...context.history, nextTurnKey(context)!],
      cursor: context.history.length,
      completion: null,
    })),
    ten: assign({ completion: 'ten' as const }),
    exhausted: assign({ completion: 'exhausted' as const }),
    saved: assign({ completion: 'saved' as const }),
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
      track: 'none' as const,
      experienceId: '',
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
        { guard: and(['onExperience', 'hasThreadNext']), target: 'answering', actions: 'next' },
        { guard: 'onExperience', target: 'offer' },
        { guard: and(['onSituations', 'hasNext']), target: 'answering', actions: 'next' },
        { guard: 'onSituations', target: 'complete', actions: 'exhausted' },
        { guard: 'resumesExperience', target: 'answering', actions: ['resume', 'next'] },
        {
          guard: 'continuesSituations',
          target: 'answering',
          actions: ['chooseSituations', 'next'],
        },
        { guard: 'asksFirstExperience', target: 'choosing' },
        { target: 'offer' },
      ],
    },
    choosing: {
      on: {
        'start-experience': { guard: 'owned', target: 'starting', actions: 'begin' },
        situations: { guard: 'owned', target: 'routing', actions: 'chooseSituations' },
        back: { guard: 'hasHistory', target: 'answering', actions: 'last' },
      },
    },
    starting: {
      on: {
        'experience-started': {
          guard: 'startsExperience',
          target: 'routing',
          actions: 'startedExperience',
        },
        'experience-failed': { guard: 'failsExperience', target: 'choosing' },
      },
    },
    offer: {
      on: {
        'another-experience': { guard: 'owned', target: 'choosing' },
        situations: { guard: 'owned', target: 'routing', actions: 'chooseSituations' },
        'review-saved': { guard: 'hasSaved', target: 'complete', actions: 'saved' },
        back: { guard: 'hasHistory', target: 'answering', actions: 'last' },
      },
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
