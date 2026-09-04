import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Trash2 } from 'lucide-react'
import { decode } from '../lib/grammar'
import { remainingChars, TEMPLATE_LIMITS } from '../model/types'
import {
  Button,
  FieldLabel,
  FieldMessage,
  SegmentedControl,
  SortableList,
  TextField,
  Textarea,
  Typography,
  typographyStyles,
} from '@/shared/ui'
import {
  fromBody,
  newBlock,
  reorder,
  toValidBody,
  type BuilderBlock,
  type BuilderBlockKind,
} from '../model/blocks'

type Mode = 'visual' | 'source'

/** The template body editor: a structure view over the grammar and a source view over the
 *  same string (spec/tech/post-template-grammar.md).
 *
 *  It lives in the ENTITY rather than in a feature because it performs no mutation — it is a
 *  controlled input over one of the template's own fields, and both `create-template` and
 *  `edit-template` need it. Two features importing each other is what FSD forbids
 *  (ARCHITECTURE §3.1), and the thing they share is the entity.
 *
 *  The BODY is the single source of truth in both modes — the structure view parses it on
 *  entry and serializes it on every edit, so switching modes cannot desync them and there is
 *  no second representation to keep. A body that does not parse can only be edited in source
 *  mode, which is also where its error is shown: the structure view has nothing to show for
 *  bytes it could not read, and guessing would drop the structure the author asked for. */
export function TemplateBodyEditor({
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
  const { t } = useTranslation(['templates', 'common'])
  const id = useId()
  const read = useMemo(() => fromBody(value, decode), [value])
  // A body that does not parse forces source mode: there is nothing for the structure view to
  // render, and it must not silently drop what it could not read.
  const [requested, setRequested] = useState<Mode>('visual')
  const mode: Mode = read.ok ? requested : 'source'
  const left = remainingChars(value, TEMPLATE_LIMITS.body)

  return (
    <div className={className}>
      <div className="flex flex-wrap items-center gap-3">
        <SegmentedControl
          value={mode}
          options={[
            { value: 'visual', label: t('builder.visual', { ns: 'templates' }) },
            { value: 'source', label: t('builder.source', { ns: 'templates' }) },
          ]}
          onChange={setRequested}
          ariaLabel={t('builder.mode', { ns: 'templates' })}
          controls={`${id}-panel`}
          disabled={disabled || !read.ok}
          className="max-w-64"
        />
        {left < 0 ? (
          <FieldMessage role="status">
            {t('count.exceeded', { ns: 'common', count: -left })}
          </FieldMessage>
        ) : (
          <Typography variant="meta" as="p">
            {t('count.remaining', { ns: 'common', count: left })}
          </Typography>
        )}
      </div>

      {!read.ok && (
        <FieldMessage role="alert" className="mt-3">
          {t('builder.parseFailed', {
            ns: 'templates',
            line: read.failure.line,
            reason: t(`builder.reasons.${read.failure.reason}`, { ns: 'templates' }),
          })}{' '}
          {t('builder.parseFailedHint', { ns: 'templates' })}
        </FieldMessage>
      )}

      {/* Deliberately unnamed: the tablist above already carries the switch's name, and a
          second element answering to the field's name would make `getByLabelText` — and a
          screen reader's form-control list — ambiguous with the source field's own label. */}
      <div id={`${id}-panel`} role="tabpanel" className="mt-3">
        {mode === 'source' ? (
          <>
            {/* The label is rendered only in this mode: it names the source field, and a
                `<label for>` pointing at an element the other mode does not render would be a
                broken association in the accessibility tree. */}
            <FieldLabel htmlFor={`${id}-source`} className="sr-only">
              {t('create.body', { ns: 'templates' })}
            </FieldLabel>
            <Textarea
              id={`${id}-source`}
              value={value}
              onChange={(event) => onChange(event.target.value)}
              rows={10}
              autoGrow
              disabled={disabled}
              spellCheck={false}
              // `mono` for the same reason the read view uses it: a template's literal
              // separator lines are alignment-significant while they are being typed.
              className={typographyStyles({ variant: 'body', mono: true })}
            />
            <Typography variant="meta" as="p" className="mt-2">
              {t('builder.sourceHelp', { ns: 'templates' })}
            </Typography>
          </>
        ) : (
          <StructureEditor value={value} onChange={onChange} disabled={disabled} />
        )}
      </div>
    </div>
  )
}

function StructureEditor({
  value,
  onChange,
  disabled,
}: {
  value: string
  onChange: (body: string) => void
  disabled: boolean
}) {
  const { t } = useTranslation('templates')
  // The rows are LOCAL state, seeded from the body. A block exists as a row the moment it is
  // added, before anything is typed into it, and an incomplete row contributes no bytes — so
  // the body the parent holds always parses, while the editor can still show an empty field to
  // type into. `emitted` is what closes the loop: a value that is not what this editor last
  // produced came from outside (the source view, a refetch), and only then are the rows reseeded.
  const [blocks, setBlocks] = useState<BuilderBlock[]>(() => readBlocks(value))
  const emitted = useRef(value)
  useEffect(() => {
    if (value === emitted.current) return
    emitted.current = value
    setBlocks(readBlocks(value))
  }, [value])

  const push = (next: BuilderBlock[]) => {
    setBlocks(next)
    const body = toValidBody(next)
    emitted.current = body
    onChange(body)
  }
  const replace = (index: number, next: BuilderBlock) =>
    push(blocks.map((block, i) => (i === index ? next : block)))

  return (
    <div>
      {blocks.length === 0 ? (
        <Typography variant="body" className="text-content-tertiary">
          {t('builder.empty')}
        </Typography>
      ) : (
        <SortableList
          items={blocks.map((block, index) => ({
            id: block.id,
            content: (
              <BlockEditor
                block={block}
                disabled={disabled}
                onChange={(next) => replace(index, next)}
                onRemove={() => push(blocks.filter((_, i) => i !== index))}
              />
            ),
          }))}
          onReorder={(from, to) => push(reorder(blocks, from, to))}
          labels={{
            drag: t('builder.block.moveUp'),
            up: t('builder.block.moveUp'),
            down: t('builder.block.moveDown'),
          }}
        />
      )}
      <AddBlock
        label={t('builder.add')}
        disabled={disabled}
        onAdd={(kind) => push([...blocks, newBlock(kind)])}
        allowRepeat
      />
    </div>
  )
}

function readBlocks(body: string): BuilderBlock[] {
  const result = fromBody(body, decode)
  return result.ok ? result.blocks : []
}

/** One block's own fields. A slot has no instruction to give a model — only a label for the
 *  person who fills it — which is exactly the distinction the grammar draws. */
function BlockEditor({
  block,
  disabled,
  onChange,
  onRemove,
}: {
  block: BuilderBlock
  disabled: boolean
  onChange: (next: BuilderBlock) => void
  onRemove: () => void
}) {
  const { t } = useTranslation('templates')
  const id = useId()
  const kindLabel =
    block.kind === 'slot'
      ? t(`builder.palette.${block.slotKind}`)
      : t(`builder.palette.${block.kind === 'text' ? 'text' : block.kind}`)

  return (
    <div>
      <div className="flex items-center justify-between gap-2">
        <Typography variant="eyebrow" as="p">
          {kindLabel}
        </Typography>
        <Button
          variant="danger"
          size="icon"
          aria-label={t('builder.block.remove')}
          disabled={disabled}
          onClick={onRemove}
        >
          <Trash2 className="size-5" />
        </Button>
      </div>

      {block.kind === 'write' && (
        <Field
          id={id}
          label={t('builder.block.instruction')}
          value={block.text}
          disabled={disabled}
          onChange={(text) => onChange({ ...block, text })}
        />
      )}
      {block.kind === 'text' && (
        <MultilineField
          id={id}
          label={t('builder.block.text')}
          value={block.text}
          disabled={disabled}
          onChange={(text) => onChange({ ...block, text })}
        />
      )}
      {block.kind === 'note' && (
        <Field
          id={id}
          label={t('builder.block.note')}
          value={block.text}
          disabled={disabled}
          onChange={(text) => onChange({ ...block, text })}
        />
      )}
      {block.kind === 'slot' && block.slotKind !== 'photo' && (
        <Field
          id={id}
          label={t('builder.block.label')}
          value={block.label}
          disabled={disabled}
          onChange={(label) => onChange({ ...block, label })}
        />
      )}
      {block.kind === 'slot' && block.slotKind === 'photo' && (
        <Typography variant="meta" as="p" className="mt-1">
          {t('builder.palette.photoHelp')}
        </Typography>
      )}
      {block.kind === 'repeat' && (
        <RepeatEditor block={block} disabled={disabled} onChange={onChange} />
      )}
    </div>
  )
}

/** A repeat's children are the same editor one level down, minus the repeat option: the
 *  grammar forbids a repeat inside a repeat, so the palette must not offer one. */
function RepeatEditor({
  block,
  disabled,
  onChange,
}: {
  block: Extract<BuilderBlock, { kind: 'repeat' }>
  disabled: boolean
  onChange: (next: BuilderBlock) => void
}) {
  const { t } = useTranslation('templates')
  const children = block.children
  return (
    <div className="mt-2">
      <Typography variant="meta" as="p">
        {t('builder.block.repeatLabel')}
      </Typography>
      <div className="bg-surface-recessed mt-2 rounded-md p-3">
        {children.length === 0 ? (
          <Typography variant="body" className="text-content-tertiary">
            {t('builder.empty')}
          </Typography>
        ) : (
          <SortableList
            items={children.map((child, index) => ({
              id: child.id,
              content: (
                <BlockEditor
                  block={child}
                  disabled={disabled}
                  onChange={(next) =>
                    onChange({
                      ...block,
                      children: children.map((c, i) => (i === index ? next : c)),
                    })
                  }
                  onRemove={() =>
                    onChange({ ...block, children: children.filter((_, i) => i !== index) })
                  }
                />
              ),
            }))}
            onReorder={(from, to) => onChange({ ...block, children: reorder(children, from, to) })}
            labels={{
              drag: t('builder.block.moveUp'),
              up: t('builder.block.moveUp'),
              down: t('builder.block.moveDown'),
            }}
          />
        )}
        <AddBlock
          label={t('builder.block.addInside')}
          disabled={disabled}
          onAdd={(kind) => onChange({ ...block, children: [...children, newBlock(kind)] })}
          allowRepeat={false}
        />
      </div>
    </div>
  )
}

/** The palette's commands. `slot` is deliberately absent: the three slot KINDS are what a
 *  person picks, and a bare "slot" would be a choice with no meaning. */
type PaletteKind = 'write' | 'text' | 'photo' | 'place' | 'link' | 'note' | 'repeat'

/** The palette. A wrapping row of buttons rather than a menu: these are COMMANDS, and a
 *  `Menu` is a radio group over a current value — picking the option that already equals the
 *  value emits no change at all, which is exactly the bug a menu would hide here. Seven short
 *  Korean labels wrap into three rows at 360px, and being visible makes the vocabulary of the
 *  grammar discoverable instead of hidden behind a trigger. */
function AddBlock({
  label,
  disabled,
  onAdd,
  allowRepeat,
}: {
  label: string
  disabled: boolean
  onAdd: (kind: BuilderBlockKind) => void
  allowRepeat: boolean
}) {
  const { t } = useTranslation('templates')
  const kinds: PaletteKind[] = ['write', 'text', 'photo', 'place', 'link', 'note']
  if (allowRepeat) kinds.splice(5, 0, 'repeat')
  return (
    <div role="group" aria-label={label} className="mt-3 flex flex-wrap gap-2">
      {kinds.map((kind) => (
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
  )
}

function Field({
  id,
  label,
  value,
  disabled,
  onChange,
}: {
  id: string
  label: string
  value: string
  disabled: boolean
  onChange: (value: string) => void
}) {
  return (
    <div className="mt-2">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <TextField
        id={id}
        value={value}
        disabled={disabled}
        autoComplete="off"
        onChange={(event) => onChange(event.target.value)}
        className="mt-1"
      />
    </div>
  )
}

function MultilineField({
  id,
  label,
  value,
  disabled,
  onChange,
}: {
  id: string
  label: string
  value: string
  disabled: boolean
  onChange: (value: string) => void
}) {
  return (
    <div className="mt-2">
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <Textarea
        id={id}
        value={value}
        disabled={disabled}
        rows={2}
        autoGrow
        onChange={(event) => onChange(event.target.value)}
        className="mt-1"
      />
    </div>
  )
}
