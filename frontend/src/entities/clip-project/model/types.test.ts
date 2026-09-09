import { describe, expect, it } from 'vitest'
import {
  emptyClipProject,
  normalizeClipProject,
  validClipProject,
  type ClipProjectDraft,
} from './types'
const draft = (): ClipProjectDraft => ({
  ...emptyClipProject(),
  title: '경험',
  videoTemplateId: 'owned',
  answers: [{ label: '장소', text: '제주' }],
})
const fields = [{ label: '장소' }]
describe('clip setup validation', () => {
  it.each([
    { title: '' },
    { title: '😀'.repeat(101) },
    { videoTemplateId: '' },
    { targetDurationMs: 14999 },
    { targetDurationMs: 90001 },
    { targetDurationMs: NaN },
    { targetDurationMs: 15000.1 },
    { answers: [] },
    { answers: [{ label: '장소', text: ' ' }] },
    { answers: [{ label: '장소', text: '😀'.repeat(501) }] },
  ])('rejects invalid setup %j', (patch) => {
    expect(validClipProject({ ...draft(), ...patch }, fields)).toBe(false)
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
