import { useRef, useState } from 'react'
import { useBlocker, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  CLIP_ACCENTS,
  CLIP_TEMPLATE_LIMITS,
  COPY_STYLES,
  CopyStylePreview,
  emptyClipRecipe,
  normalizeRecipe,
  recipeOf,
  useClipTemplateMutations,
  validateClipRecipe,
  type ClipRecipe,
  type ClipTemplate,
  type FieldError,
  type InformationField,
} from '@/entities/clip-template'
import { appFailureFromConnect } from '@/shared/api'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Checkbox,
  Dialog,
  FieldLabel,
  FieldMessage,
  Listbox,
  SortableList,
  TextField,
  Textarea,
  Typography,
} from '@/shared/ui'

const FORM_ID = 'clip-template-form'
const textInput = {
  inputMode: 'text',
  autoComplete: 'off',
  autoCapitalize: 'sentences',
  autoCorrect: 'on',
  enterKeyHint: 'next',
} as const

export function ClipTemplateEditor({
  ownerId,
  stored,
}: {
  ownerId: string
  stored?: ClipTemplate
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const [draft, setDraft] = useState<ClipRecipe>(() =>
    stored ? recipeOf(stored) : emptyClipRecipe(),
  )
  const [baseline, setBaseline] = useState(() =>
    JSON.stringify(normalizeRecipe(stored ? recipeOf(stored) : emptyClipRecipe())),
  )
  const [fieldIds, setFieldIds] = useState(() =>
    draft.informationFields.map((_, i) => `field-${i}`),
  )
  const [saved, setSaved] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const { save: saveMutation, remove } = useClipTemplateMutations(ownerId)
  const submitting = useRef(false)
  const leaving = useRef(false)
  const errors = validateClipRecipe(draft)
  const dirty = JSON.stringify(normalizeRecipe(draft)) !== baseline
  const pending = saveMutation.isPending || remove.isPending
  const guard = () => dirty && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })
  const mutationError = saveMutation.error ?? remove.error
  const failure = mutationError ? appFailureFromConnect(mutationError) : undefined

  const change = <K extends keyof ClipRecipe>(key: K, value: ClipRecipe[K]) => {
    setSaved(false)
    setDraft((current) => ({ ...current, [key]: value }))
  }
  const fieldChange = (index: number, key: keyof InformationField, value: string) =>
    change(
      'informationFields',
      draft.informationFields.map((f, i) => (i === index ? { ...f, [key]: value } : f)),
    )
  const reorder = (from: number, to: number) => {
    if (pending) return
    const fields = [...draft.informationFields]
    fields.splice(to, 0, fields.splice(from, 1)[0]!)
    const ids = [...fieldIds]
    ids.splice(to, 0, ids.splice(from, 1)[0]!)
    setFieldIds(ids)
    change('informationFields', fields)
  }
  const save = async () => {
    if (!dirty || !errors.valid || submitting.current || pending) return
    submitting.current = true
    try {
      const result = await saveMutation.mutateAsync({ id: stored?.id, recipe: draft })
      setDraft(recipeOf(result))
      setBaseline(JSON.stringify(normalizeRecipe(recipeOf(result))))
      setSaved(true)
      if (!stored) {
        leaving.current = true
        await navigate({
          to: '/video-templates/$templateId',
          params: { templateId: result.id },
          replace: true,
        })
      }
    } catch {
      /* The stable failure is rendered with the original draft intact. */
    } finally {
      submitting.current = false
    }
  }
  const deleteTemplate = async () => {
    if (!stored || submitting.current) return
    submitting.current = true
    try {
      const result = await remove.mutateAsync(stored.id)
      leaving.current = true
      await navigate({
        to: '/video-templates',
        replace: true,
        state: { clipDetachedCount: result.detachedProjects },
      })
    } catch {
      setConfirmDelete(false)
    } finally {
      submitting.current = false
    }
  }
  const message = (error: FieldError | undefined, max: number) =>
    error ? t(`validation.${error}`, { max }) : undefined

  return (
    <>
      <div role="status" aria-live="polite" className="mt-4">
        {saved && <Typography variant="meta">{t('editor.saved')}</Typography>}
      </div>
      <form
        id={FORM_ID}
        onSubmit={(event) => {
          event.preventDefault()
          void save()
        }}
        className="mt-6"
      >
        <fieldset disabled={pending} className="min-w-0 space-y-8">
          <div>
            <FieldLabel htmlFor="clip-template-name">{t('editor.name')}</FieldLabel>
            <TextField
              id="clip-template-name"
              type="text"
              {...textInput}
              value={draft.name}
              aria-invalid={!!errors.name}
              aria-describedby={errors.name ? 'clip-name-error' : undefined}
              onChange={(e) => change('name', e.target.value)}
            />
            {errors.name && (
              <FieldMessage id="clip-name-error">
                {message(errors.name, CLIP_TEMPLATE_LIMITS.name)}
              </FieldMessage>
            )}
          </div>
          <div>
            <FieldLabel htmlFor="clip-template-guidance">{t('editor.guidance')}</FieldLabel>
            <Textarea
              id="clip-template-guidance"
              {...textInput}
              autoGrow
              value={draft.cutGuidance}
              aria-invalid={!!errors.guidance}
              onChange={(e) => change('cutGuidance', e.target.value)}
            />
            {errors.guidance && (
              <FieldMessage>{message(errors.guidance, CLIP_TEMPLATE_LIMITS.guidance)}</FieldMessage>
            )}
          </div>
          <section aria-labelledby="clip-information-heading">
            <Typography variant="title" id="clip-information-heading">
              {t('editor.fields')}
            </Typography>
            <Typography variant="body" className="text-content-secondary mt-2">
              {t('editor.fieldsHelp')}
            </Typography>
            <SortableList
              labels={{ drag: t('editor.drag'), up: t('editor.up'), down: t('editor.down') }}
              onReorder={reorder}
              items={draft.informationFields.map((field, index) => ({
                id: fieldIds[index]!,
                content: (
                  <div className="min-w-0 space-y-3">
                    <div>
                      <FieldLabel htmlFor={`${fieldIds[index]}-label`}>
                        {t('editor.fieldLabel', { number: index + 1 })}
                      </FieldLabel>
                      <TextField
                        id={`${fieldIds[index]}-label`}
                        type="text"
                        {...textInput}
                        value={field.label}
                        aria-invalid={!!errors.fields[index]?.label}
                        onChange={(e) => fieldChange(index, 'label', e.target.value)}
                      />
                      {errors.fields[index]?.label && (
                        <FieldMessage>
                          {message(errors.fields[index]?.label, CLIP_TEMPLATE_LIMITS.label)}
                        </FieldMessage>
                      )}
                    </div>
                    <div>
                      <FieldLabel htmlFor={`${fieldIds[index]}-prompt`}>
                        {t('editor.fieldPrompt', { number: index + 1 })}
                      </FieldLabel>
                      <Textarea
                        id={`${fieldIds[index]}-prompt`}
                        {...textInput}
                        autoGrow
                        value={field.prompt}
                        aria-invalid={!!errors.fields[index]?.prompt}
                        onChange={(e) => fieldChange(index, 'prompt', e.target.value)}
                      />
                      {errors.fields[index]?.prompt && (
                        <FieldMessage>
                          {message(errors.fields[index]?.prompt, CLIP_TEMPLATE_LIMITS.prompt)}
                        </FieldMessage>
                      )}
                    </div>
                    <Button
                      variant="danger"
                      onClick={() => {
                        setFieldIds(fieldIds.filter((_, i) => i !== index))
                        change(
                          'informationFields',
                          draft.informationFields.filter((_, i) => i !== index),
                        )
                      }}
                    >
                      {t('editor.removeField', { number: index + 1 })}
                    </Button>
                  </div>
                ),
              }))}
            />
            <Button
              variant="secondary"
              className="mt-3"
              disabled={draft.informationFields.length >= CLIP_TEMPLATE_LIMITS.fields}
              onClick={() => {
                setFieldIds([...fieldIds, crypto.randomUUID()])
                change('informationFields', [...draft.informationFields, { label: '', prompt: '' }])
              }}
            >
              {t('editor.addField')}
            </Button>
          </section>
          <section aria-labelledby="clip-styles-heading">
            <Typography variant="title" id="clip-styles-heading">
              {t('editor.styles')}
            </Typography>
            <Typography variant="body" className="text-content-secondary mt-2">
              {t('editor.stylesHelp')}
            </Typography>
            <div className="mt-4 grid gap-6 md:grid-cols-3">
              {COPY_STYLES.map((style) => (
                <div key={style} className="min-w-0">
                  <label className="flex min-h-11 items-center gap-3 px-3">
                    <Checkbox
                      checked={draft.copyStyles.includes(style)}
                      onChange={(e) =>
                        change(
                          'copyStyles',
                          e.target.checked
                            ? [...draft.copyStyles, style]
                            : draft.copyStyles.filter((v) => v !== style),
                        )
                      }
                    />
                    <Typography variant="label">{t(`style.${style}`)}</Typography>
                  </label>
                  <CopyStylePreview
                    style={style}
                    accent={draft.accent}
                    text={t('editor.preview')}
                  />
                </div>
              ))}
            </div>
            {errors.styles && <FieldMessage>{t('validation.styles')}</FieldMessage>}
          </section>
          <div>
            <FieldLabel id="clip-accent-label" htmlFor="clip-accent">
              {t('editor.accent')}
            </FieldLabel>
            <Listbox
              id="clip-accent"
              aria-labelledby="clip-accent-label"
              value={draft.accent}
              onChange={(value) => change('accent', value)}
              options={CLIP_ACCENTS.map((value) => ({
                value,
                label: t(`accent.${value || 'none'}`),
              }))}
            />
          </div>
        </fieldset>
      </form>
      <ActionBar className="mt-auto">
        {failure && (
          <div role="alert" className="mb-3">
            <AppFailureMessage failure={failure} />
          </div>
        )}
        <div className="flex flex-wrap items-center justify-end gap-3">
          {stored && (
            <Button variant="danger" disabled={pending} onClick={() => setConfirmDelete(true)}>
              {t('delete.action')}
            </Button>
          )}
          <Button
            type="submit"
            form={FORM_ID}
            variant="cta"
            className="w-full sm:w-auto"
            pending={saveMutation.isPending}
            disabled={!dirty || !errors.valid || pending}
          >
            {t('editor.save')}
          </Button>
        </div>
      </ActionBar>
      <Dialog
        open={confirmDelete}
        title={t('delete.title')}
        confirmLabel={t('delete.action')}
        onClose={() => setConfirmDelete(false)}
        onConfirm={() => void deleteTemplate()}
        pending={remove.isPending}
      >
        {t('delete.description', { count: stored?.projectCount ?? 0 })}
      </Dialog>
      <Dialog
        open={blocker.status === 'blocked'}
        title={t('editor.leaveTitle')}
        confirmLabel={t('editor.leave')}
        onClose={() => blocker.reset?.()}
        onConfirm={() => blocker.proceed?.()}
      >
        {t('editor.leaveBody')}
      </Dialog>
    </>
  )
}
