import { useRef, type ReactNode } from 'react'
import { TemplateWizard } from '@/features/create-template-wizard'
import { ClipTemplateWizard } from '@/features/create-clip-template-wizard'
import type { SetupController } from '@/features/complete-setup'

/** Both template kinds confirm a single canonical save before setup advances. The wizard's step
 *  bar is the screen's one bar: setup's way back stands in it on the wizard's first question, and
 *  setup's skip under its row (`host`). */
export function TemplateSetup({
  ownerId,
  kind,
  controller,
  host,
}: {
  ownerId: string
  kind: 'post-template' | 'clip-template'
  controller: SetupController
  host: { back: () => void; backDisabled?: boolean; footer?: ReactNode }
}) {
  const operation = useRef<number | null>(null)
  const Wizard = kind === 'post-template' ? TemplateWizard : ClipTemplateWizard
  return (
    <Wizard
      key={ownerId}
      ownerId={ownerId}
      host={host}
      beforeSave={() => {
        operation.current = controller.begin(kind)
        return operation.current !== null
      }}
      onSaved={() => {
        if (operation.current !== null) controller.success(kind, operation.current, true)
        operation.current = null
      }}
      onSaveFailure={() => {
        if (operation.current !== null) controller.failure(kind, operation.current)
        operation.current = null
      }}
    />
  )
}
