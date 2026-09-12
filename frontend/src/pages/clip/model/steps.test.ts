import { describe, expect, it } from 'vitest'
import type { ClipProject } from '@/entities/clip-project'
import type { GenerationJob } from '@/entities/generation-job'
import { stepForProject } from './steps'

const failedJob = (kind: string, stage: string): GenerationJob => ({
  id: 'job',
  kind,
  status: 'failed',
  stage,
  progressDone: 0,
  progressTotal: 0,
  failure: undefined,
  postSlug: '',
  clipProjectId: 'one',
  observeModel: undefined,
  writeModel: undefined,
  createdAt: '2026-09-11',
  updatedAt: '2026-09-11',
  targetLanguage: undefined,
})

const base: ClipProject = {
  id: 'one',
  title: '여행',
  videoTemplateId: 'tpl',
  ratio: 'vertical',
  targetDurationMs: 30000,
  disclosure: 'ad',
  cta: '',
  answers: [],
  createdAt: '2026-09-11',
  updatedAt: '2026-09-11',
  editPlanRevision: 0,
  renderedPlanRevision: 0,
}
const result = { contentType: 'video/mp4', bytes: 1, durationMs: 1000, createdAt: '2026-09-11' }

describe('stepForProject', () => {
  it('opens a project with no plan on 클립 생성', () => {
    expect(stepForProject(base)).toBe('generate')
  })

  it('opens a project whose plan is newer than its render on 클립 다듬기', () => {
    expect(stepForProject({ ...base, editPlanRevision: 2, renderedPlanRevision: 1, result })).toBe(
      'refine',
    )
  })

  it('sends a failed generation back to the step that re-approves it', () => {
    expect(
      stepForProject({
        ...base,
        editPlanRevision: 2,
        renderedPlanRevision: 2,
        result,
        latestJob: failedJob('generate_clip', 'prepare'),
      }),
    ).toBe('generate')
  })

  it('sends a failed rerender back to the correction that re-runs it', () => {
    expect(
      stepForProject({
        ...base,
        editPlanRevision: 3,
        renderedPlanRevision: 2,
        result,
        latestJob: failedJob('render_clip', 'render'),
      }),
    ).toBe('refine')
  })

  it('keeps a rendered unfinalized project on 클립 다듬기', () => {
    expect(stepForProject({ ...base, editPlanRevision: 2, renderedPlanRevision: 2, result })).toBe(
      'refine',
    )
  })
})
