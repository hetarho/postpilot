import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button, Checkbox, SortableList, Typography } from '@/shared/ui'
import type { CompositionNode } from '../model/composition'
import { CLIP_ACCENTS, COPY_STYLES } from '../model/types'
import {
  compositionDraftTree,
  compositionLiteral,
  compositionOutline,
  newCompositionNode,
  patchCompositionSource,
} from '../lib/composition-author'
import { CompositionInput, CompositionSelect } from './CompositionFields'
import { CompositionTextControls } from './CompositionTextControls'

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
  const groups = all
    .filter((r) => r.node.name === 'group')
    .map((r, i) => ({
      value: r.node.attributes.id,
      label: t('composition.groupNumber', { n: i + 1 }),
    }))
  const labelFor = (node: CompositionNode, i: number) =>
    node.name === 'field'
      ? node.attributes.label || t('composition.newField')
      : node.name === 'text'
        ? t(`composition.role.${node.attributes.role}`, {
            defaultValue: t('composition.role.caption'),
          })
        : t(`composition.node.${node.name}`, {
            n: i + 1,
            defaultValue: t('composition.repairSource'),
          })
  const styles = (root.attributes.styles ?? 'clean').split(/\s+/).filter(Boolean)
  const patch = (node: CompositionNode, next: CompositionNode | null) =>
    onChange(patchCompositionSource(source, node, next))
  const add = (parent: CompositionNode, name: string, inScene: boolean) => {
    const child = newCompositionNode(name, t('composition.newField'), inScene)
    if (name === 'scene' && parent.name === 'repeat' && parent.attributes.for !== 'scenes')
      child.attributes.scope = 'item'
    patch(parent, { ...parent, children: [...parent.children, child] })
  }
  const addButtons = (parent: CompositionNode, names: string[], inScene = false) => (
    <div className="flex flex-wrap gap-2">
      {names.map((name) => (
        <Button key={name} variant="ghost" onClick={() => add(parent, name, inScene)}>
          {t(`composition.add.${name}`, { defaultValue: t('composition.add.text') })}
        </Button>
      ))}
    </div>
  )
  const renderEditor = (node: CompositionNode, label: string, scope: string, repeat: string) => {
    const name = node.name
    const change = (next: CompositionNode) => patch(node, next)
    const attr = (field: string, value: string) =>
      change({ ...node, attributes: { ...node.attributes, [field]: value } })
    const bindings = fields.filter(
      (f) =>
        !f.group || (scope === 'item' && (!repeat || repeat === 'scenes' || repeat === f.group)),
    )
    return (
      <section className="my-3 min-w-0 space-y-4" aria-label={label}>
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
        {name === 'scene' && (
          <CompositionSelect
            label={t('composition.scopeLabel')}
            value={node.attributes.scope ?? 'scene'}
            options={(repeat && repeat !== 'scenes' ? ['item'] : ['scene', 'item', 'context']).map(
              (value) => ({
                value,
                label: t(`composition.scope.${value}`, {
                  defaultValue: t('composition.scope.scene'),
                }),
              }),
            )}
            onChange={(v) => attr('scope', v)}
          />
        )}
        {name === 'repeat' && (
          <CompositionSelect
            label={t('composition.repeatLabel')}
            value={node.attributes.for}
            options={[{ value: 'scenes', label: t('composition.selectedScenes') }, ...groups]}
            onChange={(value) =>
              change({
                ...node,
                attributes: { for: value },
                children: node.children.map((n) =>
                  n.name === 'scene' && value !== 'scenes'
                    ? { ...n, attributes: { ...n.attributes, scope: 'item' } }
                    : n,
                ),
              })
            }
          />
        )}
        {name === 'text' && (
          <CompositionTextControls
            node={node}
            styles={styles}
            inScene={!!scope}
            bindings={bindings}
            onChange={change}
          />
        )}
        {name === 'group' && addButtons(node, ['field'])}
        {name === 'repeat' && addButtons(node, ['scene'])}
        {name === 'scene' && addButtons(node, ['guide', 'text'], true)}
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
  const list = (parent: CompositionNode, path: string, scope = '', repeat = '') => {
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
                {['group', 'scene', 'repeat'].includes(name) &&
                  list(
                    node,
                    key,
                    name === 'scene' ? (node.attributes.scope ?? 'scene') : scope,
                    name === 'repeat' ? node.attributes.for : repeat,
                  )}
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
        selected.scope,
        selected.repeat,
      )
    : null
  const outline = list(root, 'clip')
  return (
    <div className="min-w-0 space-y-6">
      <fieldset className="min-w-0 space-y-3">
        <Typography as="legend" variant="fieldTitle">
          {t('editor.styles')}
        </Typography>
        <div className="flex flex-wrap gap-x-5 gap-y-2">
          {COPY_STYLES.map((style) => (
            <label key={style} className="flex min-h-11 items-center gap-3">
              <Checkbox
                checked={styles.includes(style)}
                onChange={(e) =>
                  patch(root, {
                    ...root,
                    attributes: {
                      ...root.attributes,
                      styles: (e.target.checked
                        ? [...styles, style]
                        : styles.filter((s) => s !== style)
                      ).join(' '),
                    },
                  })
                }
              />
              {t(`style.${style}`)}
            </label>
          ))}
        </div>
        <CompositionSelect
          label={t('editor.accent')}
          value={root.attributes.accent ?? ''}
          options={CLIP_ACCENTS.map((value) => ({ value, label: t(`accent.${value || 'none'}`) }))}
          onChange={(value) =>
            patch(root, { ...root, attributes: { ...root.attributes, accent: value } })
          }
        />
        <CompositionSelect
          label={t('pace.label')}
          value={root.attributes.pace ?? 'steady'}
          options={['steady', 'rapid'].map((value) => ({
            value,
            label: t(`pace.${value}`, { defaultValue: t('pace.steady') }),
          }))}
          onChange={(value) =>
            patch(root, { ...root, attributes: { ...root.attributes, pace: value } })
          }
        />
      </fieldset>
      {activeEditor}
      {outline}
      {addButtons(root, ['field', 'group', 'guide', 'scene', 'repeat', 'text'])}
    </div>
  )
}
