import { useTranslation } from 'react-i18next'
import { Button, Typography } from '@/shared/ui'
import type { CompositionNode } from '../model/composition'
import { boundedChars, compositionLiteral, compositionNode } from '../lib/composition-author'
import { compositionPositionChars } from '../lib/composition-parse'
import { CompositionInput, CompositionSelect } from './CompositionFields'

export type CompositionBindingOption = { value: string; label: string }
/** 자동 is the ABSENCE of the attribute, not a zero: the draft has to stay the
 *  body the author would have typed (CLIP-71). */
function patchChars(n: CompositionNode, chars: string) {
  const next = { ...n.attributes }
  if (chars.trim() === '') delete next.chars
  else next.chars = chars.trim()
  return next
}
/** The 최대 글자 수 control. The bound is the one the parser enforces for this
 *  position (CLIP-116); an empty value is 자동, which keeps that derived cap. */
function CharsInput({
  max,
  value,
  onChange,
}: {
  max: number
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation('clips')
  return (
    <CompositionInput
      label={t('composition.charsLabel')}
      value={value}
      numeric
      hint={max > 0 ? t('composition.charsHint', { max }) : t('composition.charsFreeHint')}
      onChange={(v) => onChange(boundedChars(v, max))}
    />
  )
}
function Parts({
  nodes,
  bindings,
  onChange,
}: {
  nodes: CompositionNode[]
  bindings: CompositionBindingOption[]
  onChange: (nodes: CompositionNode[]) => void
}) {
  const { t } = useTranslation('clips')
  const parts = nodes.length ? nodes : [compositionLiteral('')]
  return (
    <div className="space-y-3">
      {parts.map((part, i) => (
        <div key={i} className="min-w-0 space-y-1">
          {part.name === 'value' ? (
            <CompositionSelect
              label={t('composition.boundField', { n: i + 1 })}
              value={part.attributes.field}
              options={bindings}
              onChange={(field) =>
                onChange(nodes.map((n, j) => (j === i ? { ...n, attributes: { field } } : n)))
              }
            />
          ) : (
            <CompositionInput
              label={t('composition.textPart', { n: i + 1 })}
              value={part.text}
              multiline
              onChange={(text) => onChange(parts.map((n, j) => (j === i ? { ...n, text } : n)))}
            />
          )}
          <Button variant="ghost" onClick={() => onChange(nodes.filter((_, j) => i !== j))}>
            {t('composition.removePart', { n: i + 1 })}
          </Button>
        </div>
      ))}
      <div className="flex flex-wrap gap-2">
        <Button variant="ghost" onClick={() => onChange([...nodes, compositionLiteral(' ')])}>
          {t('composition.addLiteral')}
        </Button>
        <Button
          variant="ghost"
          disabled={!bindings.length}
          onClick={() =>
            onChange([...nodes, compositionNode('value', { field: bindings[0]!.value })])
          }
        >
          {t('composition.addBinding')}
        </Button>
      </div>
    </div>
  )
}

export function CompositionTextControls({
  node,
  bindings,
  onChange,
}: {
  node: CompositionNode
  bindings: CompositionBindingOption[]
  onChange: (node: CompositionNode) => void
}) {
  const { t } = useTranslation('clips')
  const a = node.attributes
  const attr = (key: string, value: string) =>
    onChange({ ...node, attributes: { ...a, [key]: value } })
  const options = (key: string, values: string[]) =>
    values.map((value) => ({
      value,
      label: t(`composition.${key}.${value}`, { defaultValue: t('composition.auto') }),
    }))
  // An intro or outro entry is a list of LINES the author writes: how many of
  // them are drawn is the preset the clip chooses in ① (CLIP-147), so nothing
  // here pads them to a slot count or calls a surplus an error.
  if (a.role === 'hook' || a.role === 'ending') {
    const rows = node.children.filter((child) => child.name === 'row')
    const replace = (next: CompositionNode[]) => onChange({ ...node, children: next })
    const patchRow = (i: number, row: CompositionNode) =>
      replace(rows.map((r, j) => (i === j ? row : r)))
    return (
      <div className="space-y-4">
        <Typography variant="body">{t('composition.design.slotsHelp')}</Typography>
        {rows.map((row, i) => (
          <section
            key={i}
            className="space-y-3"
            aria-label={t('composition.design.slot', { n: i + 1 })}
          >
            <Typography variant="fieldTitle">
              {t('composition.design.slot', { n: i + 1 })}
            </Typography>
            <CompositionSelect
              label={t('composition.design.slotKind', { n: i + 1 })}
              value={row.attributes.kind}
              options={options('kind', ['fixed', 'ai'])}
              onChange={(kind) => patchRow(i, { ...row, attributes: { ...row.attributes, kind } })}
            />
            <CharsInput
              max={compositionPositionChars(a.role, { index: i })}
              value={row.attributes.chars ?? ''}
              onChange={(chars) => patchRow(i, { ...row, attributes: patchChars(row, chars) })}
            />
            <Parts
              nodes={row.children}
              bindings={bindings}
              onChange={(children) => patchRow(i, { ...row, children })}
            />
            <Button variant="ghost" onClick={() => replace(rows.filter((_, j) => j !== i))}>
              {t('composition.design.removeLine', { n: i + 1 })}
            </Button>
          </section>
        ))}
        <Button
          variant="ghost"
          onClick={() => replace([...rows, compositionNode('row', { kind: 'fixed' })])}
        >
          {t('composition.design.addLine')}
        </Button>
      </div>
    )
  }
  return (
    <div className="space-y-4">
      <CompositionSelect
        label={t('composition.kindLabel')}
        value={a.kind}
        options={options('kind', ['fixed', 'ai'])}
        onChange={(v) => attr('kind', v)}
      />
      <Typography variant="body" className="text-content-secondary">
        {t(a.kind === 'ai' ? 'composition.aiHelp' : 'composition.fixedHelp')}
      </Typography>
      <CompositionSelect
        label={t('composition.positionLabel')}
        value={a.position ?? 'auto'}
        options={options('position', ['auto', 'top', 'upper_mid', 'lower_mid', 'bottom', 'header'])}
        onChange={(v) => attr('position', v)}
      />
      <CompositionSelect
        label={t('composition.alignLabel')}
        value={a.align ?? 'center'}
        options={options('align', ['left', 'center', 'right'])}
        onChange={(v) => attr('align', v)}
      />
      <CharsInput
        max={compositionPositionChars(a.role)}
        value={a.chars ?? ''}
        onChange={(chars) => onChange({ ...node, attributes: patchChars(node, chars) })}
      />
      <Parts
        nodes={node.children}
        bindings={bindings}
        onChange={(children) => onChange({ ...node, children })}
      />
    </div>
  )
}
