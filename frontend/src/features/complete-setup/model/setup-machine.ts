import { assign, getInitialSnapshot, getNextSnapshot, setup, type SnapshotFrom } from 'xstate'

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
const initialSetupData = (ownerId: string): SetupContext => ({
  ownerId,
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

type SetupContext = Omit<SetupState, 'phase'>
function owns(context: SetupContext, event: SetupEvent) {
  return !!context.ownerId && context.ownerId === event.ownerId
}
function nextStep(context: SetupContext): SetupStep {
  return (
    context.plan.find(
      (step) => !context.resolved.includes(step) && !context.skipped.includes(step),
    ) ?? 'ready'
  )
}
function matchesOperation(context: SetupContext, event: SetupEvent) {
  return (
    owns(context, event) &&
    'operation' in event &&
    'step' in event &&
    event.operation === context.operation &&
    event.step === context.step
  )
}
const editable = {
  begin: { guard: 'currentStep', target: 'saving', actions: 'begin' },
  next: { guard: 'confirmedStep', target: 'editing', actions: 'advance' },
  skip: { guard: 'currentStep', target: 'editing', actions: 'skip' },
  back: { guard: 'owned', target: 'editing', actions: 'back' },
  finish: { guard: 'ready', target: 'completed', actions: 'destination' },
  defer: { guard: 'owned', target: 'completed', actions: 'destination' },
} as const
const pending = {
  running: { guard: 'matchingOperation', target: 'running' },
  failure: { guard: 'matchingOperation', target: 'failed' },
  success: { guard: 'matchingOperation', target: 'editing', actions: 'confirmedOperation' },
} as const
export const setupMachine = setup({
  types: {
    context: {} as SetupContext,
    events: {} as SetupEvent,
    input: {} as { ownerId: string },
  },
  guards: {
    owned: ({ context, event }) => owns(context, event),
    currentStep: ({ context, event }) =>
      owns(context, event) && 'step' in event && event.step === context.step,
    confirmedStep: ({ context, event }) =>
      owns(context, event) &&
      event.type === 'next' &&
      event.step === context.step &&
      event.confirmed &&
      context.step !== 'ready',
    ready: ({ context, event }) => owns(context, event) && context.step === 'ready',
    matchingOperation: ({ context, event }) => matchesOperation(context, event),
  },
  actions: {
    hydrate: assign(({ context, event }) => {
      if (event.type !== 'hydrate') return {}
      const plan = SETUP_FORMS.filter((step) => event.missing.includes(step))
      const skipped = event.progress.skipped.filter((step) => plan.includes(step))
      const resume =
        plan.includes(event.progress.resume as SetupForm) &&
        !skipped.includes(event.progress.resume as SetupForm)
          ? event.progress.resume
          : 'welcome'
      return {
        ...context,
        plan,
        skipped,
        step: plan.every((step) => skipped.includes(step)) ? 'ready' : resume,
        target: safeSetupTarget(event.progress.target),
      }
    }),
    begin: assign(({ context }) => ({ operation: context.operation + 1 })),
    advance: assign(({ context }) => {
      const resolved =
        context.step === 'welcome'
          ? context.resolved
          : [...new Set([...context.resolved, context.step as SetupForm])]
      return { resolved, step: nextStep({ ...context, resolved }) }
    }),
    skip: assign(({ context }) => {
      const skipped = [...new Set([...context.skipped, context.step as SetupForm])]
      return { skipped, step: nextStep({ ...context, skipped }) }
    }),
    back: assign(({ context }) => {
      const before =
        context.step === 'ready'
          ? context.plan
          : context.plan.slice(0, context.plan.indexOf(context.step as SetupForm))
      return {
        step:
          before.findLast(
            (step) => !context.resolved.includes(step) && !context.skipped.includes(step),
          ) ?? 'welcome',
      }
    }),
    destination: assign(({ event }) =>
      'target' in event ? { target: safeSetupTarget(event.target) } : {},
    ),
    confirmedOperation: assign(({ context, event }) => {
      if (event.type !== 'success' || !event.complete) return {}
      const resolved = [...new Set([...context.resolved, event.step])]
      return { resolved, step: nextStep({ ...context, resolved }) }
    }),
  },
}).createMachine({
  id: 'creationSetup',
  initial: 'checking',
  context: ({ input }) => initialSetupData(input.ownerId),
  states: {
    checking: {
      on: {
        hydrate: { guard: 'owned', target: 'editing', actions: 'hydrate' },
        defer: { guard: 'owned', target: 'completed', actions: 'destination' },
      },
    },
    editing: { on: editable },
    saving: { on: pending },
    running: { on: pending },
    failed: { on: editable },
    completed: {},
  },
})
const projections = new WeakMap<object, SetupState>()
export function setupStateOf(snapshot: SnapshotFrom<typeof setupMachine>): SetupState {
  let state = projections.get(snapshot)
  if (!state) {
    state = { ...snapshot.context, phase: snapshot.value as SetupState['phase'] }
    projections.set(snapshot, state)
  }
  return state
}
export function initialSetupStateOf(ownerId: string): SetupState {
  return setupStateOf(getInitialSnapshot(setupMachine, { ownerId }))
}
export { initialSetupStateOf as initialSetupState }
export function setupTransition(state: SetupState, event: SetupEvent): SetupState {
  const { phase, ...context } = state
  const snapshot = setupMachine.resolveState({ value: phase, context })
  const next = getNextSnapshot(setupMachine, snapshot, event)
  return next.context === snapshot.context && next.value === snapshot.value
    ? state
    : setupStateOf(next)
}
export function setupProgressOf(state: SetupState): SetupProgress {
  return {
    completed: state.phase === 'completed',
    skipped: state.skipped,
    resume: state.step,
    target: state.target,
  }
}
