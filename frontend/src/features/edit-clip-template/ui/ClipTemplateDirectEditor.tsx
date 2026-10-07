import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { AuthoringArtifact } from '@/entities/ai-authoring'
import {
  CLIP_TEMPLATE_LIMITS,
  CompositionBuilder,
  CompositionProblem,
  parseClipTemplate,
} from '@/entities/clip-template'
import {
  Button,
  FieldCount,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  TextField,
  Textarea,
} from '@/shared/ui'

/** The EDIT draft owns name and structure. Render/design defaults stay in the template domain. */
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
  const [mode, setMode] = useState<'builder' | 'source'>('builder')
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
      <SegmentedControl
        value={mode}
        onChange={setMode}
        disabled={disabled}
        ariaLabel={t('composition.mode')}
        options={[
          { value: 'builder', label: t('composition.builder') },
          { value: 'source', label: t('composition.source') },
        ]}
      />
      {problem && (
        <FieldMessage role="alert">
          {t('composition.invalid', { line: problem.line, element: problem.elementId || 'clip' })}{' '}
          {t(`composition.errors.${problem.reason}`, {
            element: problem.elementId,
            defaultValue: t('composition.repairSource'),
          })}
        </FieldMessage>
      )}
      {mode === 'source' ? (
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
        </div>
      ) : (
        <CompositionBuilder
          source={source.body || '<clip version="1"/>'}
          onChange={(body) => onChange({ ...source, body })}
        />
      )}
      {mode === 'builder' && problem && (
        <Button variant="secondary" onClick={() => setMode('source')}>
          {t('composition.repairSource')}
        </Button>
      )}
    </fieldset>
  )
}
