import { assign, fromPromise, setup } from 'xstate'
import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import type { VoiceAnalysisEstimate, VoiceProfile } from '@/entities/voice'
interface ModelRef {
  providerId: string
  modelId: string
}
interface AnalysisServices {
  estimate: (model: ModelRef, signal: AbortSignal) => Promise<VoiceAnalysisEstimate>
  analyze: (model: ModelRef) => Promise<string>
  read: () => Promise<VoiceProfile>
}
interface Input {
  scopeKey: string
  runtime: { current: AnalysisServices }
}
interface Context extends Input {
  model?: ModelRef
  versionKey: string
  quote?: VoiceAnalysisEstimate
  jobId: string
  failure?: AppFailure
}
type Event = { scopeKey: string } & (
  | { type: 'ESTIMATE'; model: ModelRef; versionKey: string }
  | { type: 'CONFIRM'; versionKey: string; available: boolean }
  | { type: 'CANCEL' | 'RECHECK' }
  | { type: 'FACTS'; versionKey: string; available: boolean; activeJobId?: string }
  | { type: 'JOB_TERMINAL'; jobId: string }
)
const scoped = ({ context, event }: { context: Context; event: Event }) =>
  !!context.scopeKey && context.scopeKey === event.scopeKey
export const analysisConfirmMachine = setup({
  types: { context: {} as Context, input: {} as Input, events: {} as Event },
  actors: {
    estimate: fromPromise(async ({ input, signal }: { input: Context; signal: AbortSignal }) => {
      if (!input.model) throw new Error('No prepared analysis model')
      const quote = await input.runtime.current.estimate(input.model, signal)
      if (signal.aborted) throw new Error('Obsolete analysis estimate')
      return quote
    }),
    read: fromPromise(async ({ input, signal }: { input: Context; signal: AbortSignal }) => {
      const profile = await input.runtime.current.read()
      if (signal.aborted) throw new Error('Obsolete voice state')
      return profile
    }),
    start: fromPromise(async ({ input, signal }: { input: Context; signal: AbortSignal }) => {
      if (!input.model) throw new Error('No confirmed analysis model')
      const jobId = await input.runtime.current.analyze(input.model)
      if (signal.aborted || !jobId) throw new Error('Unconfirmed analysis admission')
      return jobId
    }),
  },
  guards: { scoped },
}).createMachine({
  id: 'voiceAnalysisConfirmation',
  initial: 'idle',
  context: ({ input }) => ({ ...input, versionKey: '', jobId: '' }),
  states: {
    idle: {
      on: {
        ESTIMATE: {
          guard: (args) =>
            scoped(args) && !!args.event.model.providerId && !!args.event.model.modelId,
          target: 'quoting',
          actions: assign({
            model: ({ event }) => ({ ...event.model }),
            versionKey: ({ event }) => event.versionKey,
            quote: undefined,
            failure: undefined,
          }),
        },
      },
    },
    quoting: {
      tags: ['busy'],
      invoke: {
        src: 'estimate',
        input: ({ context }) => context,
        onDone: { target: 'quoted', actions: assign({ quote: ({ event }) => event.output }) },
        onError: {
          target: 'idle',
          actions: assign({ failure: ({ event }) => appFailureFromConnect(event.error) }),
        },
      },
      on: {
        FACTS: {
          guard: (args) =>
            scoped(args) &&
            (!args.event.available || args.event.versionKey !== args.context.versionKey),
          target: 'idle',
          actions: assign({ quote: undefined }),
        },
        CANCEL: { guard: 'scoped', target: 'idle' },
      },
    },
    quoted: {
      on: {
        CONFIRM: {
          guard: (args) =>
            scoped(args) &&
            args.event.available &&
            args.event.versionKey === args.context.versionKey &&
            !!args.context.quote,
          target: 'starting',
        },
        CANCEL: { guard: 'scoped', target: 'idle', actions: assign({ quote: undefined }) },
        FACTS: {
          guard: (args) =>
            scoped(args) &&
            (!args.event.available || args.event.versionKey !== args.context.versionKey),
          target: 'idle',
          actions: assign({ quote: undefined }),
        },
      },
    },
    starting: {
      tags: ['busy'],
      invoke: {
        src: 'start',
        input: ({ context }) => context,
        onDone: { target: 'started', actions: assign({ jobId: ({ event }) => event.output }) },
        onError: [
          {
            guard: ({ event }) =>
              appFailureFromConnect(event.error).reason !== 'UNKNOWN_FAILURE' &&
              !retriableTransportFailure(event.error),
            target: 'idle',
            actions: assign({ failure: ({ event }) => appFailureFromConnect(event.error) }),
          },
          {
            target: 'uncertain',
            actions: assign({ failure: ({ event }) => appFailureFromConnect(event.error) }),
          },
        ],
      },
    },
    started: {
      on: {
        JOB_TERMINAL: {
          guard: (args) => scoped(args) && args.event.jobId === args.context.jobId,
          target: 'idle',
          actions: assign({ jobId: '', quote: undefined }),
        },
      },
    },
    checking: {
      tags: ['busy'],
      invoke: {
        src: 'read',
        input: ({ context }) => context,
        onDone: [
          {
            guard: ({ event }) => !!event.output.activeJobId,
            target: 'started',
            actions: assign({ jobId: ({ event }) => event.output.activeJobId, failure: undefined }),
          },
          { target: 'idle', actions: assign({ quote: undefined, failure: undefined }) },
        ],
        onError: {
          target: 'uncertain',
          actions: assign({ failure: ({ event }) => appFailureFromConnect(event.error) }),
        },
      },
    },
    uncertain: {
      on: {
        RECHECK: { guard: 'scoped', target: 'checking' },
        FACTS: {
          guard: (args) => scoped(args) && !!args.event.activeJobId,
          target: 'started',
          actions: assign({ jobId: ({ event }) => event.activeJobId ?? '', failure: undefined }),
        },
      },
    },
  },
})
