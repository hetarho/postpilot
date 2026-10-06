import {
  AuthoringEditor,
  AuthoringSheet,
  AuthoringPreview,
  type AuthoringEditorProps,
  type AuthoringSheetProps,
} from '@/features/ai-authoring'
export type AIAuthoringStudioProps = Omit<AuthoringEditorProps, 'renderPreview'>
export type AIAuthoringSheetProps = Omit<AuthoringSheetProps, 'renderPreview'>
export function AIAuthoringStudio(props: AIAuthoringStudioProps) {
  return (
    <AuthoringEditor
      {...props}
      renderPreview={(artifact) => <AuthoringPreview kind={props.kind} artifact={artifact} />}
    />
  )
}
export function AIAuthoringSheet(props: AIAuthoringSheetProps) {
  return (
    <AuthoringSheet
      {...props}
      renderPreview={(artifact) => <AuthoringPreview kind={props.kind} artifact={artifact} />}
    />
  )
}
