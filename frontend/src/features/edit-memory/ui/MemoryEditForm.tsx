import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  MEMORY_LIMITS,
  MemoryKindField,
  MemoryTagsField,
  formatMemoryTags,
  parseMemoryTags,
  remainingMemoryChars,
  type Memory,
  type MemoryKind,
  type MemoryPatch,
} from '@/entities/memory'
import { Button, FieldLabel, FieldMessage, Textarea, Typography } from '@/shared/ui'

/** One memory's whole edit (MEM-30): the text, the kind and the tags in one form, saved together
 *  by one 저장. The kind and the tags were always one decision about retrieval — the kind says
 *  whether tags gate the fact at all (MEM-6) — and the text is the fact they describe.
 *
 *  The save carries only what differs from the memory as it was when 수정 was pressed, and a form
 *  that changed nothing leaves without a call. A refused save keeps every typed value: the refusal
 *  a memory edit meets is a text the account already holds, and fixing it means changing what is
 *  typed. */
export function MemoryEditForm({
  memory,
  save,
  errorMessage,
  pending,
  onDone,
}: {
  memory: Memory
  save: (patch: MemoryPatch) => Promise<unknown>
  errorMessage: string
  pending: boolean
  /** Leaves the form: after a save that went through, on 취소, and on a save with no change. */
  onDone: () => void
}) {
  const { t } = useTranslation(['memories', 'common'])
  const id = useId()
  // Seeded once, at the mount 수정 gives this form. Deliberately not resynced from `memory`: while
  // someone is typing here, their draft outranks a value arriving from a refetch.
  const [text, setText] = useState(memory.text)
  const [kind, setKind] = useState<MemoryKind>(memory.kind)
  const [tagsValue, setTagsValue] = useState(formatMemoryTags(memory.tags))
  const [failed, setFailed] = useState(false)
  const textId = `${id}-text`
  const countId = `${id}-count`
  const errorId = `${id}-error`

  const left = remainingMemoryChars(text)
  const exceeded = left < 0
  const tags = parseMemoryTags(tagsValue)
  const showSaveError = failed && Boolean(errorMessage)
  const disabled = pending || exceeded || !text.trim() || tags.length > MEMORY_LIMITS.tags

  const commit = async () => {
    if (disabled) return
    const patch = changes(memory, { text: text.trim(), kind, tags })
    if (!patch) {
      onDone()
      return
    }
    try {
      await save(patch)
      setFailed(false)
      onDone()
    } catch {
      // Stay open: the message renders below and every typed value is still here to fix.
      setFailed(true)
    }
  }

  return (
    <div>
      <FieldLabel htmlFor={textId}>{t('edit.text', { ns: 'memories' })}</FieldLabel>
      <Textarea
        id={textId}
        value={text}
        onChange={(event) => setText(event.target.value)}
        rows={2}
        autoGrow
        autoFocus
        aria-invalid={exceeded || failed || undefined}
        aria-describedby={`${countId}${showSaveError ? ` ${errorId}` : ''}`}
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
      <MemoryKindField
        id={`${id}-kind`}
        value={kind}
        onChange={setKind}
        disabled={pending}
        className="mt-4"
      />
      <MemoryTagsField
        id={`${id}-tags`}
        value={tagsValue}
        onChange={setTagsValue}
        disabled={pending}
        className="mt-4"
      />
      {showSaveError && (
        <FieldMessage id={errorId} className="mt-2">
          {errorMessage}
        </FieldMessage>
      )}
      <div className="mt-3 flex gap-2">
        <Button onClick={() => void commit()} disabled={disabled} pending={pending}>
          {t('action.save', { ns: 'common' })}
        </Button>
        <Button variant="ghost" onClick={onDone} disabled={pending}>
          {t('action.cancel', { ns: 'common' })}
        </Button>
      </div>
    </div>
  )
}

/** The parts of `next` that differ from `memory`, or null when nothing does. Tags compare as the
 *  parsed list in order, which is how the server stores them. */
function changes(
  memory: Memory,
  next: { text: string; kind: MemoryKind; tags: string[] },
): MemoryPatch | null {
  const patch: MemoryPatch = {}
  if (next.text !== memory.text) patch.text = next.text
  if (next.kind !== memory.kind) patch.kind = next.kind
  const sameTags =
    next.tags.length === memory.tags.length && next.tags.every((tag, at) => tag === memory.tags[at])
  if (!sameTags) patch.tags = next.tags
  return Object.keys(patch).length > 0 ? patch : null
}
