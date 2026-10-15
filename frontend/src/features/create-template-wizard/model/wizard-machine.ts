import { assign, getInitialSnapshot, getNextSnapshot, setup, type SnapshotFrom } from 'xstate'
import {
  TEMPLATE_ASK_LABEL_MAX_CHARS,
  TEMPLATE_ASK_MAX_PER_BODY,
  TEMPLATE_DESCRIPTION_MAX_CHARS,
  TEMPLATE_NAME_MAX_CHARS,
  TEMPLATE_PHOTO_ROW_MAX,
} from '@/entities/template'
import { templateTargetAt, type RoleKey, type SectionContent } from '@/entities/template'

/** One section as the owner shapes it (TMPL-71, TMPL-74). Every answer starts at the catalog
 *  default, so 다음 always works until the owner changes something into an invalid state. */
export interface SectionDraft {
  /** The role's key, or `custom-<n>` for one the owner added. */
  id: string
  role?: RoleKey
  /** What the section is called in the checklist, and what it covers in one line. */
  name: string
  help: string
  checked: boolean
  heading: string
  content: SectionContent
  /** The 데이터 받기 field's title, for 직접 입력. */
  question: string
  required: boolean
  /** The fixed text, for 고정 문구. */
  literal: string
  photos: number
  repeatable: boolean
  repeat: boolean
}

export type WizardStep = 'target' | 'sections' | 'order' | 'headings' | 'section' | 'finish'

export interface WizardState {
  step: WizardStep
  /** The choices made, root first. On the target step `depth` says which question is on screen,
   *  so Back can show an earlier question with its answer still chosen. */
  path: readonly string[]
  depth: number
  /** The leaf the sections were made for; a different leaf makes them anew. */
  leaf: string
  sections: readonly SectionDraft[]
  /** Which checked section the section step is on. */
  cursor: number
  name: string
  description: string
  /** Set by a refused 다음, so the screen explains what is missing beside its input (THEME-54). */
  attempted: boolean
  customCount: number
  /** Only the selected post's headings, frozen before the owner starts answering. */
  seedHeadings: readonly string[]
}

export type WizardEvent =
  /** A target answer. The seeds are the leaf's sections, resolved by the screen in the owner's
   *  language, and are read only when the choice reaches a leaf. */
  | { type: 'choose'; key: string; seeds?: readonly SectionDraft[] }
  | { type: 'back' | 'next' }
  | { type: 'toggle'; id: string; checked: boolean }
  | { type: 'add'; name: string; help: string }
  | { type: 'remove'; id: string }
  | { type: 'reorder'; from: number; to: number }
  | { type: 'edit'; id: string; patch: Partial<SectionDraft> }
  | { type: 'name'; value: string }
  | { type: 'description'; value: string }

type WizardContext = Omit<WizardState, 'step'>

function customSection(count: number, name: string, help: string): SectionDraft {
  return {
    id: `custom-${count}`,
    name,
    help,
    checked: true,
    heading: name,
    content: 'write',
    question: name,
    required: false,
    literal: '',
    photos: 0,
    repeatable: false,
    repeat: false,
  }
}

const chars = (value: string) => Array.from(value).length

export function checkedSections(state: Pick<WizardState, 'sections'>): SectionDraft[] {
  return state.sections.filter((section) => section.checked)
}

/** What is wrong with one section's answers, if anything; the screen words it. */
export type SectionProblem =
  | 'heading_missing'
  | 'heading_too_long'
  | 'question_missing'
  | 'question_too_long'
  | 'question_repeated'
  | 'too_many_questions'
  | 'literal_missing'

export function sectionProblem(
  section: SectionDraft,
  sections: readonly SectionDraft[],
): SectionProblem | undefined {
  const heading = section.heading.trim()
  if (heading === '') return 'heading_missing'
  if (chars(heading) > TEMPLATE_ASK_LABEL_MAX_CHARS) return 'heading_too_long'
  if (section.content === 'ask') {
    const question = section.question.trim()
    if (question === '') return 'question_missing'
    if (chars(question) > TEMPLATE_ASK_LABEL_MAX_CHARS) return 'question_too_long'
    const asking = sections.filter((other) => other.checked && other.content === 'ask')
    if (asking.some((other) => other.id !== section.id && other.question.trim() === question))
      return 'question_repeated'
    if (asking.length > TEMPLATE_ASK_MAX_PER_BODY) return 'too_many_questions'
  }
  if (section.content === 'literal' && section.literal.trim() === '') return 'literal_missing'
  return undefined
}

export type FinishProblem = 'name_missing' | 'name_too_long' | 'description_too_long'

export function finishProblem(state: Pick<WizardState, 'name' | 'description'>) {
  const name = state.name.trim()
  if (name === '') return 'name_missing' as const
  if (chars(name) > TEMPLATE_NAME_MAX_CHARS) return 'name_too_long' as const
  if (chars(state.description.trim()) > TEMPLATE_DESCRIPTION_MAX_CHARS)
    return 'description_too_long' as const
  return undefined
}

function current(context: WizardContext): SectionDraft | undefined {
  return checkedSections(context)[context.cursor]
}

function headingsValid(context: WizardContext): boolean {
  return checkedSections(context).every((section) => {
    const heading = section.heading.trim()
    return heading !== '' && chars(heading) <= TEMPLATE_ASK_LABEL_MAX_CHARS
  })
}

/** Where a choice lands: the node it names, and whether that ends the narrowing. */
function chosen(context: WizardContext, key: string) {
  const path = [...context.path.slice(0, context.depth), key]
  const node = templateTargetAt(path)
  return { path, node, leaf: Boolean(node?.roles) }
}

/** Photos are clamped to the grammar's row, and only a repeatable role holds a repeat. */
function clampPatch(section: SectionDraft, patch: Partial<SectionDraft>): SectionDraft {
  const next = { ...section, ...patch, id: section.id, role: section.role }
  next.photos = Math.max(0, Math.min(TEMPLATE_PHOTO_ROW_MAX, Math.trunc(next.photos)))
  next.repeat = next.repeatable && next.repeat
  return next
}

export const wizardMachine = setup({
  types: {
    context: {} as WizardContext,
    events: {} as WizardEvent,
    input: {} as { seedHeadings?: readonly string[] } | undefined,
  },
  guards: {
    choiceIsLeaf: ({ context, event }) =>
      event.type === 'choose' && chosen(context, event.key).leaf && (event.seeds?.length ?? 0) > 0,
    choiceIsBranch: ({ context, event }) => {
      if (event.type !== 'choose') return false
      const { node, leaf } = chosen(context, event.key)
      return Boolean(node) && !leaf
    },
    earlierQuestion: ({ context }) => context.depth > 0,
    someChecked: ({ context }) => checkedSections(context).length > 0,
    headingsValid: ({ context }) => headingsValid(context),
    sectionValid: ({ context }) => {
      const section = current(context)
      return section !== undefined && sectionProblem(section, context.sections) === undefined
    },
    sectionValidWithMore: ({ context }) => {
      const section = current(context)
      return (
        section !== undefined &&
        sectionProblem(section, context.sections) === undefined &&
        context.cursor < checkedSections(context).length - 1
      )
    },
    earlierSection: ({ context }) => context.cursor > 0,
    finishValid: ({ context }) => finishProblem(context) === undefined,
  },
  actions: {
    narrow: assign(({ context, event }) => {
      if (event.type !== 'choose') return {}
      return { path: chosen(context, event.key).path, depth: context.depth + 1 }
    }),
    reachLeaf: assign(({ context, event }) => {
      if (event.type !== 'choose') return {}
      const { path } = chosen(context, event.key)
      const leaf = path.join('/')
      // The same leaf again keeps every answer given for it; another leaf has other sections.
      if (leaf === context.leaf) return { path, depth: path.length - 1, attempted: false }
      return {
        path,
        depth: path.length - 1,
        leaf,
        sections: [
          ...(event.seeds ?? []).map((seed) => ({ ...seed })),
          ...context.seedHeadings.map((heading, index) => customSection(index + 1, heading, '')),
        ],
        customCount: context.seedHeadings.length,
        cursor: 0,
        attempted: false,
      }
    }),
    earlierQuestion: assign(({ context }) => ({ depth: context.depth - 1 })),
    lastQuestion: assign(({ context }) => ({ depth: Math.max(0, context.path.length - 1) })),
    toggle: assign(({ context, event }) =>
      event.type === 'toggle'
        ? {
            sections: context.sections.map((section) =>
              section.id === event.id ? { ...section, checked: event.checked } : section,
            ),
          }
        : {},
    ),
    add: assign(({ context, event }) => {
      if (event.type !== 'add') return {}
      const name = event.name.trim()
      const count = context.customCount + 1
      return {
        customCount: count,
        sections: [...context.sections, customSection(count, name, event.help.trim())],
      }
    }),
    remove: assign(({ context, event }) =>
      event.type === 'remove' && event.id.startsWith('custom-')
        ? { sections: context.sections.filter((section) => section.id !== event.id) }
        : {},
    ),
    reorder: assign(({ context, event }) => {
      if (event.type !== 'reorder') return {}
      // The order step moves the checked sections; an unchecked one keeps its place behind them.
      const checked = checkedSections(context)
      if (event.from < 0 || event.from >= checked.length) return {}
      const next = [...checked]
      const [moved] = next.splice(event.from, 1)
      next.splice(Math.max(0, Math.min(next.length, event.to)), 0, moved!)
      return { sections: [...next, ...context.sections.filter((section) => !section.checked)] }
    }),
    edit: assign(({ context, event }) =>
      event.type === 'edit'
        ? {
            sections: context.sections.map((section) =>
              section.id === event.id ? clampPatch(section, event.patch) : section,
            ),
          }
        : {},
    ),
    name: assign(({ event }) => (event.type === 'name' ? { name: event.value } : {})),
    description: assign(({ event }) =>
      event.type === 'description' ? { description: event.value } : {},
    ),
    attempt: assign({ attempted: true }),
    settle: assign({ attempted: false }),
    firstSection: assign({ cursor: 0, attempted: false }),
    nextSection: assign(({ context }) => ({ cursor: context.cursor + 1, attempted: false })),
    previousSection: assign(({ context }) => ({ cursor: context.cursor - 1, attempted: false })),
    lastSection: assign(({ context }) => ({
      cursor: Math.max(0, checkedSections(context).length - 1),
      attempted: false,
    })),
  },
}).createMachine({
  id: 'templateWizard',
  initial: 'target',
  context: ({ input }) => ({
    path: [],
    depth: 0,
    leaf: '',
    sections: [],
    cursor: 0,
    name: '',
    description: '',
    attempted: false,
    customCount: 0,
    seedHeadings: [...(input?.seedHeadings ?? [])],
  }),
  states: {
    target: {
      on: {
        choose: [
          { guard: 'choiceIsLeaf', target: 'sections', actions: 'reachLeaf' },
          { guard: 'choiceIsBranch', actions: 'narrow' },
        ],
        back: { guard: 'earlierQuestion', actions: 'earlierQuestion' },
      },
    },
    sections: {
      on: {
        toggle: { actions: 'toggle' },
        add: { actions: 'add' },
        remove: { actions: 'remove' },
        next: [
          { guard: 'someChecked', target: 'order', actions: 'settle' },
          { actions: 'attempt' },
        ],
        back: { target: 'target', actions: ['lastQuestion', 'settle'] },
      },
    },
    order: {
      on: {
        reorder: { actions: 'reorder' },
        next: { target: 'headings' },
        back: { target: 'sections' },
      },
    },
    headings: {
      on: {
        edit: { actions: 'edit' },
        next: [
          { guard: 'headingsValid', target: 'section', actions: 'firstSection' },
          { actions: 'attempt' },
        ],
        back: { target: 'order', actions: 'settle' },
      },
    },
    section: {
      on: {
        edit: { actions: 'edit' },
        next: [
          { guard: 'sectionValidWithMore', target: 'section', actions: 'nextSection' },
          { guard: 'sectionValid', target: 'finish', actions: 'settle' },
          { actions: 'attempt' },
        ],
        back: [
          { guard: 'earlierSection', target: 'section', actions: 'previousSection' },
          { target: 'headings', actions: 'settle' },
        ],
      },
    },
    finish: {
      on: {
        name: { actions: 'name' },
        description: { actions: 'description' },
        next: [{ guard: 'finishValid', actions: 'settle' }, { actions: 'attempt' }],
        back: { target: 'section', actions: 'lastSection' },
      },
    },
  },
})

const projections = new WeakMap<object, WizardState>()

export function wizardStateOf(snapshot: SnapshotFrom<typeof wizardMachine>): WizardState {
  let state = projections.get(snapshot)
  if (!state) {
    state = { ...snapshot.context, step: snapshot.value as WizardStep }
    projections.set(snapshot, state)
  }
  return state
}

export function initialWizardState(): WizardState {
  return wizardStateOf(getInitialSnapshot(wizardMachine, {}))
}

/** One guarded step: an event the current step does not take changes nothing. */
export function wizardTransition(state: WizardState, event: WizardEvent): WizardState {
  const { step, ...context } = state
  const snapshot = wizardMachine.resolveState({ value: step, context })
  const next = getNextSnapshot(wizardMachine, snapshot, event)
  return next.context === snapshot.context && next.value === snapshot.value
    ? state
    : wizardStateOf(next)
}

/** Whether the owner has answered anything, which is what leaving asks about (TMPL-71). */
export function wizardHasAnswers(state: WizardState): boolean {
  return state.path.length > 0
}

/** Where the owner is once the target is known: the screens after it, counted. */
export function wizardProgress(state: WizardState): { current: number; total: number } | undefined {
  if (state.step === 'target') return undefined
  const sections = checkedSections(state).length
  const total = 3 + sections + 1
  const index =
    state.step === 'sections'
      ? 1
      : state.step === 'order'
        ? 2
        : state.step === 'headings'
          ? 3
          : state.step === 'section'
            ? 4 + state.cursor
            : total
  return { current: index, total }
}
