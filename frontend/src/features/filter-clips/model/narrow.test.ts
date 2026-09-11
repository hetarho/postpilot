import { describe, expect, it } from 'vitest'
import type { ClipProject } from '@/entities/clip-project'
import { narrowClips } from './narrow'

const result = {
  contentType: 'video/mp4',
  bytes: 1,
  durationMs: 15000,
  createdAt: '2026-09-10T00:00:00Z',
}
const project = (over: Partial<ClipProject> & { id: string; title: string }): ClipProject => ({
  videoTemplateId: 'template',
  ratio: 'vertical',
  targetDurationMs: 15000,
  answers: [],
  createdAt: '2026-09-10T00:00:00Z',
  updatedAt: '2026-09-10T00:00:00Z',
  editPlanRevision: 0,
  renderedPlanRevision: 0,
  ...over,
})
const draft = project({ id: 'a', title: '제주 여행' })
const refining = project({
  id: 'b',
  title: '서울 카페 기록',
  editPlanRevision: 2,
  renderedPlanRevision: 1,
  result,
})
const finished = project({
  id: 'c',
  title: 'JEJU 다시',
  editPlanRevision: 1,
  renderedPlanRevision: 1,
  result,
})
const all = [draft, refining, finished]
const ids = (values: ClipProject[]) => values.map((v) => v.id)

describe('narrowClips', () => {
  it('keeps everything when nothing is narrowing', () => {
    expect(ids(narrowClips(all, {}))).toEqual(['a', 'b', 'c'])
  })

  it('matches the title case-insensitively and ignores surrounding space', () => {
    expect(ids(narrowClips(all, { q: '  jeju ' }))).toEqual(['c'])
    expect(ids(narrowClips(all, { q: '카페' }))).toEqual(['b'])
  })

  it('collapses runs of whitespace inside the query', () => {
    expect(ids(narrowClips(all, { q: '제주   여행' }))).toEqual(['a'])
  })

  it("matches the project's own state, not the badge's", () => {
    expect(ids(narrowClips(all, { status: 'draft' }))).toEqual(['a'])
    expect(ids(narrowClips(all, { status: 'refining' }))).toEqual(['b'])
    expect(ids(narrowClips(all, { status: 'finished' }))).toEqual(['c'])
  })

  it('applies both at once', () => {
    expect(ids(narrowClips(all, { q: '다시', status: 'finished' }))).toEqual(['c'])
    expect(ids(narrowClips(all, { q: '다시', status: 'draft' }))).toEqual([])
  })

  it('keeps nothing a query cannot reach', () => {
    expect(ids(narrowClips(all, { q: '부산' }))).toEqual([])
  })
})
