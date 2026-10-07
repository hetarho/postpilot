import {
  AuthoringEditor,
  AuthoringSheet,
  AuthoringPreview,
  type AuthoringEditorProps,
  type AuthoringSheetProps,
  type AuthoringInspectionContext,
} from '@/features/ai-authoring'
import { InspectWritingRequestAction } from '@/features/inspect-writing-request'
import { useTranslation } from 'react-i18next'
export type AIAuthoringStudioProps = Omit<
  AuthoringEditorProps,
  'renderPreview' | 'renderInspection'
>
export type AIAuthoringSheetProps = Omit<AuthoringSheetProps, 'renderPreview' | 'renderInspection'>
function AuthoringInspection(context: AuthoringInspectionContext) {
  const { t } = useTranslation('requestInspection')
  return (
    <InspectWritingRequestAction
      target={{
        kind: 'authoring',
        ownerId: context.ownerId,
        sessionId: context.sessionId,
        authoringKind: context.kind,
        revision: context.revision,
        mode: context.mode,
        prompt: context.prompt,
        model: context.model,
        candidateCount: context.candidateCount,
        operationId: context.operationId,
      }}
      stages={['setting-authoring']}
      blockedReason={context.sourceDirty ? t('unsavedSource') : undefined}
      className="mt-4"
    />
  )
}
export function AIAuthoringStudio(props: AIAuthoringStudioProps) {
  return (
    <AuthoringEditor
      {...props}
      renderInspection={(context) => <AuthoringInspection {...context} />}
      renderPreview={(artifact, lastValid) => (
        <AuthoringPreview kind={props.kind} artifact={artifact} fallbackArtifact={lastValid} />
      )}
    />
  )
}
export function AIAuthoringSheet(props: AIAuthoringSheetProps) {
  return (
    <AuthoringSheet
      {...props}
      renderInspection={(context) => <AuthoringInspection {...context} />}
      renderPreview={(artifact, lastValid) => (
        <AuthoringPreview kind={props.kind} artifact={artifact} fallbackArtifact={lastValid} />
      )}
    />
  )
}
