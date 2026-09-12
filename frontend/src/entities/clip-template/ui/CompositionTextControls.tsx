import { useTranslation } from 'react-i18next'
import { Button, Checkbox, Typography } from '@/shared/ui'
import type { CompositionNode } from '../model/composition'
import { compositionLiteral, compositionNode } from '../lib/composition-author'
import { CompositionInput, CompositionSelect } from './CompositionFields'

export type CompositionBindingOption = { value: string; label: string }
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
  styles,
  inScene,
  bindings,
  onChange,
}: {
  node: CompositionNode
  styles: string[]
  inScene: boolean
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
  const rows = node.children.filter((c) => c.name === 'row')
  const card = ['hook', 'ending', 'info'].includes(a.role)
  const timing =
    a.basis === 'output-start' ||
    a.basis === 'output-end' ||
    (a.basis === 'cut' && a.start !== undefined)
  const setBasis = (basis: string) => {
    const next: Record<string, string> = { ...a, basis }
    delete next.start
    delete next.end
    if (basis === 'output-start') {
      next.start = '0'
      next.end = '3'
    }
    if (basis === 'output-end') {
      next.start = '-3'
      next.end = '0'
    }
    onChange({ ...node, attributes: next })
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
        label={t('composition.roleLabel')}
        value={a.role}
        options={options('role', ['caption', 'info', 'badge', 'hook', 'ending'])}
        onChange={(v) => attr('role', v)}
      />
      <CompositionSelect
        label={t('composition.styleLabel')}
        value={a.style ?? 'auto'}
        options={[
          { value: 'auto', label: t('composition.auto') },
          ...styles.map((value) => ({
            value,
            label: t(`style.${value}`, { defaultValue: t('composition.auto') }),
          })),
        ]}
        onChange={(v) => attr('style', v)}
      />
      <CompositionSelect
        label={t('composition.positionLabel')}
        value={a.position ?? 'auto'}
        options={options('position', [
          'auto',
          'top',
          'upper_mid',
          'lower_mid',
          'bottom',
          ...(['info', 'badge'].includes(a.role) ? ['header'] : []),
        ])}
        onChange={(v) => attr('position', v)}
      />
      <CompositionSelect
        label={t('composition.alignLabel')}
        value={a.align ?? 'center'}
        options={options('align', ['left', 'center', 'right'])}
        onChange={(v) => attr('align', v)}
      />
      <CompositionSelect
        label={t('composition.timingLabel')}
        value={a.basis}
        options={options('basis', [
          'whole',
          'output-start',
          'output-end',
          ...(inScene ? ['cut'] : []),
        ])}
        onChange={setBasis}
      />
      {a.basis === 'cut' && (
        <label className="flex min-h-11 items-center gap-3">
          <Checkbox
            checked={a.start !== undefined}
            onChange={(e) =>
              e.target.checked
                ? onChange({ ...node, attributes: { ...a, start: '0', end: '3' } })
                : setBasis('cut')
            }
          />
          {t('composition.explicitCut')}
        </label>
      )}
      {timing && (
        <div className="grid min-w-0 gap-3 sm:grid-cols-2">
          <CompositionInput
            label={t('composition.start')}
            value={a.start ?? ''}
            numeric
            onChange={(v) => attr('start', v)}
          />
          <CompositionInput
            label={t('composition.end')}
            value={a.end ?? ''}
            numeric
            onChange={(v) => attr('end', v)}
          />
        </div>
      )}
      {a.basis === 'output-end' && (
        <Typography variant="body">{t('composition.endHelp')}</Typography>
      )}
      {rows.length ? (
        rows.map((row, i) => (
          <section key={i} className="space-y-3">
            <CompositionSelect
              label={t('composition.rowRole', { n: i + 1 })}
              value={row.attributes.role}
              options={options('row', [
                'hook',
                'title',
                'mark',
                'body',
                'caption',
                'label',
                'badge',
              ])}
              onChange={(role) =>
                onChange({
                  ...node,
                  children: rows.map((r, j) => (i === j ? { ...r, attributes: { role } } : r)),
                })
              }
            />
            <Parts
              nodes={row.children}
              bindings={bindings}
              onChange={(children) =>
                onChange({
                  ...node,
                  children: rows.map((r, j) => (i === j ? { ...r, children } : r)),
                })
              }
            />
            <Button
              variant="ghost"
              onClick={() => onChange({ ...node, children: rows.filter((_, j) => i !== j) })}
            >
              {t('composition.removeRow', { n: i + 1 })}
            </Button>
          </section>
        ))
      ) : (
        <Parts
          nodes={node.children}
          bindings={bindings}
          onChange={(children) => onChange({ ...node, children })}
        />
      )}
      {card && (
        <Button
          variant="ghost"
          onClick={() =>
            onChange({
              ...node,
              children: [
                ...(rows.length
                  ? rows
                  : node.children.length
                    ? [compositionNode('row', { role: 'title' }, node.children)]
                    : []),
                compositionNode('row', { role: 'body' }, [compositionLiteral('')]),
              ],
            })
          }
        >
          {t('composition.addRow')}
        </Button>
      )}
    </div>
  )
}
