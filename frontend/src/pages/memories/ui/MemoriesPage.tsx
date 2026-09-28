import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Pencil } from 'lucide-react'
import { useMemories, useUpdateMemoryCall, type Memory } from '@/entities/memory'
import { useSession } from '@/entities/session'
import { CreateMemorySheet } from '@/features/create-memory'
import { DeleteMemoryButton } from '@/features/delete-memory'
import { MemoryEditForm } from '@/features/edit-memory'
import { ActionBar, Badge, Button, Notice, Typography, pageStyles } from '@/shared/ui'

/** The account's 기억 (MEM-24). Composition only — every action is its own feature.
 *
 *  Nothing on this screen calls a model or enqueues a job: a memory is an authored fact, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5], MEM-23). The list is
 *  rendered in the server's order because that order is the one a post that opted in would be
 *  given: most recently used first, then most recently created. */
export function MemoriesPage() {
  const { t } = useTranslation(['memories', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { memories, isPending, isError, isFetching, refetch } = useMemories(ownerId)

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Typography variant="display">{t('title', { ns: 'memories' })}</Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.description', { ns: 'memories' })}
      </Typography>

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed', { ns: 'memories' })}</span>
          <Button
            variant="ghost"
            onClick={refetch}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      )}
      {!isError && isPending && (
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      )}

      {!isError && !isPending && (
        <>
          {memories.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-labelledby="memories-heading" className="mt-8">
              <Typography variant="title" id="memories-heading">
                {t('page.saved', { ns: 'memories' })}
              </Typography>
              <Typography variant="body" as="p" className="text-content-secondary mt-1">
                {t('page.order', { ns: 'memories' })}
              </Typography>
              <ul className="divide-divider mt-3 divide-y">
                {memories.map((memory) => (
                  <MemoryRow key={memory.id} ownerId={ownerId} memory={memory} />
                ))}
              </ul>
            </section>
          )}

          {/* The page is the list; authoring happens behind this one trigger, the shape every
              sibling directory uses (MEM-24, THEME-24). */}
          <ActionBar
            dock="list"
            ariaLabel={t('create.dockAria', { ns: 'memories' })}
            className="mt-auto"
          >
            <CreateMemorySheet ownerId={ownerId} />
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** Copy, not a row: nothing here creates a memory the user did not author — there is no seeded
 *  library and nothing is inferred (MEM-23). */
function EmptyState() {
  const { t } = useTranslation('memories')
  return (
    <section aria-labelledby="memories-empty-heading" className="mt-8">
      <Typography variant="title" id="memories-empty-heading">
        {t('page.empty')}
      </Typography>
      <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
        {t('page.emptyHelp')}
      </Typography>
      <Typography variant="body" className="text-content-primary mt-3">
        {t('page.example')}
      </Typography>
    </section>
  )
}

/** One saved memory (MEM-30): its text with 수정 and 삭제 at the end of the text line, over one
 *  line of its kind and tags. The list is read to see what the account knows, so a fact keeps to
 *  one short row; 수정 turns the whole row into one form for the text, the kind and the tags. */
function MemoryRow({ ownerId, memory }: { ownerId: string; memory: Memory }) {
  const { t } = useTranslation(['memories', 'common'])
  const update = useUpdateMemoryCall(ownerId, memory.id)
  const [editing, setEditing] = useState(false)

  if (editing)
    return (
      <li className="py-3">
        <MemoryEditForm
          memory={memory}
          save={update.save}
          errorMessage={update.errorMessage}
          pending={update.isPending}
          onDone={() => setEditing(false)}
        />
      </li>
    )

  return (
    <li className="py-3">
      <div className="flex flex-wrap items-start gap-1">
        <Typography
          variant="body"
          className="text-content-primary min-w-0 flex-1 pt-2 whitespace-pre-wrap"
        >
          {memory.text}
        </Typography>
        <Button
          variant="ghost"
          size="icon"
          aria-label={t('action.editNamed', {
            ns: 'common',
            name: t('edit.text', { ns: 'memories' }),
          })}
          onClick={() => setEditing(true)}
          className="shrink-0"
        >
          <Pencil className="size-4" aria-hidden />
        </Button>
        <DeleteMemoryButton ownerId={ownerId} memoryId={memory.id} />
      </div>
      <div className="mt-1 flex flex-wrap items-center gap-2">
        <Badge tone="accent">{t(`kind.${memory.kind}`, { ns: 'memories' })}</Badge>
        {memory.tags.map((tag) => (
          <Badge key={tag}>{tag}</Badge>
        ))}
        {memory.tags.length === 0 && (
          <Typography variant="meta" as="span">
            {t('page.noTags', { ns: 'memories' })}
          </Typography>
        )}
      </div>
    </li>
  )
}
