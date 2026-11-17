import { useEffect, useRef, useState, type ComponentType } from 'react'
import { useTranslation } from 'react-i18next'
import { History, Heart, MapPin, Pencil, Plus, UserRound, Users } from 'lucide-react'
import {
  MEMORY_KINDS,
  useMemories,
  useUpdateMemoryCall,
  type Memory,
  type MemoryKind,
} from '@/entities/memory'
import { useSession } from '@/entities/session'
import { CreateMemorySheet } from '@/features/create-memory'
import { DeleteMemoryButton } from '@/features/delete-memory'
import { MemoryEditForm } from '@/features/edit-memory'
import { KitArt, KitEmpty, KitHeader } from '@/widgets/writing-kit'
import {
  ActionBar,
  Button,
  Notice,
  LoadingState,
  Typography,
  ChipToggle,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

const KIND_ICONS: Record<MemoryKind, ComponentType<{ className?: string }>> = {
  preference: Heart,
  persona: UserRound,
  place: MapPin,
  person: Users,
  history: History,
}

/** Guide: 기억 — what the AI knows about the owner, kept as a notebook with one page per kind of
 *  fact (취향 · 설정 · 장소 · 인물 · 이력). The written pages come first; the kinds with nothing yet
 *  wait together as blank pages, each saying what it would hold, and its 추가 starts a fact of
 *  THAT kind. The tags the facts carry are the way
 *  to find one: a tag narrows the notebook to the facts that carry it. A fact is changed or removed
 *  in place. Within a kind, facts keep the server's order — the order a post is given them.
 *
 *  Nothing on this screen calls a model or enqueues a job ([I5], MEM-23). */
export function MemoriesPage() {
  const { t } = useTranslation(['memories', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { memories, isPending, isError, isFetching, refetch } = useMemories(ownerId)
  const page = useRef<HTMLElement>(null)
  const [tag, setTag] = useState<string>()
  const [adding, setAdding] = useState<MemoryKind>()
  // A confirmed delete removes the row holding the focused trash button; focus moves to the
  // trash button now in its place, or to 새 기억 when the notebook is empty (MEM-24).
  const [removed, setRemoved] = useState<Removal | null>(null)
  const settled = useRef<Removal | null>(null)
  const tags = tagsByUse(memories)
  const activeTag = tag && tags.includes(tag) ? tag : undefined
  const shown = activeTag ? memories.filter((memory) => memory.tags.includes(activeTag)) : memories
  const ordered = MEMORY_KINDS.flatMap((kind) => shown.filter((memory) => memory.kind === kind))
  // Pages with facts are the notebook; the kinds with none wait together as blank pages, each
  // still a way to start a fact of that kind. A tag narrows only the written pages.
  const filled = MEMORY_KINDS.filter((kind) => shown.some((memory) => memory.kind === kind))
  const blank = activeTag
    ? []
    : MEMORY_KINDS.filter((kind) => !memories.some((memory) => memory.kind === kind))

  useEffect(() => {
    if (!removed || settled.current === removed) return
    if (memories.some((memory) => memory.id === removed.id)) return
    settled.current = removed
    const active = document.activeElement
    if (active && active !== document.body) return
    const trash = Array.from(page.current?.querySelectorAll('button') ?? []).filter(
      (button) => button.getAttribute('aria-label') === t('delete.aria', { ns: 'memories' }),
    )
    const target =
      trash[Math.min(removed.index, trash.length - 1)] ??
      Array.from(page.current?.querySelectorAll('button') ?? []).find(
        (button) => button.textContent?.trim() === t('create.open', { ns: 'memories' }),
      )
    target?.focus()
  }, [removed, memories, t])

  return (
    <main
      ref={page}
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-2 sm:pt-4' })}
    >
      <KitHeader part="memory" />

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
          variant="list"
          label={t('state.loading', { ns: 'common' })}
          className="mt-8"
        />
      )}

      {!isError && !isPending && (
        <>
          {tags.length > 0 && (
            <div
              role="group"
              aria-label={t('notebook.tags', { ns: 'memories' })}
              className="-mx-4 mt-6 flex gap-2 overflow-x-auto overscroll-x-contain px-4 pb-1 sm:mx-0 sm:flex-wrap sm:px-0"
            >
              <ChipToggle pressed={!activeTag} onClick={() => setTag(undefined)}>
                {t('notebook.all', { ns: 'memories', count: memories.length })}
              </ChipToggle>
              {tags.map((name) => (
                <ChipToggle
                  key={name}
                  pressed={activeTag === name}
                  onClick={() => setTag(activeTag === name ? undefined : name)}
                >
                  #{name}
                </ChipToggle>
              ))}
            </div>
          )}
          {/* On the desk the written pages and the blank ones stand side by side; with no blank
              page left, the written ones take both columns. */}
          <div className="mt-6 grid items-start gap-4 lg:grid-cols-2">
            {filled.length > 0 && (
              <section
                aria-label={t('page.saved', { ns: 'memories' })}
                className={
                  blank.length > 0
                    ? 'flex min-w-0 flex-col gap-4'
                    : 'gap-4 lg:col-span-2 lg:columns-2 [&>*]:mb-4 [&>*]:break-inside-avoid'
                }
              >
                {filled.map((kind) => (
                  <KindPage
                    key={kind}
                    kind={kind}
                    facts={shown.filter((memory) => memory.kind === kind)}
                    ownerId={ownerId}
                    onAdd={() => setAdding(kind)}
                    onDeleted={(memory) =>
                      setRemoved({ id: memory.id, index: ordered.indexOf(memory) })
                    }
                    onTag={setTag}
                  />
                ))}
              </section>
            )}
            {memories.length === 0 && !activeTag && (
              <KitEmpty
                className="lg:col-span-2"
                figure={<KitArt part="memory" />}
                title={t('notebook.firstTitle', { ns: 'memories' })}
              >
                {t('notebook.firstLine', { ns: 'memories' })}
              </KitEmpty>
            )}
            {blank.length > 0 && (
              <BlankPages
                kinds={blank}
                first={memories.length === 0}
                onAdd={setAdding}
                className={filled.length > 0 ? 'mt-2 lg:mt-0' : 'lg:col-span-2'}
              />
            )}
          </div>
        </>
      )}
      <CreateMemorySheet
        ownerId={ownerId}
        open={adding !== undefined}
        onOpenChange={(open) => {
          if (!open) setAdding(undefined)
        }}
        initialKind={adding}
      />
      {/* The one creation, the shape every sibling section uses (THEME-67). */}
      <ActionBar
        dock="list"
        ariaLabel={t('create.dockAria', { ns: 'memories' })}
        className="mt-auto w-full sm:w-fit"
      >
        <CreateMemorySheet ownerId={ownerId} className="w-full sm:w-auto" />
      </ActionBar>
    </main>
  )
}

/** A deleted memory and the position its row held, counted across the notebook. */
type Removal = { id: string; index: number }

/** Every tag the facts carry, most used first. */
function tagsByUse(memories: readonly Memory[]): string[] {
  const counts = new Map<string, number>()
  for (const memory of memories)
    for (const tag of memory.tags) counts.set(tag, (counts.get(tag) ?? 0) + 1)
  return [...counts.entries()].sort((a, b) => b[1] - a[1]).map(([tag]) => tag)
}

/** One kind's page of the notebook: its facts as sentences, and the 추가 that starts one more. */
function KindPage({
  kind,
  facts,
  ownerId,
  onAdd,
  onDeleted,
  onTag,
}: {
  kind: MemoryKind
  facts: Memory[]
  ownerId: string
  onAdd: () => void
  onDeleted: (memory: Memory) => void
  onTag: (tag: string) => void
}) {
  const { t } = useTranslation(['memories', 'common'])
  const Icon = KIND_ICONS[kind]
  return (
    <section
      aria-label={`${t(`kind.${kind}`, { ns: 'memories' })} ${facts.length}`}
      className="bg-card-bg flex flex-col rounded-2xl px-4 pt-3 pb-2 shadow-sm sm:px-5"
    >
      <div className="flex min-h-11 items-center gap-3">
        <span
          aria-hidden="true"
          className="bg-badge-accent-bg text-badge-accent-fg flex size-9 shrink-0 items-center justify-center rounded-full"
        >
          <Icon className="size-4.5" />
        </span>
        <h2 className="flex min-w-0 flex-1 items-baseline gap-2">
          <span className={typographyStyles({ variant: 'fieldTitle' })}>
            {t(`kind.${kind}`, { ns: 'memories' })}
          </span>
          <span className={typographyStyles({ variant: 'label' })}>{facts.length}</span>
        </h2>
        <Button
          variant="ghost"
          onClick={onAdd}
          aria-label={t('notebook.addKind', {
            ns: 'memories',
            kind: t(`kind.${kind}`, { ns: 'memories' }),
          })}
        >
          <Plus aria-hidden="true" className="size-4" />
          {t('notebook.add', { ns: 'memories' })}
        </Button>
      </div>
      <ul className="divide-divider divide-y">
        {facts.map((memory) => (
          <MemoryRow
            key={memory.id}
            ownerId={ownerId}
            memory={memory}
            onDeleted={() => onDeleted(memory)}
            onTag={onTag}
          />
        ))}
      </ul>
    </section>
  )
}

/** The kinds with no fact yet, as one sheet of blank pages: each its glyph, its name, what it
 *  would hold, and the 추가 that starts a fact of that kind. */
function BlankPages({
  kinds,
  first,
  onAdd,
  className,
}: {
  kinds: readonly MemoryKind[]
  first: boolean
  onAdd: (kind: MemoryKind) => void
  className?: string
}) {
  const { t } = useTranslation('memories')
  return (
    <section aria-labelledby="memories-blank" className={className}>
      <h2
        id="memories-blank"
        className={typographyStyles({ variant: 'groupLabel', className: 'mb-2 px-1' })}
      >
        {t(first ? 'notebook.blankFirst' : 'notebook.blank')}
      </h2>
      <ul className="divide-divider bg-card-bg divide-y overflow-hidden rounded-2xl shadow-sm">
        {kinds.map((kind) => {
          const Icon = KIND_ICONS[kind]
          const name = t(`kind.${kind}`)
          return (
            <li key={kind} className="flex min-h-16 items-center gap-3 px-4 py-2 sm:px-5">
              <span
                aria-hidden="true"
                className="border-content-tertiary/40 text-content-tertiary flex size-9 shrink-0 items-center justify-center rounded-full border-2 border-dashed"
              >
                <Icon className="size-4" />
              </span>
              <span className="flex min-w-0 flex-1 flex-col">
                <span className={typographyStyles({ variant: 'fieldTitle' })}>{name}</span>
                <span className={typographyStyles({ variant: 'meta' })}>
                  {t(`notebook.empty.${kind}`)}
                </span>
              </span>
              <Button
                variant="ghost"
                onClick={() => onAdd(kind)}
                aria-label={t('notebook.addKind', { kind: name })}
                className="shrink-0"
              >
                <Plus aria-hidden="true" className="size-4" />
                {t('notebook.add')}
              </Button>
            </li>
          )
        })}
      </ul>
    </section>
  )
}

/** One saved fact (MEM-30): its sentence with 수정 and 삭제 at the end of the line, its tags under
 *  it — each a way to narrow the notebook to that tag. 수정 turns the row into one form. */
function MemoryRow({
  ownerId,
  memory,
  onDeleted,
  onTag,
}: {
  ownerId: string
  memory: Memory
  onDeleted: () => void
  onTag: (tag: string) => void
}) {
  const { t } = useTranslation(['memories', 'common'])
  const update = useUpdateMemoryCall(ownerId, memory.id)
  const [form, setForm] = useState({ editing: false, refocus: false })
  const pencil = useRef<HTMLButtonElement>(null)

  useEffect(() => {
    const active = document.activeElement
    if (!form.refocus || (active && active !== document.body)) return
    pencil.current?.focus({ preventScroll: true })
  }, [form])

  if (form.editing)
    return (
      <li className="py-3">
        <MemoryEditForm
          memory={memory}
          save={update.save}
          errorMessage={update.errorMessage}
          pending={update.isPending}
          onDone={() =>
            setForm((current) => (current.editing ? { editing: false, refocus: true } : current))
          }
        />
      </li>
    )

  return (
    <li className="py-2 sm:pl-12">
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
      {memory.tags.length > 0 && (
        <div className="-mt-1 flex flex-wrap gap-x-2">
          {memory.tags.map((tag) => (
            <Button
              key={tag}
              variant="ghost"
              size="compact"
              onClick={() => onTag(tag)}
              className="-ml-2 px-2"
            >
              #{tag}
            </Button>
          ))}
        </div>
      )}
    </li>
  )
}
