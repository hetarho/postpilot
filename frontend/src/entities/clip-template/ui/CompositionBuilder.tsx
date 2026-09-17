import { useState } from 'react'
import { CLIP_COMPOSITION_LIMITS } from '@/shared/config'
import { useTranslation } from 'react-i18next'
import { Button, Checkbox, SortableList, Typography } from '@/shared/ui'
import type { CompositionNode } from '../model/composition'
import {
  boundedChars,
  compositionDraftTree,
  compositionLiteral,
  compositionOutline,
  newCompositionNode,
  patchCompositionSource,
} from '../lib/composition-author'
import { CompositionInput } from './CompositionFields'
import { CompositionTextControls } from './CompositionTextControls'

/** An empty value REMOVES the attribute: 자동 is its absence, and the draft has
 *  to stay the body the author would have typed (CLIP-71). */
function withoutEmpty(attributes: Record<string, string>, key: string, value: string) {
  const next = { ...attributes }
  if (value.trim() === '') delete next[key]
  else next[key] = value.trim()
  return next
}
export function CompositionBuilder({
  source,
  onChange,
}: {
  source: string
  onChange: (source: string) => void
}) {
  const { t } = useTranslation('clips')
  const [expanded, setExpanded] = useState('')
  let root: CompositionNode
  try {
    root = compositionDraftTree(source)
  } catch {
    return <Typography variant="body">{t('composition.repairSource')}</Typography>
  }
  const all = compositionOutline(root)
  const fields = all
    .filter((r) => r.node.name === 'field')
    .map((r) => ({
      group: r.group,
      value: r.group ? `${r.group}.${r.node.attributes.id}` : r.node.attributes.id,
      label: r.node.attributes.label,
    }))
  const labelFor = (node: CompositionNode, i: number) =>
    node.name === 'group' && node.attributes.label?.trim()
      ? node.attributes.label
      : node.name === 'field'
        ? node.attributes.label || t('composition.newField')
        : node.name === 'text'
          ? t(`composition.role.${node.attributes.role}`, {
              defaultValue: t('composition.role.badge'),
            })
          : t(`composition.node.${node.name}`, {
              n: i + 1,
              defaultValue: t('composition.repairSource'),
            })
  const patch = (node: CompositionNode, next: CompositionNode | null) =>
    onChange(patchCompositionSource(source, node, next))
  const add = (parent: CompositionNode, name: string) => {
    patch(parent, {
      ...parent,
      children: [...parent.children, newCompositionNode(name, t('composition.newField'))],
    })
  }
  const addButtons = (parent: CompositionNode, names: string[]) => (
    <div className="flex flex-wrap gap-2">
      {names.map((name) => (
        <Button key={name} variant="ghost" onClick={() => add(parent, name)}>
          {t(`composition.add.${name}`, { defaultValue: t('composition.add.text') })}
        </Button>
      ))}
    </div>
  )
  const renderEditor = (node: CompositionNode, label: string) => {
    const name = node.name
    const change = (next: CompositionNode) => patch(node, next)
    const attr = (field: string, value: string) =>
      change({ ...node, attributes: { ...node.attributes, [field]: value } })
    // A template's visible text binds only GLOBAL answers now: an item's facts
    // are what the narration states, never an on-screen label (CLIP-59, CLIP-61).
    const bindings = fields.filter((f) => !f.group)
    return (
      <section className="my-3 min-w-0 space-y-4" aria-label={label}>
        {name === 'group' && (
          <>
            <CompositionInput
              label={t('composition.groupLabel')}
              value={node.attributes.label ?? ''}
              onChange={(v) => attr('label', v)}
            />
            <CompositionInput
              label={t('composition.groupMinimum')}
              value={node.attributes.min ?? '0'}
              numeric
              onChange={(v) => attr('min', v)}
            />
          </>
        )}
        {name === 'field' && (
          <>
            <CompositionInput
              label={t('composition.fieldLabel')}
              value={node.attributes.label ?? ''}
              onChange={(v) => attr('label', v)}
            />
            <CompositionInput
              label={t('composition.fieldPrompt')}
              value={node.children.map((n) => n.text).join('')}
              multiline
              onChange={(v) => change({ ...node, children: [compositionLiteral(v)] })}
            />
            <label className="flex min-h-11 items-center gap-3">
              <Checkbox
                checked={node.attributes.required === 'true'}
                onChange={(e) => attr('required', String(e.target.checked))}
              />
              {t('composition.required')}
            </label>
            <CompositionInput
              label={t('composition.charsLabel')}
              value={node.attributes.chars ?? ''}
              numeric
              hint={t('composition.charsFieldHint')}
              onChange={(v) =>
                change({
                  ...node,
                  attributes: withoutEmpty(
                    node.attributes,
                    'chars',
                    boundedChars(v, CLIP_COMPOSITION_LIMITS.answerChars),
                  ),
                })
              }
            />
          </>
        )}
        {name === 'stage' && (
          <>
            <CompositionInput
              label={t('composition.stageName')}
              value={node.attributes.name ?? ''}
              onChange={(v) => attr('name', v)}
            />
            <CompositionInput
              label={t('composition.stageIntent')}
              hint={t('composition.stageHint')}
              value={node.children.map((n) => n.text).join('')}
              multiline
              onChange={(v) => change({ ...node, children: [compositionLiteral(v)] })}
            />
          </>
        )}
        {name === 'guide' && (
          <CompositionInput
            label={t('composition.guidance')}
            value={node.children.map((n) => n.text).join('')}
            multiline
            onChange={(v) => change({ ...node, children: [compositionLiteral(v)] })}
          />
        )}
        {name === 'text' && (
          <CompositionTextControls node={node} bindings={bindings} onChange={change} />
        )}
        {name === 'group' && addButtons(node, ['field'])}
        <Button
          variant="ghost"
          onClick={() => {
            patch(node, null)
            setExpanded('')
          }}
        >
          {t('composition.remove', { label })}
        </Button>
      </section>
    )
  }
  const list = (parent: CompositionNode, path: string) => {
    const nodes = parent.children.filter((n) => n.name !== '#text')
    return (
      <SortableList
        labels={{ drag: t('editor.drag'), up: t('editor.up'), down: t('editor.down') }}
        onReorder={(from, to) => {
          const children = [...parent.children]
          const a = children.indexOf(nodes[from]),
            b = children.indexOf(nodes[to])
          children.splice(b, 0, children.splice(a, 1)[0])
          patch(parent, { ...parent, children })
          setExpanded('')
        }}
        items={nodes.map((node, i) => {
          const key = `${path}.${parent.children.indexOf(node)}`
          const name = node.name
          const label = labelFor(node, i)
          return {
            id: key,
            content: (
              <div className="min-w-0">
                <Button
                  variant="ghost"
                  className="max-w-full break-words whitespace-normal"
                  aria-expanded={expanded === key}
                  onClick={() => setExpanded(expanded === key ? '' : key)}
                >
                  {label}
                </Button>
                {name === 'group' && list(node, key)}
              </div>
            ),
          }
        })}
      />
    )
  }
  const selected = all.find((row) => row.key === expanded)
  const activeEditor = selected
    ? renderEditor(
        selected.node,
        labelFor(
          selected.node,
          selected.parent.children.filter((n) => n.name !== '#text').indexOf(selected.node),
        ),
      )
    : null
  const outline = list(root, 'clip')
  return (
    <div className="min-w-0 space-y-6">
      <Typography variant="meta">{t('composition.projectSettings')}</Typography>
      {activeEditor}
      {outline}
      {addButtons(root, ['field', 'group', 'stage', 'guide', 'hook', 'caption', 'ending', 'badge'])}
    </div>
  )
}
