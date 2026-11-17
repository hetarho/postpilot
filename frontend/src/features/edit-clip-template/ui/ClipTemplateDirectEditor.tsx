import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { AuthoringArtifact } from '@/entities/ai-authoring'
import {
  CLIP_TEMPLATE_LIMITS,
  clipCompositionGuide,
  CompositionProblem,
  parseClipTemplate,
} from '@/entities/clip-template'
import { Disclosure, FieldCount, FieldLabel, FieldMessage, TextField, Textarea } from '@/shared/ui'

/** The template's own text, typed directly — the secondary path beside the conversation. The EDIT
 *  draft owns name and structure; render/design defaults stay in the template domain. There is no
 *  block builder: nothing is dragged, moved or reordered by hand — the order of a video is the
 *  order of its text, and the AI changes it when asked. */
export function ClipTemplateDirectEditor({
  source,
  onChange,
  disabled,
}: {
  source: AuthoringArtifact
  onChange: (source: AuthoringArtifact) => void
  disabled: boolean
}) {
  const { t } = useTranslation('clips')
  const id = useId()
  let problem: CompositionProblem | undefined
  try {
    parseClipTemplate(source.body)
  } catch (error) {
    if (error instanceof CompositionProblem) problem = error
    else throw error
  }
  return (
    <fieldset disabled={disabled} className="min-w-0 space-y-6">
      <div>
        <FieldLabel htmlFor={id + '-name'}>{t('editor.name')}</FieldLabel>
        <TextField
          id={id + '-name'}
          value={source.name}
          onChange={(e) => onChange({ ...source, name: e.target.value })}
          autoComplete="off"
          className="mt-1"
        />
        <FieldCount left={CLIP_TEMPLATE_LIMITS.name - Array.from(source.name.trim()).length} />
      </div>
      <div>
        <FieldLabel htmlFor={id + '-source'}>{t('composition.source')}</FieldLabel>
        <Textarea
          id={id + '-source'}
          value={source.body}
          rows={12}
          autoGrow
          spellCheck={false}
          aria-invalid={!!problem}
          onChange={(e) => onChange({ ...source, body: e.target.value })}
          className="mt-1"
        />
        {problem && (
          <FieldMessage role="alert">
            {t('composition.invalid', {
              line: problem.line,
              element: problem.elementId || 'clip',
            })}{' '}
            {t(`composition.errors.${problem.reason}`, {
              element: problem.elementId,
              defaultValue: t('composition.repairSource'),
            })}
          </FieldMessage>
        )}
        <Disclosure title={t('editor.sourceGuide')} size="row" headingLevel={3} className="mt-3">
          <Textarea
            aria-label={t('editor.sourceGuide')}
            readOnly
            rows={8}
            value={clipCompositionGuide()}
          />
        </Disclosure>
      </div>
    </fieldset>
  )
}
