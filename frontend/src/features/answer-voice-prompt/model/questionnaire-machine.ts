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

export function initialQuestionnaireState(ownerId: string, voiceId: string): QuestionnaireState {
  return {
    ownerId,
    voiceId,
    phase: 'loading',
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

export function questionnaireSavedCount(state: QuestionnaireState): number {
  return Math.min(QUESTIONNAIRE_BATCH_SIZE, state.baselineAnswered + state.sessionSaved.length)
}
export function questionnaireCurrentKey(state: QuestionnaireState): string {
  return state.history[state.cursor] ?? ''
}
function nextKey(state: QuestionnaireState, part?: VoicePromptPart): string | undefined {
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
function startQuestion(state: QuestionnaireState): QuestionnaireState {
  if (questionnaireSavedCount(state) >= QUESTIONNAIRE_BATCH_SIZE)
    return { ...state, phase: 'complete', completion: 'ten' }
  const key = nextKey(state)
  return key
    ? {
        ...state,
        phase: 'answering',
        history: [...state.history, key],
        cursor: state.history.length,
        completion: null,
      }
    : { ...state, phase: 'complete', completion: 'exhausted' }
}

/** Saved keys are facts; skipping and browsing never turn a question into an answer. */
export function questionnaireTransition(
  state: QuestionnaireState,
  event: QuestionnaireEvent,
): QuestionnaireState {
  if (
    !state.ownerId ||
    !state.voiceId ||
    event.ownerId !== state.ownerId ||
    event.voiceId !== state.voiceId
  )
    return state
  if (event.type === 'hydrate') {
    if (state.phase !== 'loading') return state
    const catalog = event.catalog.filter(
      (question, index, all) =>
        question.key && all.findIndex((item) => item.key === question.key) === index,
    )
    const saved = [...new Set(event.saved.filter(Boolean))]
    const serverCount = event.answeredQuestions
    const counted =
      typeof serverCount === 'number' && Number.isFinite(serverCount) && serverCount >= 0
        ? Math.floor(serverCount)
        : saved.length
    return startQuestion({
      ...state,
      catalog,
      saved,
      coveredParts: event.missingParts
        ? (['opening', 'description', 'closing'] as const).filter(
            (part) => !event.missingParts!.includes(part),
          )
        : [
            ...new Set(
              catalog
                .filter((question) => saved.includes(question.key) && question.part !== undefined)
                .map((question) => question.part!),
            ),
          ],
      mode: event.made ? 'more' : 'starter',
      baselineAnswered: event.made ? 0 : Math.min(QUESTIONNAIRE_BATCH_SIZE, counted),
    })
  }
  if (state.phase === 'loading') return state
  const key = questionnaireCurrentKey(state)
  if (event.type === 'success' || event.type === 'failure') {
    if (state.phase !== 'saving' || event.key !== key || event.operation !== state.operation)
      return state
    if (event.type === 'failure') return { ...state, phase: 'failed' }
    const wasSaved = state.saved.includes(key)
    const part = state.catalog.find((question) => question.key === key)?.part
    const confirmed = {
      ...state,
      saved: wasSaved ? state.saved : [...state.saved, key],
      sessionSaved: wasSaved ? state.sessionSaved : [...state.sessionSaved, key],
      coveredParts:
        part && !state.coveredParts.includes(part)
          ? [...state.coveredParts, part]
          : state.coveredParts,
    }
    if (state.reviewing) return { ...confirmed, phase: 'complete', reviewing: false }
    if (questionnaireSavedCount(confirmed) >= QUESTIONNAIRE_BATCH_SIZE)
      return { ...confirmed, phase: 'complete', completion: 'ten' }
    if (state.cursor < state.history.length - 1)
      return { ...confirmed, phase: 'answering', cursor: state.cursor + 1 }
    return startQuestion(confirmed)
  }
  if (state.phase === 'saving') return state
  if (event.type === 'new-session') {
    if (state.phase !== 'complete') return state
    return startQuestion({
      ...state,
      mode: 'more',
      baselineAnswered: 0,
      sessionSaved: [],
      skipped: [],
      history: [],
      cursor: 0,
      operation: state.operation + 1,
      completion: null,
      reviewing: false,
    })
  }
  if (event.type === 'review') {
    if (
      state.phase !== 'complete' ||
      !state.saved.includes(event.key) ||
      !state.catalog.some((question) => question.key === event.key)
    )
      return state
    return {
      ...state,
      phase: 'answering',
      history: [event.key],
      cursor: 0,
      reviewing: true,
      operation: state.operation + 1,
    }
  }
  if (event.type === 'back') {
    if (state.reviewing) return { ...state, phase: 'complete', reviewing: false }
    if (state.phase === 'complete' && state.history.length)
      return { ...state, phase: 'answering', cursor: state.history.length - 1 }
    return state.cursor > 0 ? { ...state, phase: 'answering', cursor: state.cursor - 1 } : state
  }
  if (state.phase !== 'answering' && state.phase !== 'failed') return state
  if (event.type === 'draft')
    return event.key === key ? { ...state, drafts: { ...state.drafts, [key]: event.body } } : state
  if (event.type === 'begin')
    return event.key === key ? { ...state, phase: 'saving', operation: state.operation + 1 } : state
  if (event.type === 'skip') {
    if (state.saved.includes(key) || state.reviewing) return state
    const skipped = { ...state, skipped: [...state.skipped, key] }
    const replacement = nextKey(
      skipped,
      state.catalog.find((question) => question.key === key)?.part,
    )
    return replacement
      ? {
          ...skipped,
          phase: 'answering',
          history: state.history.map((item, index) =>
            index === state.cursor ? replacement : item,
          ),
        }
      : { ...skipped, phase: 'complete', completion: 'exhausted' }
  }
  return state
}
