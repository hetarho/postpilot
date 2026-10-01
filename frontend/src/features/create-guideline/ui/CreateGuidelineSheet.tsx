import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  GuidelineScopeField,
  GuidelineTitleField,
  canSaveGuideline,
  globalScope,
  remainingGuidelineChars,
  remainingGuidelineTitleChars,
  useCreateGuidelineCall,
  type GuidelineKind,
  type GuidelineScope,
} from '@/entities/guideline'
import { Button, FieldLabel, FieldMessage, Sheet, Textarea, Typography } from '@/shared/ui'

/** The directory's one committing action and the overlay it opens (GUIDE-20). The trigger is
 *  rendered here rather than by the page so the open state stays with the form it opens, and so
 *  there is exactly one of each at every width. */
export function CreateGuidelineSheet({
  ownerId,
  kind = 'post',
  initialOpen = false,
  onClosed,
  className,
}: {
  ownerId: string
  /** A post's 지침 or a clip's 영상 지침; the copy and the scope follow it. */
  kind?: GuidelineKind
  /** Open, empty, on the first render: a link asked for a new guideline (TMPL-61). */
  initialOpen?: boolean
  onClosed?: () => void
  className?: string
}) {
  const { t } = useTranslation('guidelines')
  const [open, setOpen] = useState(initialOpen)
  const close = () => {
    setOpen(false)
    onClosed?.()
  }
  return (
    <>
      <Button variant="cta" className={className} onClick={() => setOpen(true)}>
        {t(kind === 'clip' ? 'create.openClip' : 'create.open')}
      </Button>
      {open && <CreateGuidelinePanel ownerId={ownerId} kind={kind} onClose={close} />}
    </>
  )
}

/** Mounted only while the sheet is open, so the rule starts blank on each visit and a refused
 *  attempt's message does not greet the next one. */
function CreateGuidelinePanel({
  ownerId,
  kind,
  onClose,
}: {
  ownerId: string
  kind: GuidelineKind
  onClose: () => void
}) {
  const { t } = useTranslation(['guidelines', 'common'])
  const id = useId()
  const titleId = `${id}-title`
  const textId = `${id}-text`
  const countId = `${id}-count`
  const helpId = `${id}-help`
  const errorId = `${id}-error`
  const [title, setTitle] = useState('')
  const [text, setText] = useState('')
  // 전역 is the default because a guideline is meant to apply everywhere unless it is narrowed.
  const [scope, setScope] = useState<GuidelineScope>(globalScope)
  const create = useCreateGuidelineCall(ownerId, kind)

  const textExceeded = remainingGuidelineChars(text) < 0
  const disabled =
    !canSaveGuideline(text, scope) || remainingGuidelineTitleChars(title) < 0 || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    try {
      await create.create({ title, text, scope })
      onClose()
    } catch {
      // The mutation's message renders under the field, inside the still-open sheet: a refusal
      // (too long, duplicate, the account cap) is something to fix here, not to lose.
    }
  }

  return (
    <Sheet open labelledBy={titleId} onClose={create.isPending ? () => {} : onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t(kind === 'clip' ? 'create.openClip' : 'create.title', { ns: 'guidelines' })}
      </Typography>
      <form onSubmit={(event) => void submit(event)} className="mt-4">
        {/* Optional and first: the list shows a rule by its title (GUIDE-46), while the text below
            is what the writer is given. */}
        <GuidelineTitleField
          id={`${id}-name`}
          value={title}
          onChange={setTitle}
          disabled={create.isPending}
          autoFocus
        />
        <FieldLabel htmlFor={textId} className="mt-4">
          {t('create.text', { ns: 'guidelines' })}
        </FieldLabel>
        <Textarea
          id={textId}
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={3}
          autoGrow
          disabled={create.isPending}
          placeholder={t('create.textPlaceholder', { ns: 'guidelines' })}
          aria-invalid={textExceeded || create.isError || undefined}
          aria-describedby={`${countId} ${helpId}${create.isError ? ` ${errorId}` : ''}`}
          className="mt-1"
        />
        <CharCount id={countId} value={text} />
        <Typography variant="body" as="p" id={helpId} className="text-content-secondary mt-2">
          {t('create.help', { ns: 'guidelines' })}
        </Typography>

        <GuidelineScopeField
          ownerId={ownerId}
          kind={kind}
          value={scope}
          onChange={setScope}
          disabled={create.isPending}
          className="mt-6"
        />

        {create.isError && (
          <FieldMessage id={errorId} className="mt-3">
            {create.errorMessage}
          </FieldMessage>
        )}
        {/* In flow after the last field, NOT in a pinned footer: the panel is anchored to the
            layout viewport, which the software keyboard does not resize, so a pinned footer sits
            behind the keyboard exactly while this field is being typed into. */}
        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={create.isPending} onClick={onClose}>
            {t('action.cancel', { ns: 'common' })}
          </Button>
          <Button type="submit" variant="cta" disabled={disabled} pending={create.isPending}>
            {t('create.submit', { ns: 'guidelines' })}
          </Button>
        </div>
      </form>
    </Sheet>
  )
}

/** Counts down rather than up: what a writer needs to know is how much room is left, and the count
 *  goes negative rather than clamping so an over-long paste says how much to cut. */
function CharCount({ id, value }: { id: string; value: string }) {
  const { t } = useTranslation('common')
  const left = remainingGuidelineChars(value)
  return left < 0 ? (
    <FieldMessage id={id} role="status" className="mt-2">
      {t('count.exceeded', { count: -left })}
    </FieldMessage>
  ) : (
    <Typography variant="meta" as="p" id={id} className="mt-2">
      {t('count.remaining', { count: left })}
    </Typography>
  )
}
