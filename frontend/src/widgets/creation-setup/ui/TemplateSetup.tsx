import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  canSaveTemplate,
  parseTemplate,
  TEMPLATE_PARSE_OPTIONS,
  TemplateComposition,
  TemplatePreview,
  useCreateTemplate,
} from '@/entities/template'
import {
  CompositionBuilder,
  CompositionPreview,
  emptyClipRecipe,
  parseClipTemplate,
  useClipTemplateMutations,
  validateClipRecipe,
} from '@/entities/clip-template'
import type { SetupController } from '@/features/complete-setup'
import { appFailureFromConnect } from '@/shared/api'
import { formatAppFailure } from '@/shared/lib'
import { Button, FieldLabel, FieldMessage, TextField, Typography } from '@/shared/ui'
const EMPTY_CLIP_BODY = '<clip version="1"/>'
export function TemplateSetup({
  ownerId,
  kind,
  controller,
}: {
  ownerId: string
  kind: 'post-template' | 'clip-template'
  controller: SetupController
}) {
  const { t } = useTranslation('creation')
  const [name, setName] = useState('')
  const [body, setBody] = useState(kind === 'clip-template' ? EMPTY_CLIP_BODY : '')
  const [error, setError] = useState('')
  const post = useCreateTemplate(ownerId)
  const clip = useClipTemplateMutations(ownerId)
  const fields = { name, description: '', titleArea: '', body }
  const recipe = { ...emptyClipRecipe(), name, compositionBody: body }
  const busy = controller.state.phase === 'saving'
  let valid = kind === 'post-template' ? canSaveTemplate(fields) : validateClipRecipe(recipe).valid
  let document: ReturnType<typeof parseClipTemplate> | undefined
  try {
    if (kind === 'post-template') parseTemplate('', body, TEMPLATE_PARSE_OPTIONS)
    else document = parseClipTemplate(body)
  } catch {
    valid = false
  }
  const save = async (event: FormEvent) => {
    event.preventDefault()
    if (!valid) return
    const operation = controller.begin(kind)
    if (operation === null) return
    setError('')
    try {
      const result =
        kind === 'post-template'
          ? (await post.create(fields)).template
          : await clip.save.mutateAsync({ recipe })
      if (!result?.id) throw new Error('Missing confirmed template')
      controller.success(kind, operation, true)
    } catch (failure) {
      setError(formatAppFailure(appFailureFromConnect(failure)))
      controller.failure(kind, operation)
    }
  }
  return (
    <form onSubmit={(event) => void save(event)}>
      <fieldset disabled={busy} className="min-w-0">
        <FieldLabel htmlFor="setup-template-name">{t('setup.templateName')}</FieldLabel>
        <TextField
          id="setup-template-name"
          value={name}
          onChange={(event) => setName(event.target.value)}
          autoComplete="off"
          className="mt-2"
        />
        <div className="mt-6 grid gap-8 md:grid-cols-2">
          <div className="min-w-0">
            {kind === 'post-template' ? (
              <TemplateComposition value={body} onChange={setBody} />
            ) : (
              <CompositionBuilder source={body} onChange={setBody} />
            )}
          </div>
          <details open className="min-w-0">
            <summary className="text-link-fg hover:text-link-fg-hover min-h-11 py-3">
              <Typography variant="body" as="span">
                {t('setup.preview')}
              </Typography>
            </summary>
            {kind === 'post-template' ? (
              <TemplatePreview titleArea="" body={body} />
            ) : (
              document && (
                <CompositionPreview
                  document={document}
                  presets={{ intro: recipe.introPreset, outro: recipe.outroPreset }}
                  captionStyles={recipe.allowedCaptionStyles}
                />
              )
            )}
          </details>
        </div>
      </fieldset>
      <div role="status" className="mt-4">
        {error && <FieldMessage>{error}</FieldMessage>}
      </div>
      <Button
        type="submit"
        variant="cta"
        className="mt-4 w-full sm:w-auto"
        disabled={!valid || busy}
        pending={busy}
      >
        {t('setup.save')}
      </Button>
    </form>
  )
}
