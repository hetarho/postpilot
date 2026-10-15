import {
  compositionNode,
  serializeCompositionNode,
  type CompositionNode,
} from '@/entities/clip-template'
import type { StageDraft } from '../model/wizard-machine'

/** Stage meaning stays an outline; explicit information is supplied text. Hook and ending
 * rows describe their subject for later writing and carry no shipped sentence or owner fact. */
export function toClipWizardBody(stages: readonly StageDraft[]): string {
  const nodes: CompositionNode[] = []
  const text = (value: string) => compositionNode('#text', {}, [], value)
  for (const stage of stages.filter((entry) => entry.checked)) {
    const id = `wizard_${stage.id.replaceAll('-', '_')}`
    const region = stage.role === 'hook' || stage.role === 'ending'
    nodes.push(
      compositionNode('stage', { name: stage.heading.trim() }, [text(stage.intent.trim())]),
    )
    if (stage.asks) {
      nodes.push(
        compositionNode('field', { id, label: stage.fieldLabel.trim(), required: 'false' }),
      )
      nodes.push(
        compositionNode(
          'text',
          { id: `${id}_answer`, kind: 'fixed', role: region ? stage.role : 'caption' },
          region
            ? [compositionNode('row', { kind: 'fixed' }, [compositionNode('value', { field: id })])]
            : [compositionNode('value', { field: id })],
        ),
      )
    } else if (stage.role === 'hook' || stage.role === 'ending') {
      nodes.push(
        compositionNode('text', { id, kind: 'ai', role: stage.role }, [
          compositionNode('row', { kind: 'ai' }, [text(stage.intent.trim())]),
        ]),
      )
    }
  }
  return serializeCompositionNode(compositionNode('clip', { version: '1' }, nodes))
}
