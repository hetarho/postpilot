import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import {
  MemoryKindField,
  MemoryTagsField,
  canSaveMemory,
  parseMemoryTags,
  remainingMemoryChars,
  useCreateMemoryCall,
  type MemoryKind,
} from '@/entities/memory'
import { Button, FieldLabel, FieldMessage, Sheet, Textarea, Typography } from '@/shared/ui'

/** The directory's one committing action and the overlay it opens (MEM-24). The trigger is
 *  rendered here rather than by the page so the open state stays with the form it opens, and so
 *  there is exactly one of each at every width.
 *
 *  A fact the author knows on day one should not require generating a post first (MEM-25), which
 *  is what this is for: the same field rules as an approval, with no post behind it. */
export function CreateMemorySheet({ ownerId, className }: { ownerId: string; className?: string }) {
  const { t } = useTranslation('memories')
  const [open, setOpen] = useState(false)
  return (
    <>
      <Button variant="cta" className={className} onClick={() => setOpen(true)}>
        {t('create.open')}
      </Button>
      {open && <CreateMemoryPanel ownerId={ownerId} onClose={() => setOpen(false)} />}
    </>
  )
}

/** Mounted only while the sheet is open, so it starts blank on each visit and a refused
 *  attempt's message does not greet the next one. */
function CreateMemoryPanel({ ownerId, onClose }: { ownerId: string; onClose: () => void }) {
  const { t } = useTranslation(['memories', 'common'])
  const id = useId()
  const titleId = `${id}-title`
  const textId = `${id}-text`
  const countId = `${id}-count`
  const helpId = `${id}-help`
  const errorId = `${id}-error`
  const [text, setText] = useState('')
  // 취향 first: a standing preference is the kind a fact is most often, and it is also the one
  // whose retrieval half (a candidate for every post) is the safe reading of an unsure choice.
  const [kind, setKind] = useState<MemoryKind>('preference')
  const [tagsValue, setTagsValue] = useState('')
  const create = useCreateMemoryCall(ownerId)

  const tags = parseMemoryTags(tagsValue)
  const textExceeded = remainingMemoryChars(text) < 0
  const disabled = !canSaveMemory(text, tags) || create.isPending

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (disabled) return
    try {
      await create.create(text, kind, tags)
      onClose()
    } catch {
      // The mutation's message renders under the field, inside the still-open sheet: a refusal
      // (too long, too many tags, the account cap) is something to fix here, not to lose.
    }
  }

  return (
    <Sheet open labelledBy={titleId} onClose={create.isPending ? () => {} : onClose}>
      <Typography variant="title" as="h2" id={titleId}>
        {t('create.title', { ns: 'memories' })}
      </Typography>
      <form onSubmit={(event) => void submit(event)} className="mt-4">
        <FieldLabel htmlFor={textId}>{t('create.text', { ns: 'memories' })}</FieldLabel>
        <Textarea
          id={textId}
          value={text}
          onChange={(event) => setText(event.target.value)}
          rows={3}
          autoGrow
          autoFocus
          disabled={create.isPending}
          placeholder={t('create.textPlaceholder', { ns: 'memories' })}
          aria-invalid={textExceeded || create.isError || undefined}
          aria-describedby={`${countId} ${helpId}${create.isError ? ` ${errorId}` : ''}`}
          className="mt-1"
        />
        <CharCount id={countId} value={text} />
        <Typography variant="body" as="p" id={helpId} className="text-content-secondary mt-2">
          {t('create.help', { ns: 'memories' })}
        </Typography>

        <MemoryKindField
          id={`${id}-kind`}
          value={kind}
          onChange={setKind}
          disabled={create.isPending}
          className="mt-6"
        />
        <MemoryTagsField
          id={`${id}-tags`}
          value={tagsValue}
          onChange={setTagsValue}
          disabled={create.isPending}
          className="mt-4"
        />

        {create.isError && (
          <FieldMessage id={errorId} className="mt-3">
            {create.errorMessage}
          </FieldMessage>
        )}
        {/* In flow after the last field, NOT in a pinned footer: the panel is anchored to the
            layout viewport, which the software keyboard does not resize. */}
        <div className="mt-6 flex flex-wrap justify-end gap-2">
          <Button variant="ghost" disabled={create.isPending} onClick={onClose}>
            {t('action.cancel', { ns: 'common' })}
          </Button>
          <Button type="submit" variant="cta" disabled={disabled} pending={create.isPending}>
            {t('create.submit', { ns: 'memories' })}
          </Button>
        </div>
      </form>
    </Sheet>
  )
}

/** Counts down rather than up: what a writer needs to know is how much room is left, and the
 *  count goes negative rather than clamping so an over-long paste says how much to cut. */
function CharCount({ id, value }: { id: string; value: string }) {
  const { t } = useTranslation('common')
  const left = remainingMemoryChars(value)
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
