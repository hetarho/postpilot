import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { Trash2 } from 'lucide-react'
import { decode } from '../lib/grammar'
import {
  asksForData,
  askTitle,
  blockKindKey,
  blockSummary,
  canInsert,
  duplicateAskTitles,
  endPosition,
  fromBody,
  insertAt,
  positionAfter,
  photoSummaryKey,
  repeatPhotoCount,
  reorder,
  toValidBody,
  type AskableBlock,
  type BodyRead,
  type BuilderBlock,
  type PaletteKind,
  type Position,
} from '../model/blocks'
import { remainingChars, TEMPLATE_LIMITS, TEMPLATE_PARSE_OPTIONS } from '../model/types'
import { TEMPLATE_PHOTO_ROW_MAX } from '@/shared/config'
import {
  Badge,
  Button,
  FieldCount,
  FieldLabel,
  FieldMessage,
  SortableList,
  Stepper,
  Switch,
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
  onFixInSource,
  onAskConflict,
}: {
  value: string
  onChange: (body: string) => void
  disabled?: boolean
  className?: string
  /** Switches the screen to 원문 with the caret in the text. Absent when there is no source mode
   *  to switch to, which is why the clear action can never be the only way out. */
  onFixInSource?: () => void
  /** Two rows are asking under one title. Such a row is left OUT of the emitted body — the
   *  parser refuses a body with a repeated title, and the builder must never emit one it cannot
   *  read back — so the caller has to refuse 저장 or the row would be silently dropped from a
   *  saved template (TEMPLATE-44). */
  onAskConflict?: (conflicted: boolean) => void
}) {
  const { t } = useTranslation('templates')
  return readBody(value, t).ok ? (
    <Composition
      value={value}
      onChange={onChange}
      disabled={disabled}
      className={className}
      onAskConflict={onAskConflict}
    />
  ) : (
    <Unreadable
      disabled={disabled}
      onClear={() => onChange('')}
      onFixInSource={onFixInSource}
      className={className}
    />
  )
}

/** A body from before the builder existed, one written by an outside AI, or one edited elsewhere.
 *  The composition cannot be shown, and inventing a structure the author did not write would be
 *  worse than saying so — so it says so and offers TWO ways out (TEMPLATE-30): fix the text where
 *  the parse error actually is, or empty the composition and start again.
 *
 *  The order is deliberate. Fixing keeps what the author wrote and is what a pasted body usually
 *  needs; clearing throws it away, so it is the second, destructive one. Neither writes anything:
 *  the screen's 저장 is still the only write. */
function Unreadable({
  disabled,
  onClear,
  onFixInSource,
  className,
}: {
  disabled: boolean
  onClear: () => void
  onFixInSource?: () => void
  className?: string
}) {
  const { t } = useTranslation('templates')
  return (
    <div className={className}>
      <FieldMessage role="alert">{t('composition.unreadable')}</FieldMessage>
      <div className="mt-3 flex flex-wrap gap-2">
        {onFixInSource && (
          <Button variant="secondary" disabled={disabled} onClick={onFixInSource}>
            {t('composition.fixInSource')}
          </Button>
        )}
        <Button variant="danger" disabled={disabled} onClick={onClear}>
          {t('composition.clearAndRestart')}
        </Button>
      </div>
    </div>
  )
}

function Composition({
  value,
  onChange,
  disabled,
  className,
  onAskConflict,
}: {
  value: string
  onChange: (body: string) => void
  disabled: boolean
  className?: string
  onAskConflict?: (conflicted: boolean) => void
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
  const [blocks, setBlocks] = useState<BuilderBlock[]>(() => readBlocks(value, t))
  const emitted = useRef(value)
  useEffect(() => {
    if (value === emitted.current) return
    emitted.current = value
    setBlocks(readBlocks(value, t))
  }, [value, t])
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

  const duplicates = duplicateAskTitles(blocks)
  // Reported upward rather than derived by the caller: the colliding row is not in the body it
  // can see, so there would be nothing left there to notice.
  const conflicted = duplicates.size > 0
  useEffect(() => onAskConflict?.(conflicted), [conflicted, onAskConflict])

  const context: RowContext = {
    disabled,
    nested: false,
    duplicates,
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

/** The one place the two things the model cannot look up are supplied: the configured row
 *  ceiling, and what an unlabelled legacy position is called once it is read as fixed text. */
function readBody(body: string, t: TFunction<'templates'>): BodyRead {
  return fromBody(body, decode, TEMPLATE_PARSE_OPTIONS, {
    place: t('builder.legacy.place'),
    link: t('builder.legacy.link'),
  })
}

function readBlocks(body: string, t: TFunction<'templates'>): BuilderBlock[] {
  const result = readBody(body, t)
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
  /** Inside a 사진마다 반복, where a row may not ask for data: how many fields a template asks
   *  for is the template's own answer and must not depend on this post's photo count
   *  (TEMPLATE-43). The switch is shown DISABLED with its reason rather than hidden — a control
   *  that silently disappears in a nested list reads as a bug. */
  nested: boolean
  /** Titles more than one row asks under, so the offending rows can say so in place. */
  duplicates: Set<string>
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
  // Five buttons, named for what the reader gets rather than for what the grammar calls it,
  // in the order a post is usually built (TEMPLATE-36). The place and link positions are gone:
  // a thing the author fills in later is fixed text in their own words (TEMPLATE-37).
  const kinds: PaletteKind[] = ['write', 'text', 'photo', 'repeat', 'note']
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
              disabled={disabled}
              onClick={() => onAdd(kind)}
              // The help is VISIBLE text on the button, not a title: a tooltip is unreachable
              // on the phone this is built for, and the help is what tells the five names apart.
              className="h-auto flex-col items-start gap-0 py-2 text-left"
            >
              <span>{t(`builder.palette.${kind}`)}</span>
              <span
                className={typographyStyles({
                  variant: 'meta',
                  className: 'text-content-tertiary',
                })}
              >
                {t(`builder.palette.${kind}Help`)}
              </span>
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
  // A photo row's summary is the only one the UI FORMATS: it is a count, not text the author
  // typed, and it has to say whether the photos stand side by side (TEMPLATE-38).
  const summary =
    block.kind === 'photo'
      ? t(photoSummaryKey(block.count), { count: block.count })
      : blockSummary(block)

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
        {/* The row keeps its KIND and adds the mark: what it contributes to the post has not
            changed, only where its words come from (TEMPLATE-44). */}
        {asksForData(block) && <Badge tone="accent">{t('builder.block.asksForData')}</Badge>}
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
          <BlockFields block={block} context={context} onChange={onChange} />
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
              context={{ ...context, nested: true }}
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
  context,
  onChange,
}: {
  block: BuilderBlock
  context: RowContext
  onChange: (next: BuilderBlock) => void
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  const disabled = context.disabled
  switch (block.kind) {
    case 'write':
      return (
        <>
          <AskFields block={block} context={context} onChange={onChange} />
          <Field
            id={id}
            label={t('builder.block.instruction')}
            value={block.text}
            disabled={disabled}
            onChange={(text) => onChange({ ...block, text })}
          />
        </>
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
        <>
          <AskFields block={block} context={context} onChange={onChange} />
          {/* A 고정 문구 that asks for data has no text of ITS own left — what the post's author
              types takes its place, so the title is this row's whole authored content
              (TEMPLATE-44). The text is kept underneath, which is what makes turning the switch
              back off restore it. */}
          {!asksForData(block) && (
            <Field
              id={id}
              label={t('builder.block.text')}
              value={block.text}
              disabled={disabled}
              multiline
              onChange={(text) => onChange({ ...block, text })}
            />
          )}
        </>
      )
    case 'photo':
      return (
        <Stepper
          label={t('builder.block.count')}
          value={block.count}
          min={1}
          max={TEMPLATE_PHOTO_ROW_MAX}
          disabled={disabled}
          decrementLabel={t('builder.block.fewer')}
          incrementLabel={t('builder.block.more')}
          onChange={(count) => onChange({ ...block, count })}
        />
      )
    case 'repeat':
      return (
        <Typography variant="meta" as="p">
          {/* How many photos ONE iteration takes is the thing a repeat's author has to know
              once a position inside it can hold a row (TEMPLATE-38). */}
          {t('composition.repeatHelp', { count: repeatPhotoCount(block) })}
        </Typography>
      )
  }
}

/** 데이터 받기 on one row: the switch, and the title it asks under once it is on.
 *
 *  Turning it on SEEDS the title from the row's own text, which is what makes the flip cheap for
 *  a body the author already wrote — a 고정 문구 line reading `총평 별점` becomes the question.
 *  The seed is skipped when that text would not fit the title's own ceiling: a title the server
 *  refuses would leave a draft that cannot be saved, which is worse than an empty field with a
 *  counter beside it.
 *
 *  Turning it off drops the title and the row's authored text is there again, because nothing
 *  ever cleared it. */
function AskFields({
  block,
  context,
  onChange,
}: {
  block: AskableBlock
  context: RowContext
  onChange: (next: BuilderBlock) => void
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  // The field shows the RAW title, not the trimmed one: trimming what is in the input would eat
  // a space the moment it is typed, so `총평 별점` could never be written. Trimming belongs to
  // serialization and to the duplicate check, which is what `askTitle` is for.
  const raw = block.ask ?? ''
  const title = askTitle(block)
  // The SWITCH's state, not the title's: clearing the title to retype it must not collapse the
  // field under the caret.
  const on = asksForData(block)
  const duplicate = title !== '' && context.duplicates.has(title)

  return (
    <div className="mb-3">
      <div className="flex min-h-11 items-center gap-3">
        <Switch
          aria-label={t('builder.block.asksForData')}
          checked={on}
          disabled={context.disabled || context.nested}
          onChange={(event) => {
            if (!event.target.checked) {
              onChange({ ...block, ask: undefined })
              return
            }
            const seed = block.text.replace(/\s+/g, ' ').trim()
            onChange({ ...block, ask: seed.length <= TEMPLATE_LIMITS.askLabel ? seed : '' })
          }}
        />
        <Typography variant="body" as="span" className="min-w-0 flex-1">
          {t('builder.block.asksForData')}
        </Typography>
      </div>
      {context.nested ? (
        <FieldMessage>{t('builder.block.askInRepeat')}</FieldMessage>
      ) : (
        <Typography variant="meta" as="p">
          {t('builder.block.asksForDataHelp')}
        </Typography>
      )}
      {on && (
        <div className="mt-3">
          <Field
            id={`${id}-ask`}
            label={t('builder.block.askTitle')}
            value={raw}
            disabled={context.disabled}
            max={TEMPLATE_LIMITS.askLabel}
            invalid={duplicate}
            message={duplicate ? t('builder.reasons.duplicate_ask_label') : undefined}
            onChange={(next) => onChange({ ...block, ask: next })}
          />
        </div>
      )}
    </div>
  )
}

function Field({
  id,
  label,
  value,
  disabled,
  multiline = false,
  max,
  invalid = false,
  message,
  onChange,
}: {
  id: string
  label: string
  value: string
  disabled: boolean
  multiline?: boolean
  /** Present only where the server bounds this field on its own — the data field's title. The
   *  counter mirrors that ceiling; the backend stays authoritative. */
  max?: number
  invalid?: boolean
  message?: string
  onChange: (value: string) => void
}) {
  const messageId = message ? `${id}-message` : undefined
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
          aria-invalid={invalid || undefined}
          aria-describedby={messageId}
          onChange={(event) => onChange(event.target.value)}
          className="mt-1"
        />
      )}
      {message && (
        <FieldMessage id={messageId} role="alert">
          {message}
        </FieldMessage>
      )}
      {max !== undefined && <FieldCount left={remainingChars(value, max)} />}
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
