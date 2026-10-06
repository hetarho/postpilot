import { useRef, useState, type FormEvent } from 'react'
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
import { AuthoringEditor, AuthoringPreview } from '@/features/ai-authoring'
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
  const { t } = useTranslation('authoring')
  const [manual, setManual] = useState(false)
  const [studioBusy, setStudioBusy] = useState(false)
  const operation = useRef<number | null>(null)
  const studioKind = kind === 'post-template' ? 'post-template' : 'video-template'
  const onBusy = (busy: boolean) => {
    setStudioBusy(busy)
    if (busy && operation.current === null) {
      const started = controller.begin(kind)
      if (started !== null) {
        operation.current = started
        controller.running(kind, started)
      }
    } else if (!busy && operation.current !== null) {
      controller.success(kind, operation.current, false)
      operation.current = null
    }
  }
  if (manual) return <ManualTemplateSetup ownerId={ownerId} kind={kind} controller={controller} />
  return (
    <div className="space-y-6">
      <AuthoringEditor
        ownerId={ownerId}
        kind={studioKind}
        onBusyChange={onBusy}
        renderPreview={(artifact) => <AuthoringPreview kind={studioKind} artifact={artifact} />}
        onSaved={(saved) => {
          if (!saved.id) return
          const pending = operation.current
          operation.current = null
          if (pending !== null) controller.success(kind, pending, true)
          else controller.next(true)
        }}
      />
      <Button
        variant="ghost"
        disabled={studioBusy}
        onClick={() => {
          if (operation.current === null) setManual(true)
        }}
      >
        {t('host.manual')}
      </Button>
    </div>
  )
}

function ManualTemplateSetup({
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
