import { useState } from 'react'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ClipSpeechService, SpokenVoiceService, PlanService } from '@/shared/api'
import { applyTimelineEdit, type TimelineEdit } from '@/entities/clip-plan'
import type { ClipProject } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ClipDubbingEditor } from './ClipDubbingEditor'
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to, ...rest }: { children: React.ReactNode; to: string }) => (
    <a href={to} {...rest}>
      {children}
    </a>
  ),
}))
afterEach(() => vi.restoreAllMocks())
function setup(expired = false, fail = false) {
  const state = clipTimelineFixture()
  state.plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      {
        id: 'spoken-1',
        text: '첫 번째 더빙 대본',
        textRevision: 1,
        inputHash: 'text',
        startMs: 0,
        endMs: 3000,
      },
    ],
  }
  let flushed = false,
    quotes = 0
  const starts: unknown[] = []
  const flush = vi.fn(async () => {
    flushed = true
    return 4
  })
  const transport = createRouterTransport((router) => {
    router.rpc(SpokenVoiceService.method.listSpokenVoices, () => ({ voices: [] }))
    router.rpc(SpokenVoiceService.method.listSpokenDrafts, () => ({ drafts: [] }))
    router.rpc(PlanService.method.getMyPlan, () => ({ balance: { unlimited: true } }))
    router.rpc(ClipSpeechService.method.quoteClipSpeech, (req) => {
      expect(flushed).toBe(true)
      expect(req.expectedRevision).toBe(4)
      quotes++
      return {
        quoteId: `quote-${quotes}`,
        maximumCredits: 7,
        expiresAt: new Date(Date.now() + (expired ? -1000 : 60000)).toISOString(),
        segmentIds: ['spoken-1'],
        planRevision: 4,
        cancellationPolicyVersion: 1,
      }
    })
    router.rpc(ClipSpeechService.method.startClipSpeech, (req) => {
      starts.push(req)
      if (fail) throw new ConnectError('offline', Code.Unavailable)
      return { jobId: 'speech-job' }
    })
  })
  const project = {
    id: 'clip',
    targetDurationMs: 30000,
    allowedCaptionStyles: [],
  } as unknown as ClipProject
  function View() {
    const [plan, setPlan] = useState(state.plan)
    const change = (edit: TimelineEdit) => setPlan((p) => applyTimelineEdit(p, edit))
    return (
      <ClipDubbingEditor
        ownerId="alice"
        project={project}
        plan={plan}
        state={state}
        revision={4}
        change={change}
        flush={flush}
        onSelect={() => undefined}
      />
    )
  }
  const Wrapper = withProviders(transport, createTestQueryClient())
  render(<View />, { wrapper: Wrapper })
  return { starts, flush, quotes: () => quotes }
}
describe('explicit speech regeneration approval', () => {
  it('typing, mute and disabling narration neither quote nor synthesize', async () => {
    const h = setup()
    fireEvent.click(screen.getByText(/대본 1 ·/))
    fireEvent.change(screen.getByLabelText('더빙 대본'), { target: { value: '수정 대본' } })
    fireEvent.change(screen.getByLabelText('더빙 음량'), { target: { value: '0' } })
    expect(screen.getByText(/선택 목록에 없습니다/)).toBeInTheDocument()
    fireEvent.click(screen.getByLabelText('더빙 사용'))
    expect(h.quotes()).toBe(0)
    expect(h.starts).toHaveLength(0)
    expect(h.flush).not.toHaveBeenCalled()
  })
  it('flushes before quoting only changed segments and freezes revision, ceiling, policy and retry key', async () => {
    const h = setup()
    expect(h.quotes()).toBe(0)
    fireEvent.click(screen.getByRole('button', { name: '변경 음성 견적 보기' }))
    await screen.findByRole('dialog', { name: '변경 음성 생성 승인' })
    expect(h.quotes()).toBe(1)
    fireEvent.click(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' }))
    await waitFor(() => expect(h.starts).toHaveLength(1))
    expect(h.starts[0]).toMatchObject({
      projectId: 'clip',
      expectedRevision: 4,
      approvedMaxCredits: 7,
      cancellationPolicyVersion: 1,
      quoteId: 'quote-1',
    })
    expect((h.starts[0] as { idempotencyKey: string }).idempotencyKey).toBeTruthy()
  })
  it('expired quotes cannot start speech and require an explicit refresh', async () => {
    const h = setup(true)
    fireEvent.click(screen.getByRole('button', { name: '변경 음성 견적 보기' }))
    await screen.findByText(/견적 유효 시간이 바뀌었습니다/)
    expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).toBeDisabled()
    expect(h.starts).toHaveLength(0)
    fireEvent.click(screen.getByRole('button', { name: '견적 새로 받기' }))
    await waitFor(() => expect(h.quotes()).toBe(2))
    expect(h.starts).toHaveLength(0)
  })
  it('transport failure keeps the same explicit request key on owner retry', async () => {
    const h = setup(false, true)
    fireEvent.click(screen.getByRole('button', { name: '변경 음성 견적 보기' }))
    await screen.findByRole('button', { name: '최대 7 크레딧으로 생성' })
    fireEvent.click(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' }))
    await waitFor(() => expect(h.starts).toHaveLength(1))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).not.toBeDisabled(),
    )
    fireEvent.click(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' }))
    await waitFor(() => expect(h.starts).toHaveLength(2))
    expect(h.starts[1]).toEqual(h.starts[0])
    expect(h.quotes()).toBe(1)
  })
})
