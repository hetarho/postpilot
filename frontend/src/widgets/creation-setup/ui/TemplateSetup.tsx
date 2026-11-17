import { useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { ArrowLeft } from 'lucide-react'
import { AuthoringEditor, AuthoringPreview } from '@/features/ai-authoring'
import { ClipTemplateWizard } from '@/features/create-clip-template-wizard'
import type { SetupController } from '@/features/complete-setup'
import { TemplateDirectEditor } from '@/features/edit-template'
import { Button } from '@/shared/ui'

/** Both template kinds confirm a single canonical save before setup advances.
 *
 *  A post template is made the way it is everywhere else: by talking to the AI (owner
 *  2026-11-17) — the same conversation the template's own screen holds, its saved template
 *  advancing setup. Setup's way back and skip stand above it, so the conversation keeps its own
 *  dock. A video template still asks its wizard's questions, whose step bar carries setup's way
 *  back and skip (`host`). */
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
  if (kind === 'post-template')
    return <PostTemplateSetup ownerId={ownerId} controller={controller} host={host} />
  return (
    <ClipTemplateWizard
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

function PostTemplateSetup({
  ownerId,
  controller,
  host,
}: {
  ownerId: string
  controller: SetupController
  host: { back: () => void; backDisabled?: boolean; footer?: ReactNode }
}) {
  const { t } = useTranslation('creation')
  return (
    <div className="mt-6 flex min-w-0 flex-col gap-6 md:mt-8">
      <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
        <Button variant="ghost" disabled={host.backDisabled} onClick={host.back} className="-ml-3">
          <ArrowLeft aria-hidden="true" className="size-4" />
          {t('setup.back')}
        </Button>
        <div className="w-auto">{host.footer}</div>
      </div>
      <AuthoringEditor
        key={ownerId}
        ownerId={ownerId}
        kind="post-template"
        layout="conversation"
        candidateCount={1}
        startFromSaved
        renderPreview={(artifact, lastValid) => (
          <AuthoringPreview kind="post-template" artifact={artifact} fallbackArtifact={lastValid} />
        )}
        renderDirectEditor={(props) => <TemplateDirectEditor {...props} />}
        onSaved={() => {
          // The save is the server's, already made: setup records it as this step's answer.
          const operation = controller.begin('post-template')
          if (operation !== null) controller.success('post-template', operation, true)
        }}
      />
    </div>
  )
}
