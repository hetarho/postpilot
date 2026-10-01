import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Link } from '@tanstack/react-router'
import { X } from 'lucide-react'
import { sameRef, useStageSelection } from '@/entities/model-catalog'
import {
  TEMPLATE_REQUEST_MAX_CHARS,
  remainingChars,
  useTemplateRequestEstimate,
  type TemplateDraftTexts,
} from '@/entities/template'
import { isLocale, type Locale } from '@/shared/lib/localization'
import {
  AppFailureMessage,
  Button,
  FieldCount,
  Notice,
  Textarea,
  Typography,
  buttonStyles,
} from '@/shared/ui'
import type { TemplateRequestState } from '../model/useTemplateRequest'

/** A post attached as the request's sample (TMPL-64), or one that could not be read. */
export type TemplateRequestSample =
  { kind: 'post'; slug: string; title: string } | { kind: 'missing' }

interface TemplateRequestBoxProps {
  request: TemplateRequestState
  /** The draft as the editor holds it, sent whole with every request (TMPL-58). */
  draft: TemplateDraftTexts
  /** The stored template the request edits; absent on `/templates/new`. */
  templateId?: string
  /** An empty new template, or one opened from a post: the box is the first action (TMPL-62). */
  startOpen: boolean
  /** A new template on an account at its cap: its request would make a draft no save accepts. */
  atCap: boolean
  sample?: TemplateRequestSample
  onRemoveSample: () => void
  className?: string
}

/** The template request box at the top of the editor (TMPL-58, TMPL-62): one free-text field
 *  for a description, a pasted post or what to change, the 글 작성 모델 and what one request
 *  costs, the running state with its 취소, and after an answer the separated 지침 material and
 *  요청 전으로 되돌리기. */
export function TemplateRequestBox({
  request,
  draft,
  templateId,
  startOpen,
  atCap,
  sample,
  onRemoveSample,
  className,
}: TemplateRequestBoxProps) {
  const { t, i18n } = useTranslation('templates')
  const id = useId()
  const fieldId = `${id}-request`
  const [open, setOpen] = useState(startOpen)
  const [text, setText] = useState('')
  // What the last press sent: shown while it runs, and handed back when it fails or is
  // cancelled so it can be sent again. A finished request empties the box.
  const [sent, setSent] = useState('')
  const write = useStageSelection('write')
  const estimate = useTemplateRequestEstimate(write.selected)
  const locale: Locale = isLocale(i18n.resolvedLanguage) ? i18n.resolvedLanguage : 'ko'
  const { phase, running } = request

  const shown =
    text !== ''
      ? text
      : phase === 'running' || phase === 'failed' || phase === 'cancelled'
        ? sent
        : ''
  const trimmed = shown.trim()
  const left = remainingChars(shown, TEMPLATE_REQUEST_MAX_CHARS)
  const attached = sample?.kind === 'post' ? sample : undefined
  const model = write.selected
    ? (write.models.find((candidate) => sameRef(candidate.ref, write.selected!))?.label ??
      write.selected.modelId)
    : ''
  const blocked = !write.selected || write.isPending || atCap
  const sendDisabled =
    blocked || running || request.startPending || left < 0 || (trimmed === '' && !attached)

  const send = async () => {
    if (sendDisabled || !write.selected) return
    const started = await request.send({
      writeModel: write.selected,
      language: locale,
      text: trimmed,
      draft,
      templateId,
      samplePostSlug: attached?.slug,
    })
    if (started) {
      setSent(trimmed)
      setText('')
    }
  }

  if (!open) {
    return (
      <div className={className}>
        <Button variant="secondary" onClick={() => setOpen(true)}>
          {t('request.open')}
        </Button>
      </div>
    )
  }

  return (
    <section aria-labelledby={`${fieldId}-label`} className={className}>
      <div className="flex items-center justify-between gap-3">
        <Typography variant="fieldTitle" as="label" id={`${fieldId}-label`} htmlFor={fieldId}>
          {t('request.label')}
        </Typography>
        <Button variant="ghost" onClick={() => setOpen(false)} disabled={running}>
          {t('request.collapse')}
        </Button>
      </div>

      {attached && (
        <div className="mt-2 flex items-center gap-1">
          <Typography
            variant="meta"
            as="span"
            className="bg-surface-raised min-w-0 truncate rounded-sm px-2 py-1"
          >
            {t('request.sample', { title: attached.title || t('request.sampleUntitled') })}
          </Typography>
          <Button
            variant="ghost"
            size="icon"
            aria-label={t('request.sampleRemove')}
            onClick={onRemoveSample}
            disabled={running}
          >
            <X aria-hidden="true" className="size-4" />
          </Button>
        </div>
      )}
      {sample?.kind === 'missing' && (
        <Typography variant="meta" as="p" role="status" className="text-content-secondary mt-2">
          {t('request.sampleMissing')}
        </Typography>
      )}

      {/* One stated reason, above the field so a phone keyboard never hides it (THEME-31). */}
      {!write.isPending && !write.selected ? (
        <Typography variant="body" as="p" role="status" className="text-content-secondary mt-2">
          {t('request.noModel')}{' '}
          <Link to="/ai-models" className="underline">
            {t('request.chooseModel')}
          </Link>
        </Typography>
      ) : atCap ? (
        <Typography variant="body" as="p" role="status" className="text-content-secondary mt-2">
          {t('request.atCap')}
        </Typography>
      ) : null}

      <Textarea
        id={fieldId}
        value={shown}
        rows={3}
        autoGrow
        disabled={running || blocked}
        placeholder={t('request.placeholder')}
        onChange={(event) => setText(event.target.value)}
        className="max-h-field mt-2"
      />
      <FieldCount left={left} />

      <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
        {model && (
          <Typography variant="meta" as="p" className="text-content-secondary">
            {t('request.model', { model })}
            {estimate.free
              ? ` · ${t('request.free')}`
              : estimate.credits !== undefined
                ? ` · ${t('request.credits', { credits: estimate.credits })}`
                : ''}
          </Typography>
        )}
        {running ? (
          <Button variant="secondary" onClick={request.cancel}>
            {t('request.cancel')}
          </Button>
        ) : (
          <Button
            variant="cta"
            onClick={() => void send()}
            disabled={sendDisabled}
            pending={request.startPending}
          >
            {t('request.send')}
          </Button>
        )}
      </div>

      {running && (
        <Typography variant="meta" as="p" role="status" className="text-content-secondary mt-2">
          {t('request.running')}
        </Typography>
      )}
      {request.startFailure && (
        <Notice tone="danger" role="alert" className="mt-3">
          <AppFailureMessage failure={request.startFailure} />
        </Notice>
      )}
      {phase === 'failed' && (
        <Notice tone="danger" role="alert" className="mt-3">
          <div className="grid gap-2">
            {request.job?.failure ? (
              <AppFailureMessage failure={request.job.failure} />
            ) : (
              <span>{t('request.failed')}</span>
            )}
            {request.job?.failure?.reason === 'INSUFFICIENT_CREDITS' && (
              <Link to="/plans" className={buttonStyles({ variant: 'secondary' })}>
                {t('request.plans')}
              </Link>
            )}
          </div>
        </Notice>
      )}
      {phase === 'cancelled' && (
        <Typography variant="meta" as="p" role="status" className="text-content-secondary mt-2">
          {t('request.cancelled')}
        </Typography>
      )}
      {request.canUndo && (
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <Typography variant="meta" as="p" role="status" className="text-content-secondary">
            {t('request.applied')}
          </Typography>
          <Button variant="secondary" onClick={request.undo}>
            {t('request.undo')}
          </Button>
        </div>
      )}
      {request.wishes.length > 0 && (
        <section aria-labelledby={`${fieldId}-wishes`} className="mt-4">
          <Typography variant="label" as="h3" id={`${fieldId}-wishes`}>
            {t('request.wishesHeading')}
          </Typography>
          <Typography variant="meta" as="p" className="text-content-tertiary mt-1">
            {t('request.wishesHelp')}
          </Typography>
          <ul className="mt-2 list-disc space-y-1 pl-5">
            {request.wishes.map((wish) => (
              <li key={wish}>
                <Typography variant="body" as="span">
                  {wish}
                </Typography>
              </li>
            ))}
          </ul>
          <Link
            to="/guidelines"
            search={{ new: 1 }}
            className={buttonStyles({ variant: 'secondary', className: 'mt-3' })}
          >
            {t('request.makeGuideline')}
          </Link>
        </section>
      )}
    </section>
  )
}
