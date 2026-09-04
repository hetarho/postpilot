import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  NO_TEMPLATE_VALUE,
  noTemplateLabel,
  useTemplates,
  type TemplateRef,
} from '@/entities/template'
import {
  Button,
  FieldLabel,
  FieldMessage,
  Listbox,
  Typography,
  type ListboxOption,
} from '@/shared/ui'
import { runningJobNote, assignmentFailureMessage } from '../model/assignment'

interface PostTemplateSelectProps {
  ownerId: string
  /** The assignment as the editor shows it; '' is 없음. */
  value: string
  /** The post's template as the server reports it — undefined for a draft with no post yet.
   *  Listed even while the directory loads, so a post that plainly has one is never shown
   *  as 없음 for a beat. */
  current?: TemplateRef
  /** True while an AI job could still be running. The select stays usable either way; this
   *  only decides whether the note about the frozen brief is shown. */
  jobRunning?: boolean
  onSelect: (templateId: string) => Promise<void> | void
  className?: string
}

/** The optional 템플릿 of a post: an app-drawn listbox wearing the field well (design-language §7),
 *  defaulting to 없음, beside the required voice select on the editor dock's own row.
 *
 *  Like its neighbour it carries no VISIBLE label — three columns on a 360px screen have no width
 *  for one — and its `sr-only` label keeps the control announced as '템플릿 <값>'. It also offers no
 *  way to the 템플릿 page: this is the row a post is written from, the directory is a tab of its own
 *  in the app's navigation, and a link out of a docked bar mid-draft is an invitation to lose the
 *  draft (owner decision 2026-09-02). */
export function PostTemplateSelect({
  ownerId,
  value,
  current,
  jobRunning = false,
  onSelect,
  className,
}: PostTemplateSelectProps) {
  const { t } = useTranslation(['templates', 'common'])
  const id = useId()
  const labelId = `${id}-label`
  const hintId = `${id}-hint`
  const errorId = `${id}-error`
  const { templates, isPending, isError, isFetching, refetch } = useTemplates(ownerId)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState('')

  // The post's own template is listed even while the directory is still loading; a select
  // showing 없음 under a post that has one reads as if the assignment were lost.
  const unlisted =
    current && current.id && !templates.some((template) => template.id === current.id)
      ? current
      : undefined
  const describedBy = [jobRunning ? hintId : '', error || isError ? errorId : '']
    .filter(Boolean)
    .join(' ')

  const options: ListboxOption<string>[] = [
    { value: NO_TEMPLATE_VALUE, label: noTemplateLabel() },
    ...(unlisted ? [{ value: unlisted.id, label: unlisted.name || unlisted.id }] : []),
    ...templates.map((template) => ({ value: template.id, label: template.name })),
  ]

  const apply = async (templateId: string) => {
    setApplying(true)
    setError('')
    try {
      await onSelect(templateId)
    } catch (cause) {
      setError(assignmentFailureMessage(cause))
    } finally {
      setApplying(false)
    }
  }

  const onChange = (next: string) => {
    if (next === value) return
    // No confirmation sheet, unlike the voice: changing a template moves no content, drops no
    // baseline and costs no learning, so there is nothing to warn about.
    void apply(next)
  }

  return (
    <div className={className}>
      <div className="flex items-center gap-2">
        <FieldLabel id={labelId} htmlFor={id} className="sr-only">
          {t('title', { ns: 'templates' })}
        </FieldLabel>
        <span className="min-w-0 flex-1">
          <Listbox
            id={id}
            aria-labelledby={labelId}
            value={value}
            options={options}
            onChange={onChange}
            disabled={applying || isError || (isPending && !unlisted)}
            aria-invalid={error || isError ? true : undefined}
            aria-describedby={describedBy || undefined}
          />
        </span>
      </div>
      {jobRunning && (
        <Typography variant="label" as="p" id={hintId} role="status" className="mt-2">
          {runningJobNote()}
        </Typography>
      )}
      {/* A failed directory read must not render as "you have no 템플릿": the select would be
          enabled with 없음 alone, and the only thing the user could do is clear a template they
          still have. Disabled, said out loud, with a retry. */}
      {isError && (
        <FieldMessage id={errorId} className="mt-2">
          {t('loadFailed', { ns: 'templates' })}{' '}
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-field-error underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </FieldMessage>
      )}
      {error && (
        <FieldMessage id={errorId} className="mt-2">
          {error}
        </FieldMessage>
      )}
    </div>
  )
}
