import { assign, fromPromise, setup } from 'xstate'
import { appFailureFromConnect, type AppFailure } from '@/shared/api'

export interface WizardTemplateFields {
  name: string
  description: string
  body: string
  titleArea: string
}
interface Input {
  ownerId: string
  save: (fields: WizardTemplateFields) => Promise<string>
  onSaved: (id: string) => void
  beforeSave?: () => boolean
  onFailure?: () => void
}
interface Context extends Input {
  fields: WizardTemplateFields | null
  failure: AppFailure | undefined
}
type Event = { type: 'save'; ownerId: string; fields: WizardTemplateFields }

/** A canonical save is admitted once and frozen. A stopped owner actor cannot publish its
 * late result or move setup to another step; interruption never cancels the server write. */
export const saveTemplateMachine = setup({
  types: { context: {} as Context, input: {} as Input, events: {} as Event },
  actors: {
    save: fromPromise(async ({ input }: { input: Input & { fields: WizardTemplateFields } }) => {
      if (input.beforeSave && !input.beforeSave()) throw new Error('Setup cannot admit this save')
      const id = await input.save(input.fields)
      if (!id) throw new Error('No saved template was returned')
      return id
    }),
  },
  guards: {
    owned: ({ context, event }) => !!context.ownerId && event.ownerId === context.ownerId,
  },
}).createMachine({
  id: 'saveWizardTemplate',
  initial: 'idle',
  context: ({ input }) => ({ ...input, fields: null, failure: undefined }),
  states: {
    idle: {
      on: {
        save: {
          guard: 'owned',
          target: 'saving',
          actions: assign(({ event }) => ({ fields: { ...event.fields }, failure: undefined })),
        },
      },
    },
    saving: {
      invoke: {
        src: 'save',
        input: ({ context }) => ({ ...context, fields: context.fields! }),
        onDone: {
          target: 'saved',
          actions: ({ context, event }) => context.onSaved(event.output),
        },
        onError: {
          target: 'idle',
          actions: [
            assign(({ event }) => ({ failure: appFailureFromConnect(event.error) })),
            ({ context }) => context.onFailure?.(),
          ],
        },
      },
    },
    saved: { type: 'final' },
  },
})
