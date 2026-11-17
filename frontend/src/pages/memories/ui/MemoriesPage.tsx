import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Infinity as Always, Pencil, TextSearch } from 'lucide-react'
import { useMemories, useUpdateMemoryCall, type Memory, type MemoryKind } from '@/entities/memory'
import { useSession } from '@/entities/session'
import { CreateMemorySheet } from '@/features/create-memory'
import { DeleteMemoryButton } from '@/features/delete-memory'
import { MemoryEditForm } from '@/features/edit-memory'
import { KitScreenHeader } from '@/widgets/writing-kit'
import { memoryIllustration } from '@/shared/assets'
import {
  Badge,
  Button,
  Notice,
  LoadingState,
  Typography,
  cardStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

/** The two halves retrieval treats the store in (MEM-6): a standing fact about the author is a
 *  candidate for every post; a place, a person or a piece of history only when the post shares
 *  one of its tags. The screen is split the same way, because that IS what each memory does to
 *  the writing. */
type Zone = 'always' | 'keyword'
const ZONE_KINDS: Record<Zone, readonly MemoryKind[]> = {
  always: ['preference', 'persona'],
  keyword: ['place', 'person', 'history'],
}
const zoneOf = (kind: MemoryKind): Zone => (ZONE_KINDS.always.includes(kind) ? 'always' : 'keyword')

/** Guide: 기억 — what the AI knows about the owner, split by WHEN it reaches a post. The top half
 *  is always in mind; the bottom half waits for a word, so each of its memories leads with the
 *  words that recall it — and says so when it has none and will never be recalled. Each half has
 *  its own add, which opens the shared sheet already on that half's kind. Every memory shows
 *  where it came from: a post it was saved from, or the owner's own hand.
 *
 *  Nothing on this screen calls a model or enqueues a job: a memory is an authored fact, and
 *  reading, editing or deleting one is a plain CRUD round trip ([I5], MEM-23). Within a half the
 *  rows keep the server's order — most recently used first, the order a post is given them in. */
export function MemoriesPage() {
  const { t } = useTranslation(['memories', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { memories, isPending, isError, isFetching, refetch } = useMemories(ownerId)
  const page = useRef<HTMLElement>(null)
  // A confirmed delete hands focus back to the trash button, then the re-read list removes the row
  // holding it, which drops focus to <body>. The move waits until the row has left the list, then
  // lands on the next row's delete in the same half, or that half's add.
  const [removed, setRemoved] = useState<Removal | null>(null)
  const settled = useRef<Removal | null>(null)
  useEffect(() => {
    if (!removed || settled.current === removed) return
    if (memories.some((memory) => memory.id === removed.id)) return
    settled.current = removed
    const active = document.activeElement
    if (active && active !== document.body) return
    const zone = page.current?.querySelector(`[data-zone="${removed.zone}"]`)
    const rows = zone?.querySelectorAll('li') ?? []
    const target =
      rows.length > 0
        ? buttonNamed(rows[Math.min(removed.index, rows.length - 1)], t('delete.aria'))
        : zone?.querySelector<HTMLButtonElement>('header button')
    target?.focus()
  }, [removed, memories, t])

  return (
    <main
      ref={page}
      className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col pt-4 sm:pt-8' })}
    >
      <KitScreenHeader
        kit="memory"
        title={t('title')}
        statement={t('page.statement')}
        illustration={memoryIllustration}
      />

      {isError && (
        <Notice tone="danger" role="alert" className="mt-8">
          <span>{t('loadFailed')}</span>
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
        <div className="mt-8 grid gap-10 lg:grid-cols-2 lg:gap-8">
          {(['always', 'keyword'] as const).map((zone) => {
            const items = memories.filter((memory) => zoneOf(memory.kind) === zone)
            return (
              <ZoneSection
                key={zone}
                zone={zone}
                ownerId={ownerId}
                items={items}
                onDeleted={(id, index) => setRemoved({ id, index, zone })}
              />
            )
          })}
        </div>
      )}
    </main>
  )
}

/** A deleted memory, the half it was in and the position its row held. */
type Removal = { id: string; index: number; zone: Zone }

function buttonNamed(root: Element | null | undefined, name: string) {
  return Array.from(root?.querySelectorAll('button') ?? []).find(
    (button) => (button.getAttribute('aria-label') ?? button.textContent?.trim()) === name,
  )
}

function ZoneSection({
  zone,
  ownerId,
  items,
  onDeleted,
}: {
  zone: Zone
  ownerId: string
  items: readonly Memory[]
  onDeleted: (id: string, index: number) => void
}) {
  const { t } = useTranslation('memories')
  const Icon = zone === 'always' ? Always : TextSearch
  const headingId = `memories-${zone}`
  return (
    <section aria-labelledby={headingId} data-zone={zone} className="flex min-w-0 flex-col gap-3">
      <header className="flex flex-wrap items-start gap-3">
        <span
          aria-hidden="true"
          className="bg-badge-accent-bg text-badge-accent-fg flex size-11 shrink-0 items-center justify-center rounded-xl"
        >
          <Icon className="size-5" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col">
          <Typography variant="title" as="h2" id={headingId} className="flex items-baseline gap-2">
            {t(`page.${zone}.title`)}
            <span className={typographyStyles({ variant: 'label' })}>{items.length}</span>
          </Typography>
          <Typography variant="meta" as="p">
            {t(`page.${zone}.how`)}
          </Typography>
        </span>
        <CreateMemorySheet
          ownerId={ownerId}
          initialKind={ZONE_KINDS[zone][0]}
          label={t(`page.${zone}.add`)}
          variant="secondary"
          className="w-full sm:w-auto"
        />
      </header>
      {items.length === 0 ? (
        <Typography
          variant="body"
          as="p"
          className="text-content-tertiary border-divider rounded-xl border border-dashed px-4 py-6 text-center"
        >
          {t('page.none')}
        </Typography>
      ) : (
        <ul className="grid gap-3">
          {items.map((memory, index) => (
            <MemoryRow
              key={memory.id}
              ownerId={ownerId}
              memory={memory}
              zone={zone}
              onDeleted={() => onDeleted(memory.id, index)}
            />
          ))}
        </ul>
      )}
    </section>
  )
}

/** One saved memory (MEM-30). In the word half its tags lead, as the words that recall it; in
 *  the always half they are the quiet line under the text they are for. 수정 turns the row into
 *  one form for the text, the kind and the tags; 삭제 confirms. */
function MemoryRow({
  ownerId,
  memory,
  zone,
  onDeleted,
}: {
  ownerId: string
  memory: Memory
  zone: Zone
  onDeleted: () => void
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
      <li className={cardStyles({ padding: 'sm' })}>
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

  const origin =
    memory.sourcePostSlugs.length > 0
      ? t('page.fromPosts', { count: memory.sourcePostSlugs.length })
      : t('page.byHand')
  return (
    <li className={cardStyles({ padding: 'sm', className: 'flex flex-col gap-2' })}>
      {zone === 'keyword' &&
        (memory.tags.length > 0 ? (
          <p className="flex flex-wrap items-center gap-1.5">
            <span className={typographyStyles({ variant: 'meta' })}>{t('page.trigger')}</span>
            {memory.tags.map((tag) => (
              <Badge key={tag} tone="accent">
                {tag}
              </Badge>
            ))}
          </p>
        ) : (
          <Typography variant="meta" as="p" className="text-notice-warning-fg">
            {t('page.noTrigger')}
          </Typography>
        ))}
      <div className="flex items-start gap-1">
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
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <Badge tone="neutral">{t(`kind.${memory.kind}`, { ns: 'memories' })}</Badge>
        {zone === 'always' &&
          memory.tags.map((tag) => (
            <span key={tag} className={typographyStyles({ variant: 'meta' })}>
              #{tag}
            </span>
          ))}
        <span className={typographyStyles({ variant: 'meta', className: 'ml-auto' })}>
          {origin}
        </span>
      </p>
    </li>
  )
}
