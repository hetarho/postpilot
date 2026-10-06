import { useTranslation } from 'react-i18next'
import { Button, Sheet } from '@/shared/ui'
import { AuthoringEditor, type AuthoringEditorProps } from './AuthoringEditor'

export interface AuthoringSheetProps extends AuthoringEditorProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  label?: string
}
export function AuthoringSheet({ open, onOpenChange, label, ...props }: AuthoringSheetProps) {
  const { t } = useTranslation('authoring')
  if (!open) return null
  return (
    <Sheet
      open
      size="wide"
      label={label ?? t('title', { kind: t(`kinds.${props.kind}`) })}
      onClose={() => onOpenChange(false)}
      header={
        <div className="flex justify-end">
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t('close')}
          </Button>
        </div>
      }
    >
      <AuthoringEditor
        {...props}
        onSaved={(ref) => {
          onOpenChange(false)
          props.onSaved?.(ref)
        }}
      />
    </Sheet>
  )
}
