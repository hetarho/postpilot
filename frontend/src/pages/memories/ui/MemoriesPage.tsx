import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Pencil } from 'lucide-react'
import { useMemories, useUpdateMemoryCall, type Memory } from '@/entities/memory'
import { useSession } from '@/entities/session'
import { CreateMemorySheet } from '@/features/create-memory'
import { DeleteMemoryButton } from '@/features/delete-memory'
import { MemoryEditForm } from '@/features/edit-memory'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

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
  const page = useRef<HTMLElement>(null)
  const list = useRef<HTMLUListElement>(null)
  // A confirmed delete hands focus back to the trash button, then the re-read list removes the row
  // holding it, which drops focus to <body> and sends a keyboard user to the top of the page. The
  // delete does not await that re-read, so the move waits until the row has left the list.
  const [removed, setRemoved] = useState<Removal | null>(null)
  const settled = useRef<Removal | null>(null)

  useEffect(() => {
    if (!removed || settled.current === removed) return
    if (memories.some((memory) => memory.id === removed.id)) return
    settled.current = removed
    // Only while focus is lost, the rule Editable follows: focus the user moved elsewhere in the
    // meantime stays where they put it.
    const active = document.activeElement
    if (active && active !== document.body) return
    const target =
      memories.length > 0
        ? buttonNamed(
            list.current?.children[Math.min(removed.index, memories.length - 1)],
            t('delete.aria', { ns: 'memories' }),
          )
        : buttonNamed(page.current, t('create.open', { ns: 'memories' }))
    target?.focus()
  }, [removed, memories, t])

  return (
    <main
      ref={page}
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}
    >
      {/* Guide: the hub's title and the 기억 chip say where this is; the page leads with what the
          list IS — what the AI remembers — and the facts themselves. */}
      <h1 className="sr-only">{t('title', { ns: 'memories' })}</h1>
      <Typography variant="display" as="p" aria-hidden="true">
        {t('page.lead', { ns: 'memories' })}
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
            <section aria-label={t('page.saved', { ns: 'memories' })} className="mt-6">
              <h2 className="flex items-baseline gap-2">
                <span className={typographyStyles({ variant: 'title' })}>
                  {t('page.saved', { ns: 'memories' })}
                </span>
                <span className={typographyStyles({ variant: 'label' })}>{memories.length}</span>
              </h2>
              <Typography variant="meta" as="p" className="mt-1">
                {t('page.order', { ns: 'memories' })}
              </Typography>
              {/* Guide: each fact on its own card, its kind and tags under it. */}
              <ul ref={list} className="mt-3 grid gap-3 lg:grid-cols-2">
                {memories.map((memory, index) => (
                  <MemoryRow
                    key={memory.id}
                    ownerId={ownerId}
                    memory={memory}
                    onDeleted={() => setRemoved({ id: memory.id, index })}
                  />
                ))}
              </ul>
            </section>
          )}

          {/* The page is the list; authoring happens behind this one trigger, the shape every
              sibling directory uses (MEM-24, THEME-24). */}
          {/* The full width on a phone and a right-aligned pill from `sm:`, like every section's
              creation button (THEME-67). */}
          <ActionBar
            dock="list"
            ariaLabel={t('create.dockAria', { ns: 'memories' })}
            className="mt-auto w-full sm:w-fit"
          >
            <CreateMemorySheet ownerId={ownerId} className="w-full sm:w-auto" />
          </ActionBar>
        </>
      )}
    </main>
  )
}

/** A deleted memory and the position its row held. */
type Removal = { id: string; index: number }

/** The button named `name` inside `root`. The rows repeat the same names and carry no ids, so a
 *  name means something only within the one row (or the page) that should hold it. */
function buttonNamed(root: Element | null | undefined, name: string) {
  return Array.from(root?.querySelectorAll('button') ?? []).find(
    (button) => (button.getAttribute('aria-label') ?? button.textContent?.trim()) === name,
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
function MemoryRow({
  ownerId,
  memory,
  onDeleted,
}: {
  ownerId: string
  memory: Memory
  onDeleted: () => void
}) {
  const { t } = useTranslation(['memories', 'common'])
  const update = useUpdateMemoryCall(ownerId, memory.id)
  // Closing the form takes the focused 취소 or 저장 out of the document, which drops focus to
  // <body>, so a close carries the move back to the pencil. State, not a ref: `onDone` is handed
  // to the form during render.
  const [form, setForm] = useState({ editing: false, refocus: false })
  const pencil = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    // Only while focus is lost, as in Editable: focus the user moved elsewhere stays there.
    const active = document.activeElement
    if (!form.refocus || (active && active !== document.body)) return
    // A collapsing form must not scroll the page to the pencil.
    pencil.current?.focus({ preventScroll: true })
  }, [form])

  if (form.editing)
    return (
      <li className={cardStyles({ padding: 'sm', className: 'shadow-sm' })}>
        <MemoryEditForm
          memory={memory}
          save={update.save}
          errorMessage={update.errorMessage}
          pending={update.isPending}
          // Closing when already closed changes nothing, as before.
          onDone={() =>
            setForm((current) => (current.editing ? { editing: false, refocus: true } : current))
          }
        />
      </li>
    )

  return (
    <li className={cardStyles({ padding: 'sm', className: 'shadow-sm' })}>
      <div className="flex flex-wrap items-start gap-1">
        <Typography
          variant="fieldTitle"
          as="p"
          className="text-content-primary min-w-0 flex-1 pt-2 whitespace-pre-wrap"
        >
          {memory.text}
        </Typography>
        <Button
          ref={pencil}
          variant="ghost"
          size="icon"
          aria-label={t('action.editNamed', {
            ns: 'common',
            name: t('edit.text', { ns: 'memories' }),
          })}
          onClick={() => setForm({ editing: true, refocus: false })}
          className="shrink-0"
        >
          <Pencil className="size-4" aria-hidden />
        </Button>
        <DeleteMemoryButton ownerId={ownerId} memoryId={memory.id} onDeleted={onDeleted} />
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
