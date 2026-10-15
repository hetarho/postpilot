import { assign, getInitialSnapshot, getNextSnapshot, setup, type SnapshotFrom } from 'xstate'
import { templateTargetAt } from '@/entities/template'
import { CLIP_COMPOSITION_LIMITS } from '@/entities/clip-design'
import { emptyClipRecipe, CLIP_TEMPLATE_LIMITS, type ClipRecipe } from '@/entities/clip-template'

export const STAGE_ROLES = [
  'hook',
  'intro',
  'highlight',
  'detail',
  'drawbacks',
  'info',
  'ending',
] as const
export type StageRole = (typeof STAGE_ROLES)[number]
export interface StageDraft {
  id: string
  role: StageRole | 'custom'
  name: string
  heading: string
  intent: string
  checked: boolean
  asks: boolean
  fieldLabel: string
}
export type Step = 'target' | 'stages' | 'order' | 'headings' | 'stage' | 'design' | 'finish'
export interface ClipWizardState {
  step: Step
  path: readonly string[]
  depth: number
  leaf: string
  stages: readonly StageDraft[]
  cursor: number
  customCount: number
  attempted: boolean
  name: string
  introPreset: ClipRecipe['introPreset']
  outroPreset: ClipRecipe['outroPreset']
  allowedCaptionStyles: string[]
}
type Context = Omit<ClipWizardState, 'step'>
export type ClipWizardEvent =
  | { type: 'choose'; key: string; seeds?: readonly StageDraft[] }
  | { type: 'next' | 'back' }
  | { type: 'toggle'; id: string; checked: boolean }
  | { type: 'add'; name: string; intent: string }
  | { type: 'remove'; id: string }
  | { type: 'reorder'; from: number; to: number }
  | {
      type: 'edit'
      id: string
      patch: Partial<Pick<StageDraft, 'heading' | 'intent' | 'asks' | 'fieldLabel'>>
    }
  | {
      type: 'design'
      patch: Partial<Pick<ClipRecipe, 'introPreset' | 'outroPreset' | 'allowedCaptionStyles'>>
    }
  | { type: 'name'; value: string }

export const checkedStages = (state: Pick<ClipWizardState, 'stages'>) =>
  state.stages.filter((stage) => stage.checked)
const length = (text: string) => Array.from(text.trim()).length
export function stageProblem(stage: StageDraft) {
  if (!stage.heading.trim()) return 'headingMissing' as const
  if (length(stage.heading) > CLIP_COMPOSITION_LIMITS.labelChars) return 'headingLong' as const
  if (!stage.intent.trim()) return 'intentMissing' as const
  if (length(stage.intent) > CLIP_COMPOSITION_LIMITS.promptChars || /[\r\n]/.test(stage.intent))
    return 'intentLong' as const
  if (stage.asks && !stage.fieldLabel.trim()) return 'fieldMissing' as const
  if (stage.asks && length(stage.fieldLabel) > CLIP_COMPOSITION_LIMITS.labelChars)
    return 'fieldLong' as const
  return undefined
}
export function nameProblem(name: string) {
  return !name.trim()
    ? ('nameMissing' as const)
    : length(name) > CLIP_TEMPLATE_LIMITS.name
      ? ('nameLong' as const)
      : undefined
}
function choice(context: Context, key: string) {
  const path = [...context.path.slice(0, context.depth), key]
  return { path, node: templateTargetAt(path) }
}
const current = (context: Context) => checkedStages(context)[context.cursor]

export const clipWizardMachine = setup({
  types: { context: {} as Context, events: {} as ClipWizardEvent },
  guards: {
    leaf: ({ context, event }) =>
      event.type === 'choose' &&
      !!choice(context, event.key).node?.roles &&
      (event.seeds?.length ?? 0) > 0,
    branch: ({ context, event }) =>
      event.type === 'choose' && !!choice(context, event.key).node?.children,
    earlierQuestion: ({ context }) => context.depth > 0,
    validSelection: ({ context }) =>
      checkedStages(context).length > 0 &&
      checkedStages(context).length <= CLIP_COMPOSITION_LIMITS.stages,
    canAdd: ({ context, event }) =>
      event.type === 'add' &&
      !!event.name.trim() &&
      checkedStages(context).length < CLIP_COMPOSITION_LIMITS.stages,
    headings: ({ context }) =>
      checkedStages(context).every(
        (stage) =>
          !!stage.heading.trim() && length(stage.heading) <= CLIP_COMPOSITION_LIMITS.labelChars,
      ),
    moreStages: ({ context }) =>
      !!current(context) &&
      !stageProblem(current(context)!) &&
      context.cursor < checkedStages(context).length - 1,
    validStage: ({ context }) => !!current(context) && !stageProblem(current(context)!),
    earlierStage: ({ context }) => context.cursor > 0,
    name: ({ context }) => !nameProblem(context.name),
  },
  actions: {
    narrow: assign(({ context, event }) =>
      event.type === 'choose'
        ? { path: choice(context, event.key).path, depth: context.depth + 1 }
        : {},
    ),
    reachLeaf: assign(({ context, event }) => {
      if (event.type !== 'choose') return {}
      const { path } = choice(context, event.key)
      const leaf = path.join('/')
      return leaf === context.leaf
        ? { path, depth: path.length - 1, attempted: false }
        : {
            path,
            depth: path.length - 1,
            leaf,
            stages: (event.seeds ?? []).map((stage) => ({ ...stage })),
            cursor: 0,
            customCount: 0,
            attempted: false,
          }
    }),
    earlierQuestion: assign(({ context }) => ({ depth: context.depth - 1 })),
    lastQuestion: assign(({ context }) => ({
      depth: Math.max(0, context.path.length - 1),
      attempted: false,
    })),
    toggle: assign(({ context, event }) =>
      event.type === 'toggle'
        ? {
            stages: context.stages.map((stage) =>
              stage.id === event.id ? { ...stage, checked: event.checked } : stage,
            ),
          }
        : {},
    ),
    add: assign(({ context, event }) => {
      if (event.type !== 'add') return {}
      const count = context.customCount + 1,
        name = event.name.trim()
      return {
        customCount: count,
        attempted: false,
        stages: [
          ...context.stages,
          {
            id: `custom-${count}`,
            role: 'custom' as const,
            name,
            heading: name,
            intent: event.intent.trim() || name,
            checked: true,
            asks: false,
            fieldLabel: name,
          },
        ],
      }
    }),
    remove: assign(({ context, event }) =>
      event.type === 'remove' && event.id.startsWith('custom-')
        ? { stages: context.stages.filter((stage) => stage.id !== event.id) }
        : {},
    ),
    reorder: assign(({ context, event }) => {
      if (event.type !== 'reorder') return {}
      const selected = checkedStages(context)
      if (event.from < 0 || event.from >= selected.length) return {}
      const [moved] = selected.splice(event.from, 1)
      selected.splice(Math.max(0, Math.min(selected.length, event.to)), 0, moved!)
      return { stages: [...selected, ...context.stages.filter((stage) => !stage.checked)] }
    }),
    edit: assign(({ context, event }) =>
      event.type === 'edit'
        ? {
            stages: context.stages.map((stage) =>
              stage.id === event.id ? { ...stage, ...event.patch } : stage,
            ),
          }
        : {},
    ),
    design: assign(({ event }) =>
      event.type === 'design'
        ? {
            ...event.patch,
            ...(event.patch.allowedCaptionStyles
              ? { allowedCaptionStyles: [...event.patch.allowedCaptionStyles] }
              : {}),
          }
        : {},
    ),
    name: assign(({ event }) => (event.type === 'name' ? { name: event.value } : {})),
    attempt: assign({ attempted: true }),
    settle: assign({ attempted: false }),
    firstStage: assign({ cursor: 0, attempted: false }),
    nextStage: assign(({ context }) => ({ cursor: context.cursor + 1, attempted: false })),
    previousStage: assign(({ context }) => ({ cursor: context.cursor - 1, attempted: false })),
    lastStage: assign(({ context }) => ({
      cursor: Math.max(0, checkedStages(context).length - 1),
      attempted: false,
    })),
  },
}).createMachine({
  id: 'clipTemplateWizard',
  initial: 'target',
  context: {
    path: [],
    depth: 0,
    leaf: '',
    stages: [],
    cursor: 0,
    customCount: 0,
    attempted: false,
    ...emptyClipRecipe(),
  },
  states: {
    target: {
      on: {
        choose: [
          { guard: 'leaf', target: 'stages', actions: 'reachLeaf' },
          { guard: 'branch', actions: 'narrow' },
        ],
        back: { guard: 'earlierQuestion', actions: 'earlierQuestion' },
      },
    },
    stages: {
      on: {
        toggle: { actions: 'toggle' },
        add: [{ guard: 'canAdd', actions: 'add' }, { actions: 'attempt' }],
        remove: { actions: 'remove' },
        next: [
          { guard: 'validSelection', target: 'order', actions: 'settle' },
          { actions: 'attempt' },
        ],
        back: { target: 'target', actions: 'lastQuestion' },
      },
    },
    order: {
      on: {
        reorder: { actions: 'reorder' },
        next: { target: 'headings' },
        back: { target: 'stages' },
      },
    },
    headings: {
      on: {
        edit: { actions: 'edit' },
        next: [
          { guard: 'headings', target: 'stage', actions: 'firstStage' },
          { actions: 'attempt' },
        ],
        back: { target: 'order', actions: 'settle' },
      },
    },
    stage: {
      on: {
        edit: { actions: 'edit' },
        next: [
          { guard: 'moreStages', target: 'stage', actions: 'nextStage' },
          { guard: 'validStage', target: 'design', actions: 'settle' },
          { actions: 'attempt' },
        ],
        back: [
          { guard: 'earlierStage', target: 'stage', actions: 'previousStage' },
          { target: 'headings', actions: 'settle' },
        ],
      },
    },
    design: {
      on: {
        design: { actions: 'design' },
        next: { target: 'finish', actions: 'settle' },
        back: { target: 'stage', actions: 'lastStage' },
      },
    },
    finish: {
      on: {
        name: { actions: 'name' },
        next: [{ guard: 'name', actions: 'settle' }, { actions: 'attempt' }],
        back: { target: 'design', actions: 'settle' },
      },
    },
  },
})
const views = new WeakMap<object, ClipWizardState>()
export function clipWizardStateOf(
  snapshot: SnapshotFrom<typeof clipWizardMachine>,
): ClipWizardState {
  let view = views.get(snapshot)
  if (!view) {
    view = { ...snapshot.context, step: snapshot.value as Step }
    views.set(snapshot, view)
  }
  return view
}
export const initialClipWizardState = () => clipWizardStateOf(getInitialSnapshot(clipWizardMachine))
export function clipWizardTransition(state: ClipWizardState, event: ClipWizardEvent) {
  const { step, ...context } = state
  return clipWizardStateOf(
    getNextSnapshot(
      clipWizardMachine,
      clipWizardMachine.resolveState({ value: step, context }),
      event,
    ),
  )
}
