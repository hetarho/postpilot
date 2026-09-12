import { describe, expect, it } from 'vitest'
import i18next from 'i18next'
import { CLIP_COMPOSITION_EXAMPLE, clipCompositionGuide } from '../model/composition-guide'
import { parseClipComposition } from './composition-parse'
import {
  compositionDraftTree,
  compositionOutline,
  patchCompositionSource,
} from './composition-author'
import { sampleClipComposition } from './composition-sample'

describe('composition authoring contract', () => {
  it('bounds syntax-only builder input before building a tree', () => {
    expect(() => compositionDraftTree(`<clip version="1">${'x'.repeat(16000)}</clip>`)).toThrow(
      'source_limit',
    )
  })
  it.each(['ko', 'en'])(
    'copies one supported example and exact grammar in %s',
    async (language) => {
      const previous = i18next.language
      try {
        await i18next.changeLanguage(language)
        const guide = clipCompositionGuide()
        expect(guide).toContain(CLIP_COMPOSITION_EXAMPLE)
        expect(guide).toContain('basis="whole|output-start|output-end|cut"')
        const doc = parseClipComposition(CLIP_COMPOSITION_EXAMPLE)
        const preview = sampleClipComposition(doc, 30000, (label, i) => `${label} ${i}`)
        expect(preview.elements.filter((e) => e.element.id === 'price')).toHaveLength(2)
        expect(
          new Set(preview.elements.filter((e) => e.element.id === 'price').map((e) => e.itemId))
            .size,
        ).toBe(2)
      } finally {
        await i18next.changeLanguage(previous)
      }
    },
  )
  it('does not discard an invalid interval when editing its syntax tree', () => {
    const source =
      '<clip version="1">\n<text id="x" kind="fixed" role="info" basis="output-end" start="-1" end="-4">💡 exact &amp; text</text>\n</clip>'
    const node = compositionOutline(compositionDraftTree(source))[0].node
    expect(() => parseClipComposition(source)).toThrow('invalid_interval')
    const corrected = patchCompositionSource(source, node, {
      ...node,
      attributes: { ...node.attributes, end: '0' },
    })
    expect(parseClipComposition(corrected).elements[0].parts[0].literal).toBe('💡 exact & text')
  })
})
