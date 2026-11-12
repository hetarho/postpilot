import { describe, expect, it } from 'vitest'
import { parseClipComposition } from '@/entities/clip-template/@x/clip-project'
import {
  emptyClipProject,
  normalizeClipProject,
  savableClipProject,
  validClipProject,
  type ClipProjectDraft,
} from './types'
const composition = parseClipComposition(
  '<clip version="1"><field id="place" label="장소" required="true"/>' +
    '<text id="hook" kind="fixed" role="hook" basis="output-start"/>' +
    '<text id="ending" kind="fixed" role="ending" basis="output-end"/></clip>',
)
const answered = (place: string) => ({ values: { place }, items: {}, associations: [] })
const draft = (): ClipProjectDraft => ({
  ...emptyClipProject(),
  title: '경험',
  videoTemplateId: 'owned',
  // The length is chosen in ① (CLIP-130), so a complete setup carries one even
  // though the draft a project is minted from does not.
  targetDurationMs: 30000,
  compositionInputs: answered('제주'),
})
describe('clip setup validation', () => {
  it.each([
    { title: '😀'.repeat(101) },
    { targetDurationMs: 0 },
    { targetDurationMs: 14999 },
    { targetDurationMs: 60001 },
    { targetDurationMs: NaN },
    { targetDurationMs: 15000.1 },
    { compositionInputs: answered('') },
    { compositionInputs: answered(' ') },
    { compositionInputs: { values: { unknown: '값' }, items: {}, associations: [] } },
  ])('rejects invalid setup %j', (patch) => {
    expect(validClipProject({ ...draft(), ...patch }, composition)).toBe(false)
  })
  // Minting needs nothing the owner chose (CLIP-130): an empty draft — no title, no answers,
  // no length and no template — is what `/clips/new`'s first save may send, and it saves.
  it('saves an empty new draft and an untitled project, with or without a template', () => {
    const minting = emptyClipProject()
    expect(savableClipProject(minting)).toBe(true)
    expect(savableClipProject({ ...minting, videoTemplateId: 'owned' })).toBe(true)
    expect(validClipProject({ ...draft(), videoTemplateId: '' }, composition)).toBe(true)
    // A saved project's title is optional: an empty one saves and generates (CLIP-130).
    expect(validClipProject({ ...draft(), title: '' }, composition)).toBe(true)
    expect(savableClipProject({ ...draft(), title: '' })).toBe(true)
    expect(savableClipProject({ ...minting, ratio: 'wide' as ClipProjectDraft['ratio'] })).toBe(
      false,
    )
  })
  it('requires the outline the project answers', () => {
    expect(validClipProject(draft(), undefined)).toBe(false)
    expect(validClipProject(draft(), composition)).toBe(true)
    expect(
      validClipProject(
        { ...draft(), title: '😀'.repeat(100), targetDurationMs: 60000 },
        composition,
      ),
    ).toBe(true)
    expect(normalizeClipProject({ ...draft(), title: ' 경험 ' })).toMatchObject({ title: '경험' })
  })
})

describe('normalizeClipProject against key order', () => {
  const base = {
    title: '클립',
    videoTemplateId: 'template',
    ratio: 'vertical' as const,
    targetDurationMs: 30000,
    disclosure: '' as const,
  }
  // The server holds these as maps, so the same project can come back naming the same fields in a
  // different order. The form compares serialized drafts to decide whether it is in sync
  // (CLIP-39), so an order-sensitive comparison left an edit 'unsaved' for good.
  it('serializes the same draft identically whatever order the server names its fields in', () => {
    const one = normalizeClipProject({
      ...base,
      compositionInputs: {
        values: { region: '강릉', place: '가게' },
        items: { menu: [{ id: 'first', values: { price: '9000', name: '갈비' } }] },
        associations: [],
      },
    })
    const other = normalizeClipProject({
      ...base,
      compositionInputs: {
        values: { place: '가게', region: '강릉' },
        items: { menu: [{ id: 'first', values: { name: '갈비', price: '9000' } }] },
        associations: [],
      },
    })
    expect(JSON.stringify(one)).toBe(JSON.stringify(other))
  })
  it('keeps the order that is the owner’s own', () => {
    const value = normalizeClipProject({
      ...base,
      compositionInputs: {
        values: {},
        items: {
          menu: [
            { id: 'second', values: {} },
            { id: 'first', values: {} },
          ],
        },
        associations: [],
      },
    })
    expect(value.compositionInputs?.items.menu.map((v) => v.id)).toEqual(['second', 'first'])
  })
})
