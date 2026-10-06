import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
const route = '/voices/voice-default/materials'
const personal = [{ id: 'voice-default', name: '내 말투', made: false }]
const writing = '직접 쓴 글이에요.\n'.repeat(60)
const providers = {
  models: [{ providerId: 'stub', modelId: 'analyze', label: '추천 분석', stages: [Stage.ANALYZE] }],
  selections: [{ stage: Stage.ANALYZE, providerId: 'stub', modelId: 'analyze' }],
}
describe('personal learning transport flow', () => {
  it('offers three purposeful methods without an unavailable analysis or a create/start call', async () => {
    const calls: string[] = []
    renderAppAt(route, {
      user: { id: 'alice' },
      existingSetup: true,
      calls,
      voice: { voices: personal },
      providers,
    })
    expect(await screen.findByRole('button', { name: '질문 10개로 내 말투 찾기' })).toBeVisible()
    expect(screen.getByRole('button', { name: '내가 쓴 글로 말투 알려 주기' })).toBeVisible()
    expect(screen.getByRole('button', { name: 'AI가 추천한 말투에서 고르기' })).toBeVisible()
    expect(screen.queryByRole('button', { name: /말투 만들기/ })).toBeNull()
    expect(calls.filter((call) => /^(Create|Start|Analyze|SetDefault)/.test(call))).toEqual([])
  })
  it('preserves a pasted draft across method Back and saves it before explicit analysis/default use', async () => {
    const user = userEvent.setup(),
      calls: string[] = []
    const analyses: Array<{ voiceId: string; model: string }> = []
    const { router } = renderAppAt(route, {
      user: { id: 'alice' },
      existingSetup: true,
      calls,
      voice: { voices: personal, analyses, analysisAfterAnalysis: '차분한 문장으로 이야기해요.' },
      jobs: {
        jobs: [{ id: 'voice-job', kind: 'analyze_voice', status: 'done', stage: 'analyze' }],
      },
      providers,
    })
    await user.click(await screen.findByRole('button', { name: '내가 쓴 글로 말투 알려 주기' }))
    await user.type(await screen.findByLabelText('제목 (선택)'), '내 글')
    await user.click(screen.getByLabelText('내가 쓴 글'))
    await user.paste(writing)
    expect(screen.queryByLabelText('답')).toBeNull()
    await user.click(screen.getByRole('button', { name: '다른 방법으로 알려 주기' }))
    await user.click(screen.getByRole('button', { name: '내가 쓴 글로 말투 알려 주기' }))
    expect(await screen.findByLabelText('내가 쓴 글')).toHaveValue(writing)
    await user.click(screen.getByRole('button', { name: '추가' }))
    await screen.findByRole('heading', { name: '말투를 알려 줄 자료가 모였어요' })
    expect(calls.filter((call) => call === 'AddVoiceSample')).toHaveLength(1)
    expect(analyses).toEqual([])
    expect(calls).not.toContain('SetDefaultVoice')
    await user.dblClick(await screen.findByRole('button', { name: '이 자료로 내 말투 만들기' }))
    await screen.findByRole('heading', { name: '내 말투가 준비됐어요' })
    expect(analyses).toEqual([{ voiceId: 'voice-default', model: 'stub/analyze' }])
    expect(calls).not.toContain('SetDefaultVoice')
    await user.dblClick(screen.getByRole('button', { name: '이 말투를 내 글에 사용하기' }))
    await waitFor(() => expect(calls.filter((call) => call === 'SetDefaultVoice')).toHaveLength(1))
    await waitFor(() => expect(router.state.location.pathname).toBe('/voices/voice-default'))
  })
  it('keeps the question actor and unsaved answer through Back without collecting a pasted post', async () => {
    const user = userEvent.setup(),
      calls: string[] = []
    renderAppAt(route, {
      user: { id: 'alice' },
      existingSetup: true,
      calls,
      voice: { voices: personal },
      providers,
    })
    await user.click(await screen.findByRole('button', { name: '질문 10개로 내 말투 찾기' }))
    await user.type(await screen.findByLabelText('답'), '아직 저장하지 않은 내 표현이에요.')
    await user.click(screen.getByRole('button', { name: '다른 방법으로 알려 주기' }))
    await user.click(screen.getByRole('button', { name: '질문 10개로 내 말투 찾기' }))
    expect(await screen.findByLabelText('답')).toHaveValue('아직 저장하지 않은 내 표현이에요.')
    expect(screen.queryByLabelText('내가 쓴 글')).toBeNull()
    expect(
      calls.filter((call) =>
        /^(AnswerVoicePrompt|AddVoiceSample|AnalyzeVoice|SetDefaultVoice|CreateVoice)$/.test(call),
      ),
    ).toEqual([])
  })
})
