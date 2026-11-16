import { Profiler } from 'react'
import { act, fireEvent, render, screen } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ClipSpeechService, SpokenVoiceService, PlanService } from '@/shared/api'
import type { ClipProject } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ClipDubbingEditor } from './ClipDubbingEditor'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
}))
afterEach(() => vi.useRealTimers())

describe('an open speech quote', () => {
  it('re-renders the editor once, at expiry, and not every second', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-11-16T00:00:00Z'))
    let expiresAt = 0
    const commits = vi.fn()
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
    const transport = createRouterTransport((router) => {
      router.rpc(SpokenVoiceService.method.listSpokenVoices, () => ({ voices: [] }))
      router.rpc(SpokenVoiceService.method.listSpokenDrafts, () => ({ drafts: [] }))
      router.rpc(PlanService.method.getMyPlan, () => ({ balance: { unlimited: true } }))
      router.rpc(ClipSpeechService.method.quoteClipSpeech, () => ({
        quoteId: 'quote-1',
        maximumCredits: 7,
        expiresAt: new Date((expiresAt = Date.now() + 60000)).toISOString(),
        segmentIds: ['spoken-1'],
        planRevision: 4,
        cancellationPolicyVersion: 1,
      }))
    })
    const project = {
      id: 'clip',
      targetDurationMs: 30000,
      allowedCaptionStyles: [],
    } as unknown as ClipProject
    render(
      <Profiler id="editor" onRender={commits}>
        <ClipDubbingEditor
          ownerId="alice"
          project={project}
          plan={state.plan}
          state={state}
          revision={4}
          change={() => undefined}
          flush={async () => 4}
          onSelect={() => undefined}
        />
      </Profiler>,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )
    fireEvent.click(screen.getByRole('button', { name: '변경 음성 견적 보기' }))
    // waitFor would poll on the fake clock, so let the quote's request settle by hand.
    for (let i = 0; i < 20 && !expiresAt; i++) {
      await act(async () => {
        await vi.advanceTimersByTimeAsync(0)
      })
    }
    expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).not.toBeDisabled()

    const opened = commits.mock.calls.length
    // Just before the expiry the quote is still valid, and nothing has re-rendered meanwhile.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(expiresAt - 1 - Date.now())
    })
    expect(commits.mock.calls.length).toBe(opened)
    expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).not.toBeDisabled()

    // At expiresAt it turns invalid, in one re-render.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1)
    })
    expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).toBeDisabled()
    expect(screen.getByText(/견적 유효 시간이 바뀌었습니다/)).toBeInTheDocument()
    expect(commits.mock.calls.length).toBe(opened + 1)
  })
})
