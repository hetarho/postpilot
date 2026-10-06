export const SETUP_FORMS = ['voice', 'post-template', 'clip-template'] as const
export type SetupForm = (typeof SETUP_FORMS)[number]
export type SetupStep = 'welcome' | SetupForm | 'ready'
export type SetupTarget = '/' | '/posts/new' | '/clips/new'
export interface SetupProgress {
  completed: boolean
  skipped: SetupForm[]
  resume: SetupStep
  target: SetupTarget
}
export interface SetupState {
  ownerId: string
  phase: 'checking' | 'editing' | 'saving' | 'running' | 'failed' | 'completed'
  step: SetupStep
  plan: SetupForm[]
  resolved: SetupForm[]
  skipped: SetupForm[]
  operation: number
  target: SetupTarget
}
export type SetupEvent = { ownerId: string } & (
  | { type: 'hydrate'; missing: readonly SetupForm[]; progress: SetupProgress }
  | { type: 'next'; step: SetupStep; confirmed: boolean }
  | { type: 'begin'; step: SetupForm }
  | { type: 'running'; step: SetupForm; operation: number }
  | { type: 'success'; step: SetupForm; operation: number; complete: boolean }
  | { type: 'failure'; step: SetupForm; operation: number }
  | { type: 'skip'; step: SetupForm }
  | { type: 'back' }
  | { type: 'finish'; target: SetupTarget }
  | { type: 'defer'; target: SetupTarget }
)
export const emptySetupProgress = (): SetupProgress => ({
  completed: false,
  skipped: [],
  resume: 'welcome',
  target: '/',
})
export const initialSetupState = (ownerId: string): SetupState => ({
  ownerId,
  phase: 'checking',
  step: 'welcome',
  plan: [],
  resolved: [],
  skipped: [],
  operation: 0,
  target: '/',
})
export function safeSetupTarget(value: unknown): SetupTarget {
  return value === '/posts/new' || value === '/clips/new' ? value : '/'
}
function advance(state: SetupState): SetupState {
  const step =
    state.plan.find(
      (candidate) => !state.resolved.includes(candidate) && !state.skipped.includes(candidate),
    ) ?? 'ready'
  return { ...state, phase: 'editing', step }
}
export function setupTransition(state: SetupState, event: SetupEvent): SetupState {
  if (!state.ownerId || event.ownerId !== state.ownerId || state.phase === 'completed') return state
  if (event.type === 'hydrate') {
    if (state.phase !== 'checking') return state
    const plan = SETUP_FORMS.filter((step) => event.missing.includes(step))
    const skipped = event.progress.skipped.filter((step) => plan.includes(step))
    const resume =
      plan.includes(event.progress.resume as SetupForm) &&
      !skipped.includes(event.progress.resume as SetupForm)
        ? event.progress.resume
        : 'welcome'
    return {
      ...state,
      phase: 'editing',
      plan,
      skipped,
      step: plan.every((step) => skipped.includes(step)) ? 'ready' : resume,
      target: safeSetupTarget(event.progress.target),
    }
  }
  if (event.type === 'defer' && state.phase === 'checking')
    return { ...state, phase: 'completed', target: safeSetupTarget(event.target) }
  if (state.phase === 'checking') return state
  if (event.type === 'success' || event.type === 'failure' || event.type === 'running') {
    if (
      (state.phase !== 'saving' && state.phase !== 'running') ||
      event.step !== state.step ||
      event.operation !== state.operation
    )
      return state
    if (event.type === 'failure') return { ...state, phase: 'failed' }
    if (event.type === 'running') return { ...state, phase: 'running' }
    return event.complete
      ? advance({ ...state, resolved: [...new Set([...state.resolved, event.step])] })
      : { ...state, phase: 'editing' }
  }
  if (state.phase !== 'editing' && state.phase !== 'failed') return state
  switch (event.type) {
    case 'begin':
      return event.step === state.step
        ? { ...state, phase: 'saving', operation: state.operation + 1 }
        : state
    case 'next':
      if (event.step !== state.step || !event.confirmed || state.step === 'ready') return state
      return advance({
        ...state,
        resolved:
          state.step === 'welcome' ? state.resolved : [...new Set([...state.resolved, state.step])],
      })
    case 'skip':
      return event.step === state.step
        ? advance({ ...state, skipped: [...new Set([...state.skipped, event.step])] })
        : state
    case 'back': {
      const before =
        state.step === 'ready'
          ? state.plan
          : state.plan.slice(0, state.plan.indexOf(state.step as SetupForm))
      return {
        ...state,
        phase: 'editing',
        step:
          before.findLast(
            (step) => !state.resolved.includes(step) && !state.skipped.includes(step),
          ) ?? 'welcome',
      }
    }
    case 'finish':
      return state.step === 'ready'
        ? { ...state, phase: 'completed', target: safeSetupTarget(event.target) }
        : state
    case 'defer':
      return { ...state, phase: 'completed', target: safeSetupTarget(event.target) }
  }
}
export function setupProgressOf(state: SetupState): SetupProgress {
  return {
    completed: state.phase === 'completed',
    skipped: state.skipped,
    resume: state.step,
    target: state.target,
  }
}
