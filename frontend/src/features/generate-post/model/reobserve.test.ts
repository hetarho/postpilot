import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import type { PostImage } from '@/entities/image'
import { ObservationSchema } from '@/shared/api'
import { defaultSelection, needsPicker, reobserveRows, storedObservationModels } from './reobserve'

const image = (filename: string): PostImage => ({
  id: filename,
  filename,
  width: 100,
  height: 100,
  bytes: 1000,
  viewUrl: `https://example.test/${filename}`,
})

const observation = (filename: string, fields: { scene?: string; model?: string } = {}) =>
  create(ObservationSchema, { file: filename, scene: fields.scene ?? '봤음', model: fields.model })

describe('reobserveRows', () => {
  it('is one row per attached photo, in post order, pairing by exact filename', () => {
    const rows = reobserveRows(
      [image('B.jpg'), image('A.jpg')],
      [observation('A.jpg'), observation('B.jpg')],
    )
    expect(rows.map((row) => row.filename)).toEqual(['B.jpg', 'A.jpg'])
    expect(rows.every((row) => !row.forced)).toBe(true)
  })

  it('forces a photo with no stored entry', () => {
    const rows = reobserveRows([image('A.jpg'), image('NEW.jpg')], [observation('A.jpg')])
    expect(rows.map((row) => row.forced)).toEqual([false, true])
    expect(rows[1].stored).toBeUndefined()
  })

  it('forces a photo whose stored entry carries no eyesight, and offers it nothing to reuse', () => {
    const rows = reobserveRows([image('A.jpg')], [observation('A.jpg', { scene: '' })])
    expect(rows[0].forced).toBe(true)
    expect(rows[0].stored).toBeUndefined()
  })

  it('does not treat provenance alone as eyesight', () => {
    const rows = reobserveRows(
      [image('A.jpg')],
      [observation('A.jpg', { scene: '', model: 'p/m' })],
    )
    expect(rows[0].forced).toBe(true)
  })
})

describe('defaultSelection', () => {
  it('checks the forced photos and nothing else', () => {
    const rows = reobserveRows(
      [image('A.jpg'), image('NEW.jpg'), image('B.jpg')],
      [observation('A.jpg'), observation('B.jpg')],
    )
    expect(defaultSelection(rows)).toEqual(['NEW.jpg'])
  })

  it('is empty when every photo has something to reuse', () => {
    const rows = reobserveRows([image('A.jpg')], [observation('A.jpg')])
    expect(defaultSelection(rows)).toEqual([])
  })
})

describe('needsPicker', () => {
  it('is false for a post with no photos', () => {
    expect(needsPicker([], [observation('A.jpg')])).toBe(false)
  })

  it('is false with no stored snapshot at all', () => {
    expect(needsPicker([image('A.jpg')], [])).toBe(false)
  })

  it('is false when nothing stored is reusable', () => {
    expect(needsPicker([image('A.jpg')], [observation('A.jpg', { scene: '' })])).toBe(false)
  })

  it('is true as soon as one photo has something to reuse', () => {
    expect(needsPicker([image('A.jpg'), image('NEW.jpg')], [observation('A.jpg')])).toBe(true)
  })
})

describe('storedObservationModels', () => {
  it('deduplicates in row order and ignores photos with nothing to reuse', () => {
    const rows = reobserveRows(
      [image('A.jpg'), image('B.jpg'), image('C.jpg'), image('NEW.jpg')],
      [
        observation('A.jpg', { model: 'p/one' }),
        observation('B.jpg', { model: 'p/two' }),
        observation('C.jpg', { model: 'p/one' }),
      ],
    )
    expect(storedObservationModels(rows)).toEqual(['p/one', 'p/two'])
  })

  it('reports an entry written before provenance existed as an empty ref', () => {
    const rows = reobserveRows([image('A.jpg')], [observation('A.jpg')])
    expect(storedObservationModels(rows)).toEqual([''])
  })
})
