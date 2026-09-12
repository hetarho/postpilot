import { useRef, useState } from 'react'
import { useBlocker, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  CLIP_TEMPLATE_LIMITS,
  CompositionBuilder,
  CompositionPreview,
  CompositionProblem,
  EMPTY_CLIP_COMPOSITION,
  clipCompositionGuide,
  emptyClipRecipe,
  normalizeRecipe,
  parseClipComposition,
  recipeOf,
  useClipTemplateMutations,
  validateClipRecipe,
  type ClipComposition,
  type ClipRecipe,
  type ClipTemplate,
} from '@/entities/clip-template'
import { useClipCapabilities } from '@/entities/clip-project'
import { appFailureFromConnect } from '@/shared/api'
import { copyText } from '@/shared/lib'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  Sheet,
  TextField,
  Textarea,
  Typography,
} from '@/shared/ui'

const FORM_ID = 'clip-template-form'
function authoredRecipe(stored?: ClipTemplate): ClipRecipe {
  const recipe = stored ? recipeOf(stored) : emptyClipRecipe()
  return {
    ...recipe,
    compositionBody: stored?.compositionBody ?? (stored ? '' : EMPTY_CLIP_COMPOSITION),
    compositionLegacy: false,
  }
}
export function ClipTemplateEditor({
  ownerId,
  stored,
}: {
  ownerId: string
  stored?: ClipTemplate
}) {
  const { t } = useTranslation('clips')
  const navigate = useNavigate()
  const [draft, setDraft] = useState(() => authoredRecipe(stored))
  const [baseline, setBaseline] = useState(() =>
    JSON.stringify(normalizeRecipe(authoredRecipe(stored))),
  )
  const [mode, setMode] = useState<'builder' | 'source'>('builder')
  const [saved, setSaved] = useState(false)
  const [copyStatus, setCopyStatus] = useState('')
  const [guide, setGuide] = useState<string | null>(null)
  const sourceField = useRef<HTMLTextAreaElement>(null)
  const { save: saveMutation } = useClipTemplateMutations(ownerId)
  const capabilities = useClipCapabilities(ownerId)
  const submitting = useRef(false),
    leaving = useRef(false)
  const errors = validateClipRecipe(draft)
  const dirty = JSON.stringify(normalizeRecipe(draft)) !== baseline
  const pending = saveMutation.isPending
  const guard = () => dirty && !leaving.current
  const blocker = useBlocker({
    shouldBlockFn: guard,
    enableBeforeUnload: guard,
    withResolver: true,
  })
  const body = draft.compositionBody ?? ''
  let document: ClipComposition | undefined, problem: CompositionProblem | undefined
  try {
    document = parseClipComposition(body)
  } catch (error) {
    if (error instanceof CompositionProblem) problem = error
    else throw error
  }
  const change = (patch: Partial<ClipRecipe>) => {
    setSaved(false)
    setDraft((current) => ({ ...current, ...patch }))
  }
  const save = async () => {
    if (!dirty || !errors.valid || pending || submitting.current) return
    submitting.current = true
    try {
      const result = await saveMutation.mutateAsync({ id: stored?.id, recipe: draft })
      const next = authoredRecipe(result)
      setDraft(next)
      setBaseline(JSON.stringify(normalizeRecipe(next)))
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
      /* Retain the exact draft after a server refusal. */
    } finally {
      submitting.current = false
    }
  }
  const copy = async (format: boolean) => {
    setSaved(false)
    const text = format ? clipCompositionGuide() : body
    const result = await copyText(text)
    setCopyStatus(t(result.copied ? 'composition.copied' : 'composition.copyManually'))
    if (!result.copied) {
      if (format) setGuide(text)
      else {
        setMode('source')
        requestAnimationFrame(() => {
          sourceField.current?.focus()
          sourceField.current?.select()
        })
      }
    }
  }
  return (
    <>
      <div role="status" aria-live="polite" className="mt-4">
        <Typography variant="meta">{saved ? t('editor.saved') : copyStatus}</Typography>
      </div>
      <form
        id={FORM_ID}
        className="mt-6 min-w-0 space-y-6"
        onSubmit={(event) => {
          event.preventDefault()
          void save()
        }}
      >
        <fieldset disabled={pending} className="min-w-0 space-y-6">
          <div>
            <FieldLabel htmlFor="clip-template-name">{t('editor.name')}</FieldLabel>
            <TextField
              id="clip-template-name"
              value={draft.name}
              autoComplete="off"
              onChange={(e) => change({ name: e.target.value })}
              aria-invalid={!!errors.name}
            />
            {errors.name && (
              <FieldMessage>
                {t(`validation.${errors.name}`, { max: CLIP_TEMPLATE_LIMITS.name })}
              </FieldMessage>
            )}
          </div>
          {stored?.compositionLegacy && (
            <Typography variant="body" className="text-content-secondary">
              {t('composition.converted')}
            </Typography>
          )}
          {capabilities.data &&
            (capabilities.data.compositionVersion !== 1 ||
              capabilities.data.compositionPlanVersion < 5) && (
              <Typography variant="body" role="status">
                {t('composition.unavailable')}
              </Typography>
            )}
          <SegmentedControl
            value={mode}
            onChange={setMode}
            ariaLabel={t('composition.mode')}
            controls="clip-composition-panel"
            options={[
              { value: 'builder', label: t('composition.builder') },
              { value: 'source', label: t('composition.source') },
            ]}
          />
          <div className="flex flex-wrap gap-2">
            <Button variant="ghost" onClick={() => void copy(false)}>
              {t('composition.copySource')}
            </Button>
            <Button variant="ghost" onClick={() => void copy(true)}>
              {t('composition.copyGuide')}
            </Button>
          </div>
          {problem && (
            <div>
              <FieldMessage>
                {t('composition.invalid', {
                  line: problem.line,
                  element: problem.elementId || 'clip',
                })}{' '}
                {t(`composition.errors.${problem.reason}`, {
                  defaultValue: t('composition.repairSource'),
                })}
              </FieldMessage>
            </div>
          )}
          <div
            id="clip-composition-panel"
            role="tabpanel"
            aria-label={t(mode === 'source' ? 'composition.source' : 'composition.builder')}
            className="min-w-0"
          >
            {mode === 'source' ? (
              <>
                <FieldLabel htmlFor="clip-composition-source">{t('composition.source')}</FieldLabel>
                <Textarea
                  ref={sourceField}
                  id="clip-composition-source"
                  value={body}
                  onChange={(e) => change({ compositionBody: e.target.value })}
                  rows={16}
                  spellCheck={false}
                  aria-invalid={!!problem}
                />
              </>
            ) : (
              <CompositionBuilder
                source={body}
                onChange={(compositionBody) => change({ compositionBody })}
              />
            )}
          </div>
          {document && <CompositionPreview document={document} />}
        </fieldset>
      </form>
      <ActionBar className="mt-auto">
        {saveMutation.error && (
          <div role="alert" className="mb-3">
            <AppFailureMessage failure={appFailureFromConnect(saveMutation.error)} />
          </div>
        )}
        <div className="flex justify-end">
          <Button
            type="submit"
            form={FORM_ID}
            variant="cta"
            className="w-full sm:w-auto"
            pending={pending}
            disabled={!dirty || !errors.valid || pending}
          >
            {t('editor.save')}
          </Button>
        </div>
      </ActionBar>
      <Sheet
        open={guide !== null}
        labelledBy="clip-guide-title"
        onClose={() => setGuide(null)}
        header={
          <Typography id="clip-guide-title" variant="fieldTitle">
            {t('composition.copyGuide')}
          </Typography>
        }
      >
        <Textarea
          aria-label={t('composition.copyGuide')}
          value={guide ?? ''}
          readOnly
          onFocus={(e) => e.currentTarget.select()}
        />
      </Sheet>
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
