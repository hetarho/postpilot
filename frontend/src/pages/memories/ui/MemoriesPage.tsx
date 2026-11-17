import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Pencil } from 'lucide-react'
import { MEMORY_KINDS, useMemories, useUpdateMemoryCall, type Memory } from '@/entities/memory'
import { memoriesIllustration } from '@/shared/assets'
import { useSession } from '@/entities/session'
import { CreateMemorySheet } from '@/features/create-memory'
import { DeleteMemoryButton } from '@/features/delete-memory'
import { MemoryEditForm } from '@/features/edit-memory'
import {
  ActionBar,
  Badge,
  Button,
  ChipToggle,
  Notice,
  LoadingState,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** Guide: 기억 — what the AI knows about the owner, kept like a notebook: the facts grouped by what
 *  they are about (취향, 설정, 장소, 인물, 이력), each a plain line with its tags and the posts it was
 *  learned from, and the tags as a row that narrows the notebook to one subject. The account's 기억
 *  (MEM-24). Composition only — every action is its own feature.
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
  const search = useSearch({ strict: false }) as { new?: unknown }
  const navigate = useNavigate()
  // `?new=1` is 홈's 기억 더하기: the sheet opens as the page mounts and the address drops it.
  const [requested] = useState(search.new === 1 || search.new === '1')
  useEffect(() => {
    if (requested) void navigate({ to: '/memories', search: {}, replace: true })
  }, [requested, navigate])
  const [tag, setTag] = useState<string>()
  const tags = Array.from(new Set(memories.flatMap((memory) => memory.tags)))
  const shown = tag ? memories.filter((memory) => memory.tags.includes(tag)) : memories
  // The notebook's own order: by kind, then the server's order within one.
  const ordered = MEMORY_KINDS.flatMap((kind) => shown.filter((memory) => memory.kind === kind))
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
    // The rows as they stand now, in the notebook's order (grouped by kind, narrowed by a tag).
    const rows = list.current?.querySelectorAll('[data-memory-row]') ?? []
    const target =
      rows.length > 0
        ? buttonNamed(
            rows[Math.min(removed.index, rows.length - 1)],
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
      {/* Guide: the hub's title, the 기억 chip and the kit's head (what the AI knows about the owner)
          say where this is; the page is the facts themselves. */}
      <h1 className="sr-only">{t('title', { ns: 'memories' })}</h1>

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
        <LoadingState
          variant="cards"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8 [&>.loading-reveal]:gap-3 sm:[&>.loading-reveal]:grid-cols-1 lg:[&>.loading-reveal]:grid-cols-2"
        />
      )}

      {!isError && !isPending && (
        <>
          {memories.length === 0 ? (
            <EmptyState />
          ) : (
            <section aria-label={t('page.saved', { ns: 'memories' })} className="mt-4">
              {tags.length > 0 && (
                // Finding a fact: the owner's own tags narrow the notebook to one subject.
                <nav
                  aria-label={t('page.tags', { ns: 'memories' })}
                  className="-mx-4 flex gap-2 overflow-x-auto overscroll-x-contain px-4 pb-2 sm:mx-0 sm:flex-wrap sm:px-0"
                >
                  {[undefined, ...tags].map((value) => (
                    <ChipToggle
                      key={value ?? ''}
                      pressed={tag === value}
                      onClick={() => setTag(value)}
                      className="min-h-10 rounded-full px-4"
                    >
                      {value === undefined ? t('page.allTags', { ns: 'memories' }) : `#${value}`}
                    </ChipToggle>
                  ))}
                </nav>
              )}
              <ul ref={list} className="mt-4 gap-4 sm:columns-2 lg:columns-2">
                {MEMORY_KINDS.map((kind) => {
                  const members = shown.filter((memory) => memory.kind === kind)
                  if (members.length === 0) return null
                  return (
                    <li
                      key={kind}
                      className={cardStyles({
                        padding: 'none',
                        className: 'mb-4 break-inside-avoid overflow-hidden',
                      })}
                    >
                      <h2 className="border-divider flex items-center gap-2 border-b px-5 py-3">
                        <span
                          aria-hidden="true"
                          className="bg-kit-memory size-2.5 shrink-0 rounded-full"
                        />
                        <span className={typographyStyles({ variant: 'fieldTitle' })}>
                          {t(`kind.${kind}`, { ns: 'memories' })}
                        </span>
                        <span className={typographyStyles({ variant: 'label' })}>
                          {members.length}
                        </span>
                      </h2>
                      <ul className="divide-divider divide-y">
                        {members.map((memory) => (
                          <MemoryRow
                            key={memory.id}
                            ownerId={ownerId}
                            memory={memory}
                            onDeleted={() =>
                              setRemoved({ id: memory.id, index: ordered.indexOf(memory) })
                            }
                          />
                        ))}
                      </ul>
                    </li>
                  )
                })}
              </ul>
            </section>
          )}
        </>
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
        <CreateMemorySheet ownerId={ownerId} defaultOpen={requested} className="w-full sm:w-auto" />
      </ActionBar>
    </main>
  )
}

/** A deleted memory and the position its row held in the notebook. */
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
    <section
      aria-labelledby="memories-empty-heading"
      className="mt-6 flex flex-col items-center gap-2 text-center sm:flex-row sm:text-left"
    >
      <img
        src={memoriesIllustration}
        alt=""
        width={440}
        height={440}
        className="size-44 shrink-0 object-contain"
        draggable={false}
      />
      <div>
        <Typography variant="title" id="memories-empty-heading">
          {t('page.empty')}
        </Typography>
        <Typography variant="body" className="text-content-secondary max-w-measure mt-2">
          {t('page.emptyHelp')}
        </Typography>
        <Typography variant="body" className="text-content-primary mt-3">
          {t('page.example')}
        </Typography>
      </div>
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
      <li className="p-4">
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
    <li data-memory-row="" className="px-5 py-3">
      <div className="flex items-start gap-1">
        <Typography
          variant="body"
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
      <div className="flex flex-wrap items-center gap-2">
        {memory.tags.map((tag) => (
          <Badge key={tag}>#{tag}</Badge>
        ))}
        {memory.sourcePostSlugs.length > 0 && (
          <Typography variant="meta" as="span">
            {t('page.learnedFrom', { ns: 'memories', count: memory.sourcePostSlugs.length })}
          </Typography>
        )}
      </div>
    </li>
  )
}
