import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  MEMORY_LIMITS,
  MemoryKindField,
  MemoryTagsField,
  formatMemoryTags,
  parseMemoryTags,
  type Memory,
  type MemoryKind,
} from '@/entities/memory'
import { Badge, Button, Editable, FieldMessage, Typography } from '@/shared/ui'

/** The kind and the tags, read first and saved TOGETHER. They are one edit because they are one
 *  decision about how this fact is retrieved: the kind says whether tags gate it at all (MEM-6),
 *  so saving a kind without its tags would leave a `place` fact reachable by nothing. */
export function EditableMemoryFacets({
  memory,
  save,
  errorMessage,
  pending,
  className,
}: {
  memory: Memory
  save: (kind: MemoryKind, tags: string[]) => Promise<unknown>
  errorMessage: string
  pending: boolean
  className?: string
}) {
  const { t } = useTranslation(['memories', 'common'])
  const label = t('edit.facets', { ns: 'memories' })
  return (
    <Editable
      className={className}
      editLabel={t('action.editNamed', { ns: 'common', name: label })}
      edit={(exit) => (
        <MemoryFacetsEditor
          memory={memory}
          save={save}
          errorMessage={errorMessage}
          pending={pending}
          exit={exit}
        />
      )}
    >
      <div className="flex flex-wrap items-center gap-2">
        <Badge tone="accent">{t(`kind.${memory.kind}`, { ns: 'memories' })}</Badge>
        {memory.tags.map((tag) => (
          <Badge key={tag}>{tag}</Badge>
        ))}
        {memory.tags.length === 0 && (
          <Typography variant="meta" as="span">
            {t('edit.noTags', { ns: 'memories' })}
          </Typography>
        )}
      </div>
    </Editable>
  )
}

function MemoryFacetsEditor({
  memory,
  save,
  errorMessage,
  pending,
  exit,
}: {
  memory: Memory
  save: (kind: MemoryKind, tags: string[]) => Promise<unknown>
  errorMessage: string
  pending: boolean
  exit: () => void
}) {
  const { t } = useTranslation('common')
  const id = useId()
  const [kind, setKind] = useState<MemoryKind>(memory.kind)
  const [tagsValue, setTagsValue] = useState(formatMemoryTags(memory.tags))
  const [failed, setFailed] = useState(false)

  const tags = parseMemoryTags(tagsValue)
  const disabled = pending || tags.length > MEMORY_LIMITS.tags

  const commit = async () => {
    if (disabled) return
    try {
      await save(kind, tags)
      setFailed(false)
      exit()
    } catch {
      setFailed(true)
    }
  }

  return (
    <div>
      <MemoryKindField id={`${id}-kind`} value={kind} onChange={setKind} disabled={pending} />
      <MemoryTagsField
        id={`${id}-tags`}
        value={tagsValue}
        onChange={setTagsValue}
        disabled={pending}
        className="mt-4"
      />
      {failed && errorMessage && <FieldMessage className="mt-2">{errorMessage}</FieldMessage>}
      <div className="mt-3 flex gap-2">
        <Button onClick={() => void commit()} disabled={disabled} pending={pending}>
          {t('action.save')}
        </Button>
        <Button variant="ghost" onClick={exit} disabled={pending}>
          {t('action.cancel')}
        </Button>
      </div>
    </div>
  )
}
