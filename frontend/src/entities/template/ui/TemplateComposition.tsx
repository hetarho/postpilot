import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { decode } from '../lib/grammar'
import {
  blockKindKey,
  blockSummary,
  canInsert,
  endPosition,
  fromBody,
  insertAt,
  positionAfter,
  reorder,
  toValidBody,
  type BuilderBlock,
  type PaletteKind,
  type Position,
} from '../model/blocks'
import { remainingChars, TEMPLATE_LIMITS } from '../model/types'
import {
  Badge,
  Button,
  FieldLabel,
  FieldMessage,
  SortableList,
  TextField,
  Textarea,
  Typography,
  typographyStyles,
} from '@/shared/ui'

/** The composition editor: the post's shape as a list of one-line rows, over the template body.
 *
 *  It lives in the ENTITY rather than in a feature because it performs no mutation — it is a
 *  controlled input over one of the template's own fields, and the shape of a block is the
 *  entity's business, not a page's (ARCHITECTURE §3.1).
 *
 *  THE BODY IS THE SINGLE SOURCE OF TRUTH: rows are parsed from it and serialized back on every
 *  edit, so there is no second representation to keep in step. The grammar string itself is never
 *  rendered anywhere (change 30 A9), and a body the parser cannot read is neither guessed at nor
 *  silently dropped — it says so, and offers to start over.
 *
 *  ONE ROW IS ONE BLOCK, collapsed, so the list reads as the outline of the post rather than as a
 *  stack of labelled inputs. At most one row is open at a time: the outline is the thing worth
 *  keeping legible while a single block is edited. */
export function TemplateComposition({
  value,
  onChange,
  disabled = false,
  className,
}: {
  value: string
  onChange: (body: string) => void
  disabled?: boolean
  className?: string
}) {
  return fromBody(value, decode).ok ? (
    <Composition value={value} onChange={onChange} disabled={disabled} className={className} />
  ) : (
    <Unreadable disabled={disabled} onClear={() => onChange('')} className={className} />
  )
}

/** A body from before the builder existed, or one edited outside the app. The composition cannot
 *  be shown, and inventing a structure the author did not write would be worse than saying so —
 *  so the only action is to start it over. It writes nothing: the screen's save is still the only
 *  write (change 30 A10). */
function Unreadable({
  disabled,
  onClear,
  className,
}: {
  disabled: boolean
  onClear: () => void
  className?: string
}) {
  const { t } = useTranslation('templates')
  return (
    <div className={className}>
      <FieldMessage role="alert">{t('composition.unreadable')}</FieldMessage>
      <Button variant="danger" disabled={disabled} onClick={onClear} className="mt-3">
        {t('composition.clearAndRestart')}
      </Button>
    </div>
  )
}

function Composition({
  value,
  onChange,
  disabled,
  className,
}: {
  value: string
  onChange: (body: string) => void
  disabled: boolean
  className?: string
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  // The rows are LOCAL state, seeded from the body. A block exists as a row the moment it is
  // added, before anything is typed into it — and an empty `<write></write>` does not parse, so
  // an incomplete row contributes no bytes at all. Without this the editor would emit a body that
  // no longer contains the block the user just asked for, and the next render would delete it.
  //
  // `emitted` closes the loop: a value that is not what this editor last produced came from
  // outside (a refetch, the unreadable state's clear), and only then are the rows reseeded.
  const [blocks, setBlocks] = useState<BuilderBlock[]>(() => readBlocks(value))
  const emitted = useRef(value)
  useEffect(() => {
    if (value === emitted.current) return
    emitted.current = value
    setBlocks(readBlocks(value))
  }, [value])
  // The two pieces of view state, both keyed by BLOCK ID rather than by index so an insertion or
  // a reorder above them cannot silently move either.
  //
  // `aimedAt` is the row last touched, and the toolbar's target is derived from it — which is
  // what makes ONE toolbar unambiguous: the aim is always drawn against a row that is on screen.
  const [aimedAt, setAimedAt] = useState<string | null>(null)
  const [openId, setOpenId] = useState<string | null>(null)
  // `aimedAt` can outlive its block — the row was deleted, or an outside value reseeded the rows.
  // `resolved` is the aim that still EXISTS, and everything keys off it: the marker and the target
  // must agree, or the toolbar would silently aim at the end while nothing on screen said so.
  const resolved = aimedAt === null ? null : positionAfter(blocks, aimedAt)
  const target: Position = resolved ?? endPosition(blocks)
  const aimIsAtEnd = resolved === null

  const push = (next: BuilderBlock[]) => {
    setBlocks(next)
    const body = toValidBody(next)
    emitted.current = body
    onChange(body)
  }

  const add = (kind: PaletteKind) => {
    const result = insertAt(blocks, target, kind)
    if (!result.inserted) return
    push(result.blocks)
    // The new block opens for typing and becomes the aim, so adding three in a row builds
    // downward instead of stacking them all at one point.
    setOpenId(result.inserted.id)
    setAimedAt(result.inserted.id)
  }

  const context: RowContext = {
    disabled,
    openId,
    aimedAt: aimIsAtEnd ? null : aimedAt,
    onOpen: (blockId) => {
      setOpenId((current) => (current === blockId ? null : blockId))
      // Touching a row aims the toolbar whether or not the row stays open.
      setAimedAt(blockId)
    },
    onTouch: setAimedAt,
  }

  return (
    <div className={className}>
      <AddToolbar id={`${id}-toolbar`} disabled={disabled} target={target} onAdd={add} />

      {/* A surface step rather than a border, and NO nested scroller: the page scrolls this
          (design-language §1.3, §4.4). */}
      <div className="bg-surface-recessed mt-2 rounded-lg px-2">
        {blocks.length === 0 ? (
          <Typography variant="body" className="text-content-tertiary block px-2 py-6 text-center">
            {t('composition.empty')}
          </Typography>
        ) : (
          <BlockList blocks={blocks} context={context} onChange={push} />
        )}
        {/* The aim past the last row has no row of its own to draw it. */}
        {aimIsAtEnd && blocks.length > 0 && <InsertionPoint />}
      </div>

      {/* The body is what the server bounds, so the count is over the body and not over the rows:
          an incomplete row contributes none of it yet. */}
      <BodyCount value={value} />
    </div>
  )
}

function readBlocks(body: string): BuilderBlock[] {
  const result = fromBody(body, decode)
  return result.ok ? result.blocks : []
}

/** What every row needs from the composition, passed as one value so a repeat's children get it
 *  unchanged one level down. */
interface RowContext {
  disabled: boolean
  openId: string | null
  aimedAt: string | null
  onOpen: (blockId: string) => void
  onTouch: (blockId: string) => void
}

/** One sibling group. A repeat's children are the same list one level down, which is what keeps
 *  a reorder scoped to siblings: the grammar has no way to express a block that left its repeat,
 *  so a drag must not be able to produce one. */
function BlockList({
  blocks,
  context,
  onChange,
}: {
  blocks: readonly BuilderBlock[]
  context: RowContext
  onChange: (next: BuilderBlock[]) => void
}) {
  const { t } = useTranslation('templates')
  return (
    <SortableList
      density="compact"
      items={blocks.map((block, index) => ({
        id: block.id,
        content: (
          <BlockRow
            block={block}
            context={context}
            onChange={(next) => onChange(blocks.map((b, i) => (i === index ? next : b)))}
            onRemove={() => onChange(blocks.filter((_, i) => i !== index))}
          />
        ),
      }))}
      onReorder={(from, to) => {
        // A reorder is a touch too: the aim follows the block the user just moved.
        context.onTouch(blocks[from].id)
        onChange(reorder(blocks, from, to))
      }}
      labels={{
        drag: t('builder.block.drag'),
        up: t('builder.block.moveUp'),
        down: t('builder.block.moveDown'),
      }}
    />
  )
}

/** The commands, always in reach above the composition. A wrapping row of buttons rather than a
 *  menu: these are COMMANDS, and a menu is a radio group over a current value — picking the option
 *  that already equals the value would emit nothing at all.
 *
 *  It sticks below the app header so it stays reachable in a long composition, which is the whole
 *  reason there is one toolbar instead of one per nesting level. */
function AddToolbar({
  id,
  disabled,
  target,
  onAdd,
}: {
  id: string
  disabled: boolean
  target: Position
  onAdd: (kind: PaletteKind) => void
}) {
  const { t } = useTranslation('templates')
  const kinds: PaletteKind[] = ['write', 'text', 'photo', 'place', 'link', 'repeat', 'note']
  return (
    <div className="bg-surface-base sm:top-header sticky top-0 z-10 -mx-4 px-4 py-2 sm:-mx-6 sm:px-6 lg:-mx-8 lg:px-8">
      <Typography variant="label" as="p" id={id}>
        {t('composition.add')}
      </Typography>
      <div role="group" aria-labelledby={id} className="mt-1 flex flex-wrap gap-2">
        {kinds
          // A repeat inside a repeat is refused by the grammar, so the button is not offered where
          // the aim is inside one. The model refuses it too — this is only the affordance.
          .filter((kind) => canInsert(kind, target))
          .map((kind) => (
            <Button
              key={kind}
              variant="secondary"
              size="compact"
              disabled={disabled}
              onClick={() => onAdd(kind)}
            >
              {t(`builder.palette.${kind}`)}
            </Button>
          ))}
      </div>
    </div>
  )
}

/** Where the next block lands, drawn in the list itself. Without it one toolbar would be worse
 *  than the two entry points it replaces: 추가 would put a block somewhere the user has to go and
 *  find afterwards (change 30 A7). */
function InsertionPoint() {
  const { t } = useTranslation('templates')
  return (
    <p role="status" className="flex min-h-8 items-center gap-2">
      {/* A hairline between two things is one of the four cases §1.3 allows a rule in. */}
      <span aria-hidden="true" className="bg-divider h-px flex-1" />
      <Badge tone="accent">{t('composition.insertHere')}</Badge>
      <span aria-hidden="true" className="bg-divider h-px flex-1" />
    </p>
  )
}

/** One block: a collapsed line that reads as part of the outline, expanding in place to its own
 *  fields. The whole line is the toggle, so the row is one target rather than a row with a small
 *  button in it (design-language §4.1); the delete lives in the expanded panel, because a
 *  destructive control on every collapsed row would take the width the summary needs at 360px. */
function BlockRow({
  block,
  context,
  onChange,
  onRemove,
}: {
  block: BuilderBlock
  context: RowContext
  onChange: (next: BuilderBlock) => void
  onRemove: () => void
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  const open = context.openId === block.id
  const summary = blockSummary(block)

  return (
    <div>
      <button
        type="button"
        aria-expanded={open}
        aria-controls={open ? `${id}-fields` : undefined}
        onClick={() => context.onOpen(block.id)}
        className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 w-full min-w-0 items-center gap-2 rounded-md px-1 text-left"
      >
        <Badge tone={block.kind === 'repeat' ? 'accent' : 'neutral'}>
          {t(`builder.palette.${blockKindKey(block)}`)}
        </Badge>
        <span
          className={typographyStyles({
            variant: 'body',
            className: summary
              ? 'text-content-primary min-w-0 flex-1 truncate'
              : 'text-content-tertiary min-w-0 flex-1 truncate',
          })}
        >
          {summary || t(`composition.placeholder.${blockKindKey(block)}`)}
        </span>
      </button>

      {open && (
        <div id={`${id}-fields`} className="pt-1 pb-3">
          <BlockFields block={block} disabled={context.disabled} onChange={onChange} />
          <Button
            variant="danger"
            size="compact"
            disabled={context.disabled}
            onClick={onRemove}
            className="mt-3"
          >
            <Trash2 aria-hidden="true" className="size-4" />
            {t('builder.block.remove')}
          </Button>
        </div>
      )}

      {/* A repeat's children are always visible: they ARE its content, and hiding them behind the
          same toggle would take the nested half of the post's shape out of the outline. */}
      {block.kind === 'repeat' && (
        <div className="pb-2 pl-4">
          {block.children.length === 0 ? (
            <Typography variant="meta" as="p" className="py-2">
              {t('composition.repeatEmpty')}
            </Typography>
          ) : (
            <BlockList
              blocks={block.children}
              context={context}
              onChange={(children) => onChange({ ...block, children })}
            />
          )}
          {/* The aim inside this repeat, past its last child. */}
          {context.aimedAt === block.id && <InsertionPoint />}
        </div>
      )}

      {/* The aim after this row. A repeat draws its own aim inside itself instead, because that
          is where `positionAfter` puts it. */}
      {context.aimedAt === block.id && block.kind !== 'repeat' && <InsertionPoint />}
    </div>
  )
}

/** One block's own fields. A slot has no instruction to give a model — only a label for the person
 *  who fills it — which is exactly the distinction the grammar draws. A repeat has no fields at
 *  all: its content is its children, which are rows of their own. */
function BlockFields({
  block,
  disabled,
  onChange,
}: {
  block: BuilderBlock
  disabled: boolean
  onChange: (next: BuilderBlock) => void
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  switch (block.kind) {
    case 'write':
      return (
        <Field
          id={id}
          label={t('builder.block.instruction')}
          value={block.text}
          disabled={disabled}
          onChange={(text) => onChange({ ...block, text })}
        />
      )
    case 'note':
      return (
        <Field
          id={id}
          label={t('builder.block.note')}
          value={block.text}
          disabled={disabled}
          onChange={(text) => onChange({ ...block, text })}
        />
      )
    case 'text':
      return (
        <Field
          id={id}
          label={t('builder.block.text')}
          value={block.text}
          disabled={disabled}
          multiline
          onChange={(text) => onChange({ ...block, text })}
        />
      )
    case 'slot':
      return block.slotKind === 'photo' ? (
        <Typography variant="meta" as="p">
          {t('builder.palette.photoHelp')}
        </Typography>
      ) : (
        <Field
          id={id}
          label={t('builder.block.label')}
          value={block.label}
          disabled={disabled}
          onChange={(label) => onChange({ ...block, label })}
        />
      )
    case 'repeat':
      return (
        <Typography variant="meta" as="p">
          {t('composition.repeatHelp')}
        </Typography>
      )
  }
}

function Field({
  id,
  label,
  value,
  disabled,
  multiline = false,
  onChange,
}: {
  id: string
  label: string
  value: string
  disabled: boolean
  multiline?: boolean
  onChange: (value: string) => void
}) {
  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      {multiline ? (
        <Textarea
          id={id}
          value={value}
          disabled={disabled}
          rows={2}
          autoGrow
          onChange={(event) => onChange(event.target.value)}
          className="mt-1"
        />
      ) : (
        <TextField
          id={id}
          value={value}
          disabled={disabled}
          autoComplete="off"
          onChange={(event) => onChange(event.target.value)}
          className="mt-1"
        />
      )}
    </div>
  )
}

function BodyCount({ value }: { value: string }) {
  const { t } = useTranslation('common')
  const left = remainingChars(value, TEMPLATE_LIMITS.body)
  return left < 0 ? (
    <FieldMessage role="status" className="mt-2">
      {t('count.exceeded', { count: -left })}
    </FieldMessage>
  ) : (
    <Typography variant="meta" as="p" className="mt-2">
      {t('count.remaining', { count: left })}
    </Typography>
  )
}
