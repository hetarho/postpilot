import { assign, fromPromise, setup } from 'xstate'
import type { Voice, VoiceProfile } from '@/entities/voice'
import type { ModelRef } from '@/entities/generation-job'
import type { AppFailure } from '@/shared/api'
import { appFailureFromConnect } from '@/shared/api'

export type LearningMethod = 'choose' | 'paste' | 'questions' | 'ai' | 'legacy'
export interface LearningServices {
  create: (name: string) => Promise<Voice>
  analyze: (voiceId: string, model: ModelRef) => Promise<string>
  confirm: (voiceId: string) => Promise<Voice>
  read: (voiceId: string) => Promise<VoiceProfile>
}
export interface LearningInput {
  ownerId: string
  initialVoiceId?: string
  initialMethod?: LearningMethod
  name: string
  runtime: { current: LearningServices }
}
interface LearningContext extends LearningInput {
  voiceId: string
  method: LearningMethod
  profile?: VoiceProfile
  jobId: string
  childBusy: boolean
  pasteDraft: { label: string; body: string }
  model?: ModelRef
  failure?: AppFailure
  failureStep: 'create' | 'analysis' | 'confirm' | 'load'
  confirmed?: Voice
  visitedQuestions: boolean
  visitedPaste: boolean
  visitedAI: boolean
  visitedLegacy: boolean
}
type Scope = { ownerId: string }
export type LearningEvent = Scope &
  (
    | { type: 'CHOOSE'; method: Exclude<LearningMethod, 'choose'> }
    | { type: 'PROFILE'; profile: VoiceProfile }
    | { type: 'PROFILE_FAILED'; failure: AppFailure }
    | { type: 'CHILD_BUSY'; busy: boolean }
    | { type: 'PASTE_DRAFT'; label: string; body: string }
    | { type: 'REVIEW' | 'BACK' | 'ADD_MORE' | 'USE' | 'RETRY' }
    | { type: 'ANALYZE'; model: ModelRef }
    | { type: 'JOB_FAILED'; failure?: AppFailure; jobId: string }
    | { type: 'JOB_COMPLETED'; jobId: string }
    | { type: 'RECHECK' }
    | { type: 'EXTERNAL_SAVED'; voiceId: string }
  )
const scoped = ({ context, event }: { context: LearningContext; event: LearningEvent }) =>
  context.ownerId !== '' && event.ownerId === context.ownerId
const profileMatches = (context: LearningContext, profile: VoiceProfile) =>
  !!context.voiceId && profile.voice.id === context.voiceId
export const learningMachine = setup({
  types: {
    context: {} as LearningContext,
    input: {} as LearningInput,
    events: {} as LearningEvent,
  },
  actors: {
    reconcile: fromPromise(
      async ({ input, signal }: { input: LearningContext; signal: AbortSignal }) => {
        const profile = await input.runtime.current.read(input.voiceId)
        if (signal.aborted || profile.voice.id !== input.voiceId)
          throw new Error('Obsolete voice facts')
        return profile
      },
    ),
    createPersonal: fromPromise(
      async ({ input, signal }: { input: LearningContext; signal: AbortSignal }) => {
        const voice = await input.runtime.current.create(input.name)
        if (signal.aborted) throw new Error('Obsolete voice preparation')
        if (!voice.id || voice.deleted) throw new Error('Unconfirmed personal writing voice')
        return voice
      },
    ),
    analyze: fromPromise(
      async ({ input, signal }: { input: LearningContext; signal: AbortSignal }) => {
        if (!input.model || !input.voiceId) throw new Error('Missing analysis input')
        const jobId = await input.runtime.current.analyze(input.voiceId, input.model)
        if (signal.aborted) throw new Error('Obsolete voice analysis')
        if (!jobId) throw new Error('Unconfirmed voice analysis')
        return jobId
      },
    ),
    confirm: fromPromise(
      async ({ input, signal }: { input: LearningContext; signal: AbortSignal }) => {
        const voice = await input.runtime.current.confirm(input.voiceId)
        if (signal.aborted) throw new Error('Obsolete voice selection')
        if (voice.id !== input.voiceId || !voice.made || !voice.isDefault || voice.deleted)
          throw new Error('Unconfirmed default writing voice')
        return voice
      },
    ),
  },
  guards: {
    scoped,
    available: (args) => scoped(args) && !args.context.childBusy,
    ready: (args) =>
      scoped(args) &&
      !args.context.childBusy &&
      !!args.context.voiceId &&
      args.context.profile?.readiness.percent === 100 &&
      !args.context.profile.voice.deleted,
  },
  actions: {
    takeProfile: assign(({ context, event }) =>
      event.type === 'PROFILE' && profileMatches(context, event.profile)
        ? { profile: event.profile }
        : {},
    ),
    takeFailure: assign(({ event }) =>
      event.type === 'PROFILE_FAILED' || event.type === 'JOB_FAILED'
        ? { failure: event.failure ?? { reason: 'UNKNOWN_FAILURE' as const, params: {} } }
        : {},
    ),
  },
}).createMachine({
  id: 'voiceLearning',
  initial: 'routing',
  context: ({ input }) => ({
    ...input,
    voiceId: input.initialVoiceId ?? '',
    method: input.initialMethod ?? 'choose',
    jobId: '',
    childBusy: false,
    pasteDraft: { label: '', body: '' },
    failureStep: 'load',
    visitedQuestions: false,
    visitedPaste: false,
    visitedAI: false,
    visitedLegacy: false,
  }),
  on: {
    CHILD_BUSY: {
      guard: ({ context, event }) => scoped({ context, event }) && context.childBusy !== event.busy,
      actions: assign({ childBusy: ({ event }) => event.busy }),
    },
    PASTE_DRAFT: {
      guard: 'available',
      actions: assign({ pasteDraft: ({ event }) => ({ label: event.label, body: event.body }) }),
    },
    PROFILE: { guard: 'scoped', actions: 'takeProfile' },
    EXTERNAL_SAVED: {
      guard: ({ context, event }) =>
        scoped({ context, event }) &&
        !!event.voiceId &&
        (context.method === 'ai' || context.method === 'legacy'),
      target: '.verifyingExternal',
      actions: assign({
        voiceId: ({ event }) => event.voiceId,
        profile: undefined,
        childBusy: false,
      }),
    },
  },
  states: {
    routing: {
      always: [
        { guard: ({ context }) => context.method === 'ai', target: 'ai' },
        { guard: ({ context }) => context.method === 'legacy', target: 'legacy' },
        { guard: ({ context }) => !!context.voiceId, target: 'personal.checking' },
        { target: 'choose' },
      ],
    },
    choose: {
      on: {
        CHOOSE: [
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && !context.childBusy && event.method === 'ai',
            target: 'ai',
            actions: assign({ method: 'ai' }),
          },
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && !context.childBusy && event.method === 'legacy',
            target: 'legacy',
            actions: assign({ method: 'legacy' }),
          },
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && !context.childBusy && !!context.voiceId,
            target: 'personal.collecting',
            actions: assign({ method: ({ event }) => event.method }),
          },
          {
            guard: 'available',
            target: 'preparingPersonal',
            actions: assign({ method: ({ event }) => event.method, failure: undefined }),
          },
        ],
      },
    },
    preparingPersonal: {
      tags: ['busy'],
      invoke: {
        src: 'createPersonal',
        input: ({ context }) => context,
        onDone: {
          target: 'personal.checking',
          actions: assign({ voiceId: ({ event }) => event.output.id }),
        },
        onError: {
          target: 'failure',
          actions: assign({
            failure: ({ event }) => appFailureFromConnect(event.error),
            failureStep: 'create',
          }),
        },
      },
    },
    ai: {
      entry: assign({ visitedAI: true }),
      on: { BACK: { guard: 'available', target: 'choose' } },
    },
    legacy: {
      entry: assign({ visitedLegacy: true }),
      on: { BACK: { guard: 'available', target: 'choose' } },
    },
    personal: {
      initial: 'checking',
      states: {
        checking: {
          tags: ['busy'],
          on: {
            PROFILE: [
              {
                guard: ({ context, event }) =>
                  scoped({ context, event }) &&
                  profileMatches(context, event.profile) &&
                  (!!event.profile.activeJobId || (!!context.jobId && !event.profile.made)),
                target: 'analyzing.watching',
                actions: [
                  'takeProfile',
                  assign({
                    jobId: ({ context, event }) => event.profile.activeJobId || context.jobId,
                  }),
                ],
              },
              {
                guard: ({ context, event }) =>
                  scoped({ context, event }) &&
                  profileMatches(context, event.profile) &&
                  event.profile.made,
                target: 'confirmed',
                actions: 'takeProfile',
              },
              {
                guard: ({ context, event }) =>
                  scoped({ context, event }) &&
                  profileMatches(context, event.profile) &&
                  event.profile.samples.length > 0,
                target: 'review',
                actions: 'takeProfile',
              },
              {
                guard: ({ context, event }) =>
                  scoped({ context, event }) &&
                  profileMatches(context, event.profile) &&
                  context.method !== 'choose',
                target: 'collecting',
                actions: 'takeProfile',
              },
              {
                guard: ({ context, event }) =>
                  scoped({ context, event }) && profileMatches(context, event.profile),
                target: '#voiceLearning.choose',
                actions: 'takeProfile',
              },
            ],
            PROFILE_FAILED: {
              guard: 'scoped',
              target: '#voiceLearning.failure',
              actions: ['takeFailure', assign({ failureStep: 'load' })],
            },
          },
        },
        collecting: {
          initial: 'route',
          states: {
            route: {
              always: [
                { guard: ({ context }) => context.method === 'paste', target: 'paste' },
                { target: 'questions' },
              ],
            },
            paste: { entry: assign({ visitedPaste: true }) },
            questions: { entry: assign({ visitedQuestions: true }) },
          },
          on: {
            REVIEW: { guard: 'available', target: 'review' },
            BACK: { guard: 'available', target: '#voiceLearning.choose' },
          },
        },
        review: {
          on: {
            ANALYZE: {
              guard: ({ context, event }) =>
                scoped({ context, event }) &&
                !context.childBusy &&
                !!event.model.providerId &&
                !!event.model.modelId &&
                (context.profile?.readiness.percent ?? 0) >= 100 &&
                !context.profile?.voice.deleted &&
                !context.profile?.activeJobId,
              target: 'analyzing.admitting',
              actions: assign({ model: ({ event }) => ({ ...event.model }), failure: undefined }),
            },
            BACK: { guard: 'available', target: 'collecting' },
            ADD_MORE: { guard: 'available', target: 'collecting' },
          },
        },
        analyzing: {
          tags: ['busy'],
          initial: 'admitting',
          states: {
            admitting: {
              invoke: {
                src: 'analyze',
                input: ({ context }) => context,
                onDone: {
                  target: 'watching',
                  actions: assign({ jobId: ({ event }) => event.output }),
                },
                onError: {
                  target: '#voiceLearning.failure',
                  actions: assign({
                    failure: ({ event }) => appFailureFromConnect(event.error),
                    failureStep: 'analysis',
                  }),
                },
              },
            },
            watching: {
              on: {
                PROFILE: {
                  guard: ({ context, event }) =>
                    scoped({ context, event }) &&
                    profileMatches(context, event.profile) &&
                    event.profile.made &&
                    !event.profile.activeJobId,
                  target: '#voiceLearning.personal.confirmed',
                  actions: 'takeProfile',
                },
                RECHECK: { guard: 'scoped', target: '#voiceLearning.reconcilingAnalysis' },
                JOB_COMPLETED: {
                  guard: ({ context, event }) =>
                    scoped({ context, event }) && !!context.jobId && event.jobId === context.jobId,
                  target: '#voiceLearning.reconcilingAnalysis',
                },
                JOB_FAILED: {
                  guard: ({ context, event }) =>
                    scoped({ context, event }) && !!context.jobId && event.jobId === context.jobId,
                  target: '#voiceLearning.failure',
                  actions: ['takeFailure', assign({ failureStep: 'analysis' })],
                },
              },
            },
          },
        },
        confirmed: {
          on: {
            USE: {
              guard: ({ context, event }) =>
                scoped({ context, event }) &&
                !context.childBusy &&
                !!context.profile?.made &&
                !context.profile.voice.deleted,
              target: 'confirming',
              actions: assign({ failure: undefined }),
            },
            BACK: { guard: 'available', target: 'review' },
          },
        },
        confirming: {
          tags: ['busy'],
          invoke: {
            src: 'confirm',
            input: ({ context }) => context,
            onDone: {
              target: '#voiceLearning.done',
              actions: assign({ confirmed: ({ event }) => event.output }),
            },
            onError: {
              target: '#voiceLearning.failure',
              actions: assign({
                failure: ({ event }) => appFailureFromConnect(event.error),
                failureStep: 'confirm',
              }),
            },
          },
        },
      },
    },
    verifyingExternal: {
      tags: ['busy'],
      on: {
        PROFILE: {
          guard: ({ context, event }) =>
            scoped({ context, event }) &&
            profileMatches(context, event.profile) &&
            event.profile.made &&
            !event.profile.voice.deleted,
          target: 'done',
          actions: ['takeProfile', assign({ confirmed: ({ event }) => event.profile.voice })],
        },
        PROFILE_FAILED: {
          guard: 'scoped',
          target: 'failure',
          actions: ['takeFailure', assign({ failureStep: 'load' })],
        },
      },
    },
    reconcilingAnalysis: {
      tags: ['busy'],
      invoke: {
        src: 'reconcile',
        input: ({ context }) => context,
        onDone: [
          {
            guard: ({ event }) => event.output.made && !event.output.activeJobId,
            target: 'personal.confirmed',
            actions: assign({ profile: ({ event }) => event.output, failure: undefined }),
          },
          {
            guard: ({ event }) => !!event.output.activeJobId,
            target: 'personal.analyzing.watching',
            actions: assign({
              profile: ({ event }) => event.output,
              jobId: ({ event }) => event.output.activeJobId,
              failure: undefined,
            }),
          },
          {
            target: 'personal.review',
            actions: assign({ profile: ({ event }) => event.output, failure: undefined }),
          },
        ],
        onError: {
          target: 'failure',
          actions: assign({
            failure: ({ event }) => appFailureFromConnect(event.error),
            failureStep: 'analysis',
          }),
        },
      },
    },
    failure: {
      on: {
        PROFILE: [
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) &&
              context.failureStep === 'analysis' &&
              profileMatches(context, event.profile) &&
              event.profile.made &&
              !event.profile.activeJobId,
            target: 'personal.confirmed',
            actions: ['takeProfile', assign({ failure: undefined })],
          },
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) &&
              context.failureStep === 'analysis' &&
              profileMatches(context, event.profile) &&
              !!event.profile.activeJobId,
            target: 'personal.analyzing.watching',
            actions: [
              'takeProfile',
              assign({ jobId: ({ event }) => event.profile.activeJobId, failure: undefined }),
            ],
          },
          { guard: 'scoped', actions: 'takeProfile' },
        ],
        BACK: [
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && context.failureStep === 'confirm',
            target: 'personal.confirmed',
          },
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && context.failureStep === 'analysis',
            target: 'reconcilingAnalysis',
          },
          { guard: 'available', target: 'choose' },
        ],
        RETRY: [
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && context.failureStep === 'create',
            target: 'preparingPersonal',
          },
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && context.failureStep === 'confirm',
            target: 'personal.confirming',
          },
          {
            guard: ({ context, event }) =>
              scoped({ context, event }) && context.failureStep === 'analysis',
            target: 'reconcilingAnalysis',
          },
          { guard: 'available', target: 'personal.checking' },
        ],
      },
    },
    done: { type: 'final' },
  },
})
