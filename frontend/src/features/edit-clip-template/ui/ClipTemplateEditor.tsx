import { useRef, useState } from 'react'
import { useBlocker, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  CLIP_ACCENTS,
  CLIP_PRESETS_LIST,
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
  const [pendingSeed, setPendingSeed] = useState<InformationField[] | null>(null)
  const [fieldIds, setFieldIds] = useState(() =>
    draft.informationFields.map((_, i) => `field-${i}`),
  )
  const [saved, setSaved] = useState(false)
  const { save: saveMutation, seed: seedMutation } = useClipTemplateMutations(ownerId)
  const submitting = useRef(false)
  const leaving = useRef(false)
  const errors = validateClipRecipe(draft)
  const dirty = JSON.stringify(normalizeRecipe(draft)) !== baseline
  const pending = saveMutation.isPending
  const guard = () => dirty && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })
  const failure = saveMutation.error ? appFailureFromConnect(saveMutation.error) : undefined

  const change = <K extends keyof ClipRecipe>(key: K, value: ClipRecipe[K]) => {
    setSaved(false)
    setDraft((current) => ({ ...current, [key]: value }))
  }
  // Choosing a preset seeds the reserved information fields it needs. Existing
  // labels are KEPT — the owner's own questions and their answers survive
  // (CLIP-25) — and only missing reserved ones are added. When the template is
  // already used by projects, the addition changes what their step ① asks for,
  // so the owner is told before it happens.
  const applySeed = (fields: InformationField[]) => {
    const existing = new Set(draft.informationFields.map((f) => f.label.trim()))
    const additions = fields.filter((f) => !existing.has(f.label))
    if (additions.length === 0) return
    setFieldIds([...fieldIds, ...additions.map(() => crypto.randomUUID())])
    change('informationFields', [...draft.informationFields, ...additions])
  }
  const choosePreset = async (preset: ClipRecipe['preset']) => {
    change('preset', preset)
    if (preset === '') return
    const fields = await seedMutation.mutateAsync(preset)
    const existing = new Set(draft.informationFields.map((f) => f.label.trim()))
    const additions = fields.filter((f) => !existing.has(f.label))
    if (additions.length > 0 && (stored?.projectCount ?? 0) > 0) {
      setPendingSeed(fields)
      return
    }
    applySeed(fields)
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
          <div>
            <FieldLabel id="clip-preset-label" htmlFor="clip-preset">
              {t('editor.preset')}
            </FieldLabel>
            <Typography variant="body" className="text-content-secondary mb-2">
              {t('editor.presetHelp')}
            </Typography>
            <Listbox
              id="clip-preset"
              aria-labelledby="clip-preset-label"
              value={draft.preset}
              onChange={(value) => choosePreset(value as ClipRecipe['preset'])}
              options={[
                { value: '', label: t('editor.presetNone'), disabled: true },
                ...CLIP_PRESETS_LIST.map((value) => ({ value, label: t(`preset.${value}`) })),
              ]}
            />
            {errors.preset && <FieldMessage>{t('validation.preset')}</FieldMessage>}
          </div>
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
                      checked={style === 'clean' || draft.copyStyles.includes(style)}
                      disabled={style === 'clean'}
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
                  {style === 'clean' && (
                    <Typography variant="body" className="text-content-secondary px-3">
                      {t('editor.styleAlwaysOn')}
                    </Typography>
                  )}
                  <CopyStylePreview
                    style={style}
                    accent={draft.accent}
                    keyword={t('editor.previewKeyword')}
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
        {/* One action, like `/templates/$templateId`'s dock: the delete rides the directory row
            now (CLIP-42). */}
        <div className="flex flex-wrap items-center justify-end gap-3">
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
        open={pendingSeed !== null}
        title={t('editor.seedTitle')}
        confirmLabel={t('editor.seedConfirm')}
        onClose={() => setPendingSeed(null)}
        onConfirm={() => {
          if (pendingSeed) applySeed(pendingSeed)
          setPendingSeed(null)
        }}
      >
        {t('editor.seedBody', { count: stored?.projectCount ?? 0 })}
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
