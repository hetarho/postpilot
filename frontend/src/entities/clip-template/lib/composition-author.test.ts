import { describe, expect, it } from 'vitest'
import i18next from 'i18next'
import { CLIP_COMPOSITION_EXAMPLE, clipCompositionGuide } from '../model/composition-guide'
import { parseClipComposition } from './composition-parse'
import {
  boundedChars,
  compositionDraftTree,
  compositionOutline,
  patchCompositionSource,
  newCompositionNode,
} from './composition-author'
import { compositionPositionChars } from './composition-parse'
import { serializeCompositionNode } from './composition-xml'
import { sampleClipComposition } from './composition-sample'

describe('composition authoring contract', () => {
  it('creates a group with editable name and minimum defaults', () => {
    const group = newCompositionNode('group', '새 정보')
    const doc = parseClipComposition(
      `<clip version="1" intro="b" caption="bold" outro="e">${serializeCompositionNode(group)}<text id="intro" kind="fixed" role="hook" basis="output-start"/><text id="outro" kind="fixed" role="ending" basis="output-end"/></clip>`,
    )
    expect(doc.groups[0]).toMatchObject({ id: group.attributes.id, label: '', min: 0 })
    expect(group.attributes).toMatchObject({ label: '', min: '0' })
    expect(doc.fields[0].label).toBe('새 정보')
  })

  it('holds an authored maximum inside what its position allows', () => {
    // Outro E's middle slot is the 6-character display (CDS-20), so the control
    // cannot author the invalid_max the parser would refuse.
    const display = compositionPositionChars(
      { intro: 'a', caption: 'bold', outro: 'e' },
      'ending',
      {
        index: 1,
      },
    )
    expect(display).toBe(6)
    expect(boundedChars('20', display)).toBe('6')
    expect(boundedChars('4', display)).toBe('4')
    expect(boundedChars('0', display)).toBe('')
    expect(boundedChars('여섯', display)).toBe('')
    // 자동 is the attribute's absence, so an emptied control stays empty.
    expect(boundedChars('', display)).toBe('')
    // A position with no count of its own keeps the number as typed.
    expect(compositionPositionChars({ intro: 'a', caption: 'bold', outro: 'e' }, 'caption')).toBe(0)
    expect(boundedChars('40', 0)).toBe('40')
  })

  it('carries an authored maximum through the body unchanged', () => {
    const body =
      '<clip version="1" intro="a" caption="bold" outro="e">' +
      '<field id="dish" label="메뉴" chars="6">메뉴명</field>' +
      '<text id="opening" kind="fixed" role="hook" basis="output-start">' +
      '<row chars="7">여는 문구</row><row>작은 문구</row></text>' +
      '<text id="closing" kind="fixed" role="ending" basis="output-end">' +
      '<row>라벨</row><row>큰 글씨</row><row>닫는 문구</row></text></clip>'
    const doc = parseClipComposition(body)
    expect(doc.fields[0].chars).toBe(6)
    expect(doc.maxima.dish).toBe(6)
    expect(doc.elements[0].rows[0].chars).toBe(7)
    expect(serializeCompositionNode(doc.root)).toContain('chars="7"')
    expect(parseClipComposition(serializeCompositionNode(doc.root)).elements[0].rows[0].chars).toBe(
      7,
    )
  })

  it('names the element and its 1-based line when a pasted maximum is too large', () => {
    const body = [
      '<clip version="1" intro="a" caption="bold" outro="e">',
      '  <text id="opening" kind="fixed" role="hook" basis="output-start">',
      '    <row chars="9">여는 문구</row><row>작은 문구</row>',
      '  </text>',
      '  <text id="closing" kind="fixed" role="ending" basis="output-end">',
      '    <row>라벨</row><row>큰 글씨</row><row>닫는 문구</row>',
      '  </text>',
      '</clip>',
    ].join('\n')
    // Intro A's first slot is the 8-character headline, so 9 is refused there
    // and the failure names the text that carries the row, on the text's line.
    expect(() => parseClipComposition(body)).toThrow('invalid_max')
    try {
      parseClipComposition(body)
    } catch (error) {
      expect(error).toMatchObject({ reason: 'invalid_max', elementId: 'opening', line: 2 })
    }
  })

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
        expect(guide).toContain('basis="whole|output-start|output-end"')
        expect(guide).toContain('unsupported_section, unsupported_role, unsupported_basis')
        expect(guide).toContain('chars is a positive integer')
        expect(guide).toContain('outro E 22/6/18')
        const doc = parseClipComposition(CLIP_COMPOSITION_EXAMPLE)
        expect(doc.sections).toEqual([])
        // Sample values must fit the example's own maxima (score is 6 characters).
        const preview = sampleClipComposition(doc, 30000, (_label, i) => `${i}`, '자막 예시')
        expect(preview.elements.map((e) => e.element.id)).toEqual(
          expect.arrayContaining(['disclosure_badge', 'intro', 'closing', 'sample-narration']),
        )
        // The template declares no caption, so the preview supplies one line to
        // judge the caption treatment against the skeleton (CLIP-4).
        const narration = preview.elements.find((e) => e.element.id === 'sample-narration')!
        expect(narration.element.role).toBe('caption')
        expect(narration.text).toBe('자막 예시')
        expect(narration.endMs).toBeLessThanOrEqual(preview.durationMs)
      } finally {
        await i18next.changeLanguage(previous)
      }
    },
  )
  it('does not discard an invalid interval when editing its syntax tree', () => {
    const source =
      '<clip version="1" intro="b" caption="bold" outro="e">\n<text id="x" kind="fixed" role="info" basis="output-end" start="-1" end="-4">💡 exact &amp; text</text>\n<text id="empty-hook" kind="fixed" role="hook" basis="output-start"/><text id="empty-ending" kind="fixed" role="ending" basis="output-end"/></clip>'
    const node = compositionOutline(compositionDraftTree(source))[0].node
    expect(() => parseClipComposition(source)).toThrow('invalid_interval')
    const corrected = patchCompositionSource(source, node, {
      ...node,
      attributes: { ...node.attributes, end: '0' },
    })
    expect(parseClipComposition(corrected).elements[0].parts[0].literal).toBe('💡 exact & text')
  })
})
