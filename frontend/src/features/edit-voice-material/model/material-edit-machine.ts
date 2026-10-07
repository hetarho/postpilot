import { assign, fromPromise, setup } from 'xstate'
import { appFailureFromConnect, retriableTransportFailure, type AppFailure } from '@/shared/api'
import type { VoiceMaterialUpdate, VoiceSample, VoiceSampleDetail } from '@/entities/voice'
import type { ResizedJpeg } from '@/shared/lib'

export type MaterialPhotoChoice =
  { mode: 'keep' } | { mode: 'remove' } | { mode: 'replace'; photo: ResizedJpeg; preview: string }
export interface MaterialEditDraft {
  label: string
  body: string
  photo: MaterialPhotoChoice
}
export interface MaterialEditServices {
  update: (input: VoiceMaterialUpdate, signal: AbortSignal) => Promise<VoiceSample>
  read: () => Promise<VoiceSampleDetail>
  upload: (
    photo: ResizedJpeg,
    signal: AbortSignal,
  ) => Promise<{ uploadId: string; width: number; height: number }>
}
export interface MaterialEditInput {
  scopeKey: string
  baseline: VoiceSampleDetail
  photoRequired: boolean
  blocked: boolean
  minimumPostChars: number
  services: MaterialEditServices
  requestKey: () => string
}
interface MaterialEditContext extends MaterialEditInput {
  draft: MaterialEditDraft
  command?: VoiceMaterialUpdate
  replacement?: ResizedJpeg
  failure?: AppFailure
  saved?: VoiceSample
  latest?: VoiceSampleDetail
}
export type MaterialEditEvent = { scopeKey: string } & (
  | { type: 'CHANGE'; patch: Partial<MaterialEditDraft> }
  | { type: 'SAVE' | 'CANCEL' | 'RELOAD' | 'RETRY' }
)
const scoped = ({ context, event }: { context: MaterialEditContext; event: MaterialEditEvent }) =>
  !!context.scopeKey && context.scopeKey === event.scopeKey
export function materialDraftProblem(
  context: Pick<
    MaterialEditContext,
    'baseline' | 'draft' | 'photoRequired' | 'blocked' | 'minimumPostChars'
  >,
): 'blocked' | 'revision' | 'body' | 'photo' | undefined {
  if (context.blocked) return 'blocked'
  if (!context.baseline.sample.contentRevision || context.baseline.sample.contentRevision <= 0n)
    return 'revision'
  const chars = Array.from(context.draft.body.trim()).length
  if (context.baseline.sample.kind === 'post' ? chars < context.minimumPostChars : chars === 0)
    return 'body'
  if (
    context.photoRequired &&
    (context.draft.photo.mode === 'remove' ||
      (context.draft.photo.mode === 'keep' && !context.baseline.sample.hasPhoto))
  )
    return 'photo'
  return undefined
}
export function materialDraftChanged(
  context: Pick<MaterialEditContext, 'baseline' | 'draft'>,
): boolean {
  return (
    context.draft.body !== context.baseline.body ||
    (context.baseline.sample.kind === 'post' &&
      context.draft.label !== context.baseline.sample.label) ||
    context.draft.photo.mode !== 'keep'
  )
}
export const materialEditMachine = setup({
  types: {
    context: {} as MaterialEditContext,
    input: {} as MaterialEditInput,
    events: {} as MaterialEditEvent,
  },
  actors: {
    upload: fromPromise(
      async ({ input, signal }: { input: MaterialEditContext; signal: AbortSignal }) => {
        if (!input.replacement) throw new Error('No selected photo')
        const receipt = await input.services.upload(input.replacement, signal)
        if (signal.aborted || !receipt.uploadId || receipt.width <= 0 || receipt.height <= 0)
          throw new Error('The photo upload was not confirmed')
        return receipt
      },
    ),
    save: fromPromise(
      async ({ input, signal }: { input: MaterialEditContext; signal: AbortSignal }) => {
        if (!input.command) throw new Error('No material save intent')
        const saved = await input.services.update(input.command, signal)
        if (
          signal.aborted ||
          saved.id !== input.baseline.sample.id ||
          saved.kind !== input.baseline.sample.kind ||
          saved.promptKey !== input.baseline.sample.promptKey
        )
          throw new Error('Obsolete material acknowledgement')
        return saved
      },
    ),
    read: fromPromise(
      async ({ input, signal }: { input: MaterialEditContext; signal: AbortSignal }) => {
        const detail = await input.services.read()
        if (
          signal.aborted ||
          detail.sample.id !== input.baseline.sample.id ||
          detail.sample.kind !== input.baseline.sample.kind ||
          detail.sample.promptKey !== input.baseline.sample.promptKey
        )
          throw new Error('Obsolete material read')
        return detail
      },
    ),
  },
  guards: {
    scoped,
    canSave: (args) =>
      scoped(args) && !materialDraftProblem(args.context) && materialDraftChanged(args.context),
  },
  actions: {
    change: assign(({ context, event }) =>
      event.type === 'CHANGE'
        ? {
            draft: { ...context.draft, ...event.patch },
            command: undefined,
            replacement: undefined,
            failure: undefined,
          }
        : {},
    ),
    command: assign(({ context }) => ({
      command: {
        sampleId: context.baseline.sample.id,
        expectedContentRevision: context.baseline.sample.contentRevision!,
        operationKey: context.requestKey(),
        ...(context.baseline.sample.kind === 'post' &&
        context.draft.label !== context.baseline.sample.label
          ? { label: context.draft.label }
          : {}),
        ...(context.draft.body !== context.baseline.body ? { body: context.draft.body } : {}),
        ...(context.draft.photo.mode === 'remove'
          ? { photo: { uploadId: '', width: 0, height: 0 } }
          : {}),
      },
      replacement: context.draft.photo.mode === 'replace' ? context.draft.photo.photo : undefined,
      failure: undefined,
    })),
    failure: assign(({ event }) => ({
      failure: appFailureFromConnect('error' in event ? event.error : undefined),
    })),
  },
}).createMachine({
  id: 'voiceMaterialEdit',
  initial: 'editing',
  context: ({ input }) => ({
    ...input,
    draft: {
      label: input.baseline.sample.label,
      body: input.baseline.body,
      photo: { mode: 'keep' },
    },
  }),
  states: {
    editing: {
      on: {
        CHANGE: { guard: 'scoped', actions: 'change' },
        SAVE: [
          {
            guard: (args) =>
              scoped(args) &&
              !materialDraftProblem(args.context) &&
              materialDraftChanged(args.context) &&
              args.context.draft.photo.mode === 'replace',
            actions: 'command',
            target: 'uploading',
          },
          { guard: 'canSave', actions: 'command', target: 'saving' },
        ],
        CANCEL: { guard: 'scoped', target: 'cancelled' },
      },
    },
    uploading: {
      tags: ['busy'],
      invoke: {
        src: 'upload',
        input: ({ context }) => context,
        onDone: {
          target: 'saving',
          actions: assign(({ context, event }) => ({
            command: context.command ? { ...context.command, photo: event.output } : undefined,
          })),
        },
        onError: { target: 'uploadFailed', actions: 'failure' },
      },
    },
    saving: {
      tags: ['busy'],
      invoke: {
        src: 'save',
        input: ({ context }) => context,
        onDone: { target: 'saved', actions: assign({ saved: ({ event }) => event.output }) },
        onError: [
          {
            guard: ({ event }) =>
              appFailureFromConnect(event.error).reason === 'VOICE_SAMPLE_REVISION_CONFLICT',
            target: 'conflict',
            actions: 'failure',
          },
          {
            guard: ({ event }) =>
              appFailureFromConnect(event.error).reason !== 'UNKNOWN_FAILURE' &&
              !retriableTransportFailure(event.error),
            target: 'editing',
            actions: 'failure',
          },
          { target: 'saveFailed', actions: 'failure' },
        ],
      },
    },
    uploadFailed: {
      on: {
        RETRY: { guard: 'scoped', target: 'uploading' },
        CHANGE: { guard: 'scoped', target: 'editing', actions: 'change' },
        CANCEL: { guard: 'scoped', target: 'cancelled' },
      },
    },
    saveFailed: {
      on: {
        RETRY: { guard: 'scoped', target: 'saving' },
        CHANGE: { guard: 'scoped', target: 'editing', actions: 'change' },
        CANCEL: { guard: 'scoped', target: 'cancelled' },
        RELOAD: { guard: 'scoped', target: 'refreshing' },
      },
    },
    conflict: {
      on: {
        RELOAD: { guard: 'scoped', target: 'refreshing' },
        CHANGE: { guard: 'scoped', actions: 'change' },
        CANCEL: { guard: 'scoped', target: 'cancelled' },
      },
    },
    refreshing: {
      tags: ['busy'],
      invoke: {
        src: 'read',
        input: ({ context }) => context,
        onDone: {
          target: 'editing',
          actions: assign(({ context, event }) => ({
            baseline: event.output,
            latest: event.output,
            draft: {
              ...context.draft,
              label:
                context.draft.label === context.baseline.sample.label
                  ? event.output.sample.label
                  : context.draft.label,
              body:
                context.draft.body === context.baseline.body
                  ? event.output.body
                  : context.draft.body,
            },
            command: undefined,
            replacement: undefined,
            failure: undefined,
          })),
        },
        onError: { target: 'conflict', actions: 'failure' },
      },
    },
    saved: { type: 'final' },
    cancelled: { type: 'final' },
  },
})
