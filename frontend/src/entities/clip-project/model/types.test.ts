import { describe, expect, it } from 'vitest'
import {
  emptyClipProject,
  normalizeClipProject,
  validClipProject,
  validNewClipProject,
  type ClipProjectDraft,
} from './types'
const draft = (): ClipProjectDraft => ({
  ...emptyClipProject(),
  title: '경험',
  videoTemplateId: 'owned',
  // Every clip carries its ad disclosure, so the campaign type is part of a
  // complete setup (CDS-5, CDS-31).
  disclosure: 'sponsored',
  // The length is chosen in ① (CLIP-130), so a complete setup carries one even
  // though the draft a project is minted from does not.
  targetDurationMs: 30000,
  answers: [{ label: '장소', text: '제주' }],
})
const fields = [{ label: '장소' }]
describe('clip setup validation', () => {
  it.each([
    { title: '' },
    { title: '😀'.repeat(101) },
    { videoTemplateId: '' },
    { targetDurationMs: 0 },
    { targetDurationMs: 14999 },
    { targetDurationMs: 90001 },
    { targetDurationMs: NaN },
    { targetDurationMs: 15000.1 },
    { answers: [] },
    { answers: [{ label: '장소', text: ' ' }] },
    { answers: [{ label: '장소', text: '😀'.repeat(501) }] },
    { disclosure: '' as const },
    { disclosure: 'editorial' as ClipProjectDraft['disclosure'] },
    { cta: 'subscribe' as ClipProjectDraft['cta'] },
  ])('rejects invalid setup %j', (patch) => {
    expect(validClipProject({ ...draft(), ...patch }, fields)).toBe(false)
  })
  // Minting settles the ratio and nothing else (CLIP-130): a draft with no answers,
  // no campaign type and no length is what `/clips/new` sends.
  it('mints from the title, the template and the ratio alone', () => {
    const minting = { ...emptyClipProject(), title: '새 경험', videoTemplateId: 'owned' }
    expect(validNewClipProject(minting)).toBe(true)
    expect(validClipProject(minting, fields)).toBe(false)
    expect(validNewClipProject({ ...minting, title: ' ' })).toBe(false)
    expect(validNewClipProject({ ...minting, videoTemplateId: '' })).toBe(false)
    expect(validNewClipProject({ ...minting, ratio: 'wide' as ClipProjectDraft['ratio'] })).toBe(
      false,
    )
  })
  it('requires the currently owned template and its current questions', () => {
    expect(validClipProject(draft(), undefined)).toBe(false)
    expect(validClipProject(draft(), [{ label: '새 질문' }])).toBe(false)
    expect(validClipProject(draft(), fields)).toBe(true)
    expect(
      validClipProject({ ...draft(), title: '😀'.repeat(100), targetDurationMs: 90000 }, fields),
    ).toBe(true)
    expect(
      normalizeClipProject({
        ...draft(),
        title: ' 경험 ',
        answers: [{ label: '장소', text: '  보존  ' }],
      }),
    ).toMatchObject({ title: '경험', answers: [{ label: '장소', text: '  보존  ' }] })
  })
})

describe('normalizeClipProject against key order', () => {
  const base = {
    title: '클립',
    videoTemplateId: 'template',
    ratio: 'vertical' as const,
    targetDurationMs: 30000,
    disclosure: '' as const,
    cta: '' as const,
  }
  // The server holds these as maps, so the same project can come back naming the same fields in a
  // different order. The form compares serialized drafts to decide whether it is in sync
  // (CLIP-39), so an order-sensitive comparison left an edit 'unsaved' for good.
  it('serializes the same draft identically whatever order the server names its fields in', () => {
    const one = normalizeClipProject({
      ...base,
      answers: [
        { label: '나중', text: 'b' },
        { label: '먼저', text: 'a' },
      ],
      compositionInputs: {
        values: { region: '강릉', place: '가게' },
        items: { menu: [{ id: 'first', values: { price: '9000', name: '갈비' } }] },
        associations: [],
      },
    })
    const other = normalizeClipProject({
      ...base,
      answers: [
        { label: '먼저', text: 'a' },
        { label: '나중', text: 'b' },
      ],
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
      answers: [],
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
