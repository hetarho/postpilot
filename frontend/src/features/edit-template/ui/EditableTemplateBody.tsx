import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { TemplateBodyEditor } from '@/entities/template'
import { Button, Editable, FieldMessage, Typography } from '@/shared/ui'

/** The saved template's body, read-first like its sibling fields (design-language: read
 *  first, edit on request).
 *
 *  The read view shows the source rather than a rendered preview: the source IS the template,
 *  and a preview would be a third representation to keep honest beside the structure editor
 *  and the body itself. */
export function EditableTemplateBody({
  value,
  save,
  errorMessage,
  pending,
  className,
}: {
  value: string
  save: (next: string) => Promise<unknown>
  errorMessage: string
  pending: boolean
  className?: string
}) {
  const { t } = useTranslation(['templates', 'common'])
  const label = t('create.body', { ns: 'templates' })
  return (
    <Editable
      className={className}
      editLabel={t('action.editNamed', { ns: 'common', name: label })}
      edit={(exit) => (
        <BodyEditor
          label={label}
          value={value}
          save={save}
          errorMessage={errorMessage}
          pending={pending}
          exit={exit}
        />
      )}
    >
      <Typography variant="label" as="p">
        {label}
      </Typography>
      {/* `mono` because the literal separator lines in a template are alignment-significant:
          the author sees the same column widths the model will. */}
      <Typography variant="body" mono className="text-content-primary mt-1 whitespace-pre-wrap">
        {value}
      </Typography>
    </Editable>
  )
}

function BodyEditor({
  label,
  value,
  save,
  errorMessage,
  pending,
  exit,
}: {
  label: string
  value: string
  save: (next: string) => Promise<unknown>
  errorMessage: string
  pending: boolean
  exit: () => void
}) {
  const { t } = useTranslation('common')
  // Seeded once, at the mount edit mode gives this editor. It is deliberately not resynced
  // from `value`: while someone is editing here, their draft outranks a value arriving from a
  // refetch or from a sibling field's save.
  const [draft, setDraft] = useState(value)
  const [failed, setFailed] = useState(false)

  const commit = async () => {
    if (pending || draft.trim() === '') return
    try {
      await save(draft)
      setFailed(false)
      exit()
    } catch {
      // Stay open: the message renders below and the draft is still here to fix.
      setFailed(true)
    }
  }

  return (
    <div>
      <Typography variant="label" as="p">
        {label}
      </Typography>
      <TemplateBodyEditor value={draft} onChange={setDraft} disabled={pending} className="mt-1" />
      {failed && Boolean(errorMessage) && (
        <FieldMessage className="mt-2">{errorMessage}</FieldMessage>
      )}
      <div className="mt-3 flex gap-2">
        <Button
          onClick={() => void commit()}
          disabled={pending || draft.trim() === ''}
          pending={pending}
        >
          {t('action.save')}
        </Button>
        <Button variant="ghost" onClick={exit} disabled={pending}>
          {t('action.cancel')}
        </Button>
      </div>
    </div>
  )
}
