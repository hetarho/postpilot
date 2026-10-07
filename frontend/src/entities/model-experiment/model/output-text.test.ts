import { create } from '@bufbuild/protobuf'
import { expect, it } from 'vitest'
import { BlockSchema, ObservationSchema, PostContentSchema } from '@/shared/api'
import { legacyOutputText } from './output-text'

it('copies the complete retained post including summary, paragraphs, lists, captions and tags', () => {
  const content = create(PostContentSchema, {
    title: 'Saved title',
    summary: 'Saved summary',
    tags: ['saved'],
    blocks: [
      create(BlockSchema, { content: 'First paragraph\nSecond line' }),
      create(BlockSchema, { items: ['First item', 'Second item'] }),
      create(BlockSchema, { file: 'one.jpg', caption: 'First image' }),
      create(BlockSchema, { files: ['two.jpg', 'three.jpg'], caption: 'Saved group' }),
    ],
  })
  expect(legacyOutputText({ kind: 'write', content })).toBe(
    'Saved title\n\nSaved summary\n\nFirst paragraph\nSecond line\n\n• First item\n• Second item\n\none.jpg: First image\n\ntwo.jpg, three.jpg: Saved group\n\n#saved',
  )
})

it('retains exact voice text without exporting fingerprint, supplier or usage metadata', () => {
  expect(
    legacyOutputText({ kind: 'voice', text: 'Original voice\nOriginal paragraph', comparison: [] }),
  ).toBe('Original voice\nOriginal paragraph')
})

it('exports retained observation facts without leaking a blinded model identity or wire metadata', () => {
  const observation = create(ObservationSchema, {
    file: 'original.jpg',
    scene: 'Saved scene',
    objects: ['table'],
    peoplePresent: true,
    model: 'private-provider/model',
    events: ['Saved event'],
    speech: 'Saved speech',
  })
  const exported = legacyOutputText({ kind: 'observe', observations: [observation] })
  expect(JSON.parse(exported)).toEqual([
    {
      file: 'original.jpg',
      scene: 'Saved scene',
      mood: '',
      visibleText: '',
      objects: ['table'],
      peoplePresent: true,
      events: ['Saved event'],
      speech: 'Saved speech',
    },
  ])
  expect(exported).not.toContain('private-provider')
  expect(exported).not.toContain('$typeName')
})
