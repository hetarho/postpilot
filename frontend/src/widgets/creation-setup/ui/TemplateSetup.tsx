import type { ReactNode } from 'react'
import type { SetupController } from '@/features/complete-setup'

/** What a setup template question shows: the same conversation that makes a template anywhere
 *  else (the owner says what they post, the AI drafts it, they refine it by talking and save it).
 *  The setup page composes it, since a widget cannot hold another widget; setup only learns that
 *  the template was saved. */
export type SetupTemplateRenderer = (props: {
  ownerId: string
  kind: 'post-template' | 'video-template'
  onSaved: () => void
}) => ReactNode

/** Both template kinds confirm a single canonical save before setup advances: the conversation's
 *  own 저장 publishes the template, and that confirmed save is the step's answer. */
export function TemplateSetup({
  ownerId,
  kind,
  controller,
  render,
}: {
  ownerId: string
  kind: 'post-template' | 'clip-template'
  controller: SetupController
  render: SetupTemplateRenderer
}) {
  return (
    <div className="flex min-w-0 flex-1 flex-col">
      {render({
        ownerId,
        kind: kind === 'post-template' ? 'post-template' : 'video-template',
        onSaved: () => {
          const operation = controller.begin(kind)
          if (operation !== null) controller.success(kind, operation, true)
        },
      })}
    </div>
  )
}
