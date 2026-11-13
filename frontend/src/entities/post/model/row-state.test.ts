import { describe, expect, it } from 'vitest'
import i18next from 'i18next'
import type { GenerationJob } from '@/entities/generation-job/@x/post'
import { postRowState } from './row-state'

type RowPost = Parameters<typeof postRowState>[0]

function job(status: string): GenerationJob {
  return {
    id: 'job',
    kind: 'generate_post',
    status,
    stage: 'write',
    progressDone: 0,
    progressTotal: 1,
    failure: undefined,
    postSlug: 'post',
    observeModel: undefined,
    writeModel: undefined,
    createdAt: '2026-11-13T00:00:00Z',
    updatedAt: '2026-11-13T00:00:00Z',
    targetLanguage: undefined,
  }
}
const RUNNING = job('running')
const FAILED = job('failed')

function row(overrides: Partial<RowPost> = {}): RowPost {
  return {
    status: 'draft',
    activeJob: undefined,
    latestOrdinaryFailure: undefined,
    pendingExperimentId: '',
    contentReady: false,
    storylineReady: false,
    ...overrides,
  }
}

// POST-127: a draft holding a storyline and no content reads 스토리라인 작성 완료 as a mid-way
// badge, below every job and test state and only while it is a draft without content.
describe('postRowState', () => {
  it.each([
    ['a storyline-only draft', row({ storylineReady: true }), '스토리라인 작성 완료', 'accent'],
    ['a draft with nothing yet', row(), '초안', 'neutral'],
    [
      'a draft holding a storyline and content',
      row({ storylineReady: true, contentReady: true }),
      '초안',
      'neutral',
    ],
    [
      'a running job over a storyline',
      row({ storylineReady: true, activeJob: RUNNING }),
      'AI 생성 중',
      'info',
    ],
    [
      'an ordinary failure over a storyline',
      row({ storylineReady: true, latestOrdinaryFailure: FAILED }),
      'AI 결과 오류',
      'danger',
    ],
    [
      'a failed active job over a storyline',
      row({ storylineReady: true, activeJob: FAILED }),
      'AI 결과 오류',
      'danger',
    ],
    [
      'a pending test over a storyline',
      row({ storylineReady: true, pendingExperimentId: 'experiment' }),
      'AI 결과 확인',
      'warning',
    ],
    [
      'a review post holding a storyline',
      row({ status: 'review', storylineReady: true, contentReady: true }),
      '검토',
      'accent',
    ],
    [
      'a finalized post holding a storyline',
      row({ status: 'finalized', storylineReady: true, contentReady: true }),
      '확정',
      'success',
    ],
    [
      'a published post holding a storyline',
      row({ status: 'published', storylineReady: true, contentReady: true }),
      '발행됨',
      'done',
    ],
    [
      'a review post whose storyline outlived emptied content',
      row({ status: 'review', storylineReady: true }),
      '검토',
      'accent',
    ],
  ])('reads %s', (_, post, label, tone) => {
    expect(postRowState(post)).toEqual({ label, tone })
  })

  it('keeps a failed or partial pending test above the storyline', () => {
    const post = row({ storylineReady: true, pendingExperimentId: 'experiment' })
    expect(postRowState(post, 'failed').label).toBe('AI 결과 오류')
    expect(postRowState(post, 'partial').label).toBe('AI 결과 오류')
    expect(postRowState(post, 'awaiting_review').label).toBe('AI 결과 확인')
  })

  it('names the storyline-ready state in English as a clip does', async () => {
    await i18next.changeLanguage('en')
    try {
      expect(postRowState(row({ storylineReady: true })).label).toBe('Storyline ready')
    } finally {
      await i18next.changeLanguage('ko')
    }
  })
})
