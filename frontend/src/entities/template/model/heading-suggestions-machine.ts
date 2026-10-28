import { assign, fromPromise, setup } from 'xstate'
import {
  HEADING_CHOICES_PER_SECTION,
  HEADING_STYLE_REQUEST_MAX_CHARS,
  HEADING_SUGGESTION_SECTIONS_MAX,
  TEMPLATE_ASK_LABEL_MAX_CHARS,
} from '../config'
import type { HeadingSuggestionInput } from './heading-suggestions'
import { appFailureFromConnect, normalizeAppFailure, type AppFailure } from '@/shared/api'

interface Input {
  ownerId: string
  maxChars?: number
  start: (request: HeadingSuggestionInput) => Promise<string>
}
interface Context extends Input {
  request: HeadingSuggestionInput | null
  signature: string
  jobId: string
  groups: string[][]
  failure: AppFailure | undefined
}
type Event =
  | { type: 'start'; ownerId: string; signature: string; request: HeadingSuggestionInput }
  | {
      type: 'terminal'
      jobId: string
      status: 'done' | 'failed' | 'cancelled'
      failure?: AppFailure
    }
  | { type: 'result'; jobId: string; groups: string[][] }

const invalidOutput = () => normalizeAppFailure({ reason: 'MODEL_OUTPUT_INVALID', params: {} })
const startTransition = { guard: 'canStart', target: 'starting', actions: 'freeze' } as const

/** Transition authority for one explicit action: duplicate presses and stale job messages cannot
 *  issue another call or publish a partial result. Stopping the page actor never cancels the job. */
export const headingSuggestionsMachine = setup({
  types: { context: {} as Context, input: {} as Input, events: {} as Event },
  actors: {
    startRequest: fromPromise(
      async ({ input }: { input: { start: Input['start']; request: HeadingSuggestionInput } }) => {
        const id = await input.start(input.request)
        if (!id.trim()) throw new Error('No heading job was returned')
        return id
      },
    ),
  },
  guards: {
    canStart: ({ context, event }) =>
      event.type === 'start' &&
      event.ownerId === context.ownerId &&
      event.request.sections.length > 0 &&
      event.request.sections.length <= HEADING_SUGGESTION_SECTIONS_MAX &&
      Array.from(event.request.styleRequest ?? '').length <= HEADING_STYLE_REQUEST_MAX_CHARS,
    done: ({ context, event }) =>
      event.type === 'terminal' && event.jobId === context.jobId && event.status === 'done',
    terminalMatches: ({ context, event }) =>
      event.type === 'terminal' && event.jobId === context.jobId,
    validResult: ({ context, event }) =>
      event.type === 'result' &&
      event.jobId === context.jobId &&
      event.groups.length === context.request?.sections.length &&
      event.groups.every(
        (group) =>
          group.length === HEADING_CHOICES_PER_SECTION &&
          new Set(group.map((value) => value.trim())).size === HEADING_CHOICES_PER_SECTION &&
          group.every(
            (value) =>
              value.trim() !== '' &&
              Array.from(value.trim()).length <=
                (context.maxChars ?? TEMPLATE_ASK_LABEL_MAX_CHARS) &&
              !/[\r\n]/.test(value),
          ),
      ),
    resultMatches: ({ context, event }) => event.type === 'result' && event.jobId === context.jobId,
  },
  actions: {
    freeze: assign(({ event }) =>
      event.type === 'start'
        ? {
            request: {
              ...event.request,
              writeModel: { ...event.request.writeModel },
              targetPath: [...event.request.targetPath],
              sections: event.request.sections.map((section) => ({ ...section })),
            },
            signature: event.signature,
            jobId: '',
            groups: [],
            failure: undefined,
          }
        : {},
    ),
    terminalFailure: assign(({ event }) => ({
      failure: event.type === 'terminal' ? (event.failure ?? invalidOutput()) : invalidOutput(),
    })),
    accept: assign(({ event }) =>
      event.type === 'result'
        ? { groups: event.groups.map((group) => group.map((value) => value.trim())) }
        : {},
    ),
    invalid: assign({ failure: invalidOutput }),
  },
}).createMachine({
  id: 'templateHeadingSuggestions',
  initial: 'idle',
  context: ({ input }) => ({
    ...input,
    request: null,
    signature: '',
    jobId: '',
    groups: [],
    failure: undefined,
  }),
  states: {
    idle: { on: { start: startTransition } },
    starting: {
      invoke: {
        src: 'startRequest',
        input: ({ context }) => ({ start: context.start, request: context.request! }),
        onDone: { target: 'running', actions: assign(({ event }) => ({ jobId: event.output })) },
        onError: {
          target: 'failed',
          actions: assign(({ event }) => ({ failure: appFailureFromConnect(event.error) })),
        },
      },
    },
    running: {
      on: {
        terminal: [
          { guard: 'done', target: 'reading' },
          { guard: 'terminalMatches', target: 'failed', actions: 'terminalFailure' },
        ],
      },
    },
    reading: {
      on: {
        result: [
          { guard: 'validResult', target: 'done', actions: 'accept' },
          { guard: 'resultMatches', target: 'failed', actions: 'invalid' },
        ],
      },
    },
    done: { on: { start: startTransition } },
    failed: { on: { start: startTransition } },
  },
})
