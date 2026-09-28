import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useClipTemplates } from '@/entities/clip-template'
import {
  GuidelineTitleField,
  canSaveGuideline,
  globalScope,
  isDuplicateGuideline,
  remainingGuidelineChars,
  remainingGuidelineTitleChars,
  useCreateGuidelineCall,
  type GuidelineScope,
} from '@/entities/guideline'
import {
  Button,
  Dialog,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  Textarea,
  Typography,
} from '@/shared/ui'

/** Turns a completed clip revision request into a saved 영상 지침 (GUIDE-45), the clip
 *  counterpart of the post editor's 지침으로 저장.
 *
 *  An explicit save of the owner's own words, not learning: the dialog seeds the request and the
 *  owner edits it before saving, and no model or job can reach this path (GUIDE-18). Saving also
 *  approves the 영상 지침 후보 the completed revision recorded — the server matches it by text
 *  within the kind in the create's own transaction — so this request never afterwards waits in
 *  the 후보 section. */
export function SaveAsVideoGuidelineButton({
  ownerId,
  request,
  videoTemplateId,
}: {
  ownerId: string
  /** The request the completed revision ran with; it seeds the dialog. */
  request: string
  /** The project's video template, or empty when it has none — then only 전역 is offered. */
  videoTemplateId: string
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const fieldId = `${id}-text`
  const countId = `${id}-count`
  const errorId = `${id}-error`
  const [open, setOpen] = useState(false)
  const [title, setTitle] = useState('')
  const [text, setText] = useState(request)
  const [scope, setScope] = useState<GuidelineScope>(globalScope)
  const [saved, setSaved] = useState(false)
  const create = useCreateGuidelineCall(ownerId, 'clip')
  const { templates } = useClipTemplates(videoTemplateId ? ownerId : '')
  const template = templates.find((candidate) => candidate.id === videoTemplateId)

  // Seeded on OPEN, not from an effect on `request`: a refetch must not overwrite what is being
  // edited here, and reopening starts from the current request rather than the last draft.
  const openDialog = () => {
    setTitle('')
    setText(request)
    setScope(globalScope())
    setSaved(false)
    setOpen(true)
  }

  const left = remainingGuidelineChars(text)
  const exceeded = left < 0
  const showCreateError = create.isError && !isDuplicateGuideline(create.error)
  const blocked =
    !canSaveGuideline(text, scope) || remainingGuidelineTitleChars(title) < 0 || create.isPending

  const confirm = async () => {
    if (blocked) return
    try {
      await create.create({ title, text, scope })
      setSaved(true)
      setOpen(false)
    } catch (cause) {
      // An exact duplicate is information, not a failure: the rule is already saved.
      if (isDuplicateGuideline(cause)) {
        setSaved(true)
        setOpen(false)
      }
      // Any other refusal keeps the dialog open with the draft intact.
    }
  }

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button variant="ghost" onClick={openDialog}>
        {t('clipCapture.action', { ns: 'guidelines' })}
      </Button>
      {saved && !open && (
        <Typography variant="body" role="status" className="text-content-secondary">
          {isDuplicateGuideline(create.error)
            ? t('clipCapture.duplicate', { ns: 'guidelines' })
            : t('clipCapture.saved', { ns: 'guidelines' })}
        </Typography>
      )}
      <Dialog
        open={open}
        title={t('clipCapture.action', { ns: 'guidelines' })}
        confirmLabel={t('clipCapture.submit', { ns: 'guidelines' })}
        pending={create.isPending}
        onClose={() => setOpen(false)}
        onConfirm={() => void confirm()}
      >
        <Typography variant="body" className="text-content-secondary">
          {t('clipCapture.description', { ns: 'guidelines' })}
        </Typography>
        <GuidelineTitleField
          id={`${id}-name`}
          value={title}
          onChange={setTitle}
          disabled={create.isPending}
          className="mt-4"
        />
        <FieldLabel htmlFor={fieldId} className="mt-4 block">
          {t('clipCapture.text', { ns: 'guidelines' })}
        </FieldLabel>
        <Textarea
          id={fieldId}
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={3}
          autoGrow
          aria-invalid={exceeded || showCreateError || undefined}
          aria-describedby={`${countId}${showCreateError ? ` ${errorId}` : ''}`}
          className="mt-1"
        />
        {exceeded ? (
          <FieldMessage id={countId} role="status" className="mt-2">
            {t('count.exceeded', { ns: 'common', count: -left })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p" id={countId} className="mt-2">
            {t('count.remaining', { ns: 'common', count: left })}
          </Typography>
        )}
        {/* Two options, and only when the project has a video template: the whole directory
            belongs on /video-guidelines, not in a dialog opened mid-edit. */}
        {template && (
          <>
            <Typography variant="label" as="p" className="mt-4">
              {t('clipCapture.scope', { ns: 'guidelines' })}
            </Typography>
            <SegmentedControl
              value={scope.kind}
              options={[
                { value: 'global', label: t('clipCapture.scopeGlobal', { ns: 'guidelines' }) },
                {
                  value: 'templates',
                  label: t('clipCapture.scopeTemplate', {
                    ns: 'guidelines',
                    name: template.name,
                  }),
                },
              ]}
              onChange={(kind) =>
                setScope(
                  kind === 'global'
                    ? globalScope()
                    : { kind: 'templates', templateIds: [template.id], fields: [] },
                )
              }
              ariaLabel={t('clipCapture.scope', { ns: 'guidelines' })}
              className="mt-2"
            />
          </>
        )}
        {showCreateError && (
          <FieldMessage id={errorId} className="mt-3">
            {create.errorMessage}
          </FieldMessage>
        )}
      </Dialog>
    </div>
  )
}
