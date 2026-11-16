import { act, fireEvent, render, screen } from '@testing-library/react'
import { createRouterTransport } from '@connectrpc/connect'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ClipSpeechService, SpokenVoiceService, PlanService } from '@/shared/api'
import type { ClipProject } from '@/entities/clip-project'
import { clipTimelineFixture } from '@/test/clip-editing'
import { createTestQueryClient, withProviders } from '@/test/session'
import { ClipDubbingEditor } from './ClipDubbingEditor'

const keyOf = vi.hoisted(() => vi.fn())
vi.mock('@/entities/clip-plan', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/entities/clip-plan')>()
  return {
    ...actual,
    clipDraftKey: (plan: Parameters<typeof actual.clipDraftKey>[0]) => {
      keyOf()
      return actual.clipDraftKey(plan)
    },
  }
})
vi.mock('@tanstack/react-router', () => ({
  Link: ({ children, to }: { children: React.ReactNode; to: string }) => (
    <a href={to}>{children}</a>
  ),
}))
afterEach(() => vi.useRealTimers())

describe('an open speech quote', () => {
  it('does not re-render the editor until it expires, then turns invalid', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
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
        expiresAt: new Date(Date.now() + 60000).toISOString(),
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
      <ClipDubbingEditor
        ownerId="alice"
        project={project}
        plan={state.plan}
        state={state}
        revision={4}
        change={() => undefined}
        flush={async () => 4}
        onSelect={() => undefined}
      />,
      { wrapper: withProviders(transport, createTestQueryClient()) },
    )
    fireEvent.click(screen.getByRole('button', { name: '변경 음성 견적 보기' }))
    const approve = await screen.findByRole('button', { name: '최대 7 크레딧으로 생성' })
    expect(approve).not.toBeDisabled()

    const opened = keyOf.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000)
    })
    expect(keyOf.mock.calls.length).toBe(opened)
    expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).not.toBeDisabled()

    await act(async () => {
      await vi.advanceTimersByTimeAsync(31000)
    })
    expect(screen.getByRole('button', { name: '최대 7 크레딧으로 생성' })).toBeDisabled()
    expect(screen.getByText(/견적 유효 시간이 바뀌었습니다/)).toBeInTheDocument()
  })
})
