import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigate, useSearch } from '@tanstack/react-router'
import { Pencil, Search } from 'lucide-react'
import { useMemories, useUpdateMemoryCall, type Memory, type MemoryKind } from '@/entities/memory'
import { useSession } from '@/entities/session'
import { CreateMemorySheet } from '@/features/create-memory'
import { DeleteMemoryButton } from '@/features/delete-memory'
import { MemoryEditForm } from '@/features/edit-memory'
import { KIT_ILLUSTRATIONS } from '@/shared/assets'
import {
  ActionBar,
  Badge,
  Button,
  Notice,
  LoadingState,
  TextField,
  Typography,
  EmptySlot,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** How a fact reaches a post (MEM-6): 취향 and 설정 are recalled for every post; 장소, 인물 and 이력
 *  only when the post mentions one of their tags. The page is drawn in these two bands, because
 *  that is what a memory DOES to the writing. */
const BANDS: readonly { key: 'always' | 'tagged'; kinds: readonly MemoryKind[] }[] = [
  { key: 'always', kinds: ['preference', 'persona'] },
  { key: 'tagged', kinds: ['place', 'person', 'history'] },
]

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
  const list = useRef<HTMLElement>(null)
  // A confirmed delete hands focus back to the trash button, then the re-read list removes the row
  // holding it, which drops focus to <body> and sends a keyboard user to the top of the page. The
  // delete does not await that re-read, so the move waits until the row has left the list.
  const [removed, setRemoved] = useState<Removal | null>(null)
  const settled = useRef<Removal | null>(null)
  const [query, setQuery] = useState('')
  // `?new=1` — home's 기억 layer while it is empty — opens the new-memory sheet once and is dropped.
  const search = useSearch({ strict: false }) as { new?: unknown }
  const requested = search.new === 1 || search.new === '1'
  const [openOnArrival] = useState(requested)
  const navigate = useNavigate()
  useEffect(() => {
    if (requested) void navigate({ to: '/memories', search: {}, replace: true })
  }, [requested, navigate])
  const needle = query.trim().toLocaleLowerCase()
  const matches = (memory: Memory) =>
    needle === '' ||
    memory.text.toLocaleLowerCase().includes(needle) ||
    memory.tags.some((tag) => tag.toLocaleLowerCase().includes(needle))
  const shown = memories.filter(matches)
  // The order the rows are drawn in: band by band, kind by kind, the server's order inside each.
  const drawn = BANDS.flatMap((band) =>
    band.kinds.flatMap((kind) => memories.filter((memory) => memory.kind === kind)),
  )

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
            list.current?.querySelectorAll('[data-memory-row]')[
              Math.min(removed.index, memories.length - 1)
            ],
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
            <>
              {/* Finding one fact among many: the text and the tags, as typed. */}
              <label className="relative mt-6 block max-w-md">
                <span className="sr-only">{t('page.search', { ns: 'memories' })}</span>
                <Search
                  aria-hidden="true"
                  className="text-content-tertiary pointer-events-none absolute top-1/2 left-3 size-5 -translate-y-1/2"
                />
                <TextField
                  type="search"
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder={t('page.search', { ns: 'memories' })}
                  className="pl-10"
                />
              </label>
              {needle !== '' && shown.length === 0 && (
                <Typography variant="body" role="status" className="text-content-tertiary mt-6">
                  {t('page.noMatch', { ns: 'memories', q: query.trim() })}
                </Typography>
              )}
              <section
                ref={list}
                aria-label={t('page.saved', { ns: 'memories' })}
                className="mt-6 grid gap-10"
              >
                {BANDS.map((band) =>
                  // While searching, a band with no match is left out whole, not kept as a bare
                  // heading over nothing.
                  needle !== '' &&
                  !shown.some((memory) => band.kinds.includes(memory.kind)) ? null : (
                    <div key={band.key}>
                      <section aria-labelledby={`memories-band-${band.key}`}>
                        <Typography variant="title" as="h2" id={`memories-band-${band.key}`}>
                          {t(`page.bands.${band.key}.title`, { ns: 'memories' })}
                        </Typography>
                        <Typography variant="body" className="text-content-secondary mt-1">
                          {t(`page.bands.${band.key}.lead`, { ns: 'memories' })}
                        </Typography>
                        <div className="mt-4 grid gap-6 lg:grid-cols-2 lg:items-start">
                          {band.kinds.map((kind) => {
                            const group = shown.filter((memory) => memory.kind === kind)
                            if (needle !== '' && group.length === 0) return null
                            return (
                              <div key={kind} className="min-w-0">
                                <div className="flex min-h-11 flex-wrap items-center gap-2">
                                  <Badge tone="accent">
                                    {t(`kind.${kind}`, { ns: 'memories' })}
                                  </Badge>
                                  <span className={typographyStyles({ variant: 'meta' })}>
                                    {group.length > 0
                                      ? group.length
                                      : t('page.kindEmpty', { ns: 'memories' })}
                                  </span>
                                  <CreateMemorySheet
                                    ownerId={ownerId}
                                    trigger="inline"
                                    initialKind={kind}
                                    label={t('page.addKind', {
                                      ns: 'memories',
                                      kind: t(`kind.${kind}`, { ns: 'memories' }),
                                    })}
                                    className="ml-auto"
                                  />
                                </div>
                                {group.length > 0 && (
                                  // One sheet per kind, its facts ruled off from each other: the
                                  // group reads as one page of what the AI knows, not a pile of cards.
                                  <ul
                                    className={cardStyles({
                                      padding: 'none',
                                      className: 'divide-divider mt-2 divide-y overflow-hidden',
                                    })}
                                  >
                                    {group.map((memory) => (
                                      <MemoryRow
                                        key={memory.id}
                                        ownerId={ownerId}
                                        memory={memory}
                                        tagged={band.key === 'tagged'}
                                        onDeleted={() =>
                                          setRemoved({
                                            id: memory.id,
                                            index: drawn.indexOf(memory),
                                          })
                                        }
                                      />
                                    ))}
                                  </ul>
                                )}
                              </div>
                            )
                          })}
                        </div>
                      </section>
                    </div>
                  ),
                )}
              </section>
            </>
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
        <CreateMemorySheet
          ownerId={ownerId}
          defaultOpen={openOnArrival}
          className="w-full sm:w-auto"
        />
      </ActionBar>
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
    // The dashed place the first fact will stand, its drawing faded, with one example of a fact.
    <section aria-labelledby="memories-empty-heading" className="mt-8">
      <EmptySlot
        image={KIT_ILLUSTRATIONS.memory}
        title={t('page.empty')}
        titleAs="h2"
        titleId="memories-empty-heading"
      >
        <p>{t('page.emptyHelp')}</p>
        <p className="text-content-primary mt-2">{t('page.example')}</p>
      </EmptySlot>
    </section>
  )
}

/** One saved memory (MEM-30): its text with 수정 and 삭제 at the end of the text line, over one
 *  line of its kind and tags. The list is read to see what the account knows, so a fact keeps to
 *  one short row; 수정 turns the whole row into one form for the text, the kind and the tags. */
function MemoryRow({
  ownerId,
  memory,
  tagged,
  onDeleted,
}: {
  ownerId: string
  memory: Memory
  /** A fact recalled only on a tag match: its tags are the words that bring it in. */
  tagged: boolean
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
      <li data-memory-row className="px-4 py-3 sm:px-5">
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
    <li data-memory-row className="px-4 py-3 sm:px-5">
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
      {(tagged || memory.tags.length > 0) && (
        <div className="mt-1 flex flex-wrap items-center gap-2">
          {/* A fact recalled on a tag says the words that bring it in — or, with none, that no
              post brings it in yet, which is what the missing tags mean for the writing. */}
          {tagged && memory.tags.length > 0 && (
            <Typography variant="meta" as="span">
              {t('page.trigger', { ns: 'memories' })}
            </Typography>
          )}
          {memory.tags.map((tag) => (
            <Badge key={tag} tone={tagged ? 'info' : 'neutral'}>
              #{tag}
            </Badge>
          ))}
          {memory.tags.length === 0 && (
            <Typography variant="meta" as="span">
              {t('page.untriggered', { ns: 'memories' })}
            </Typography>
          )}
        </div>
      )}
    </li>
  )
}
