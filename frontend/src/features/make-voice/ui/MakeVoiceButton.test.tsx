import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { useVoiceProfile } from '@/entities/voice'
import { Stage } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { MakeVoiceButton } from './MakeVoiceButton'

it('keeps reanalysis explicit after a server quote and Cancel; a confirmed click starts the frozen model exactly once', async () => {
  const user = userEvent.setup(),
    calls: string[] = [],
    analyses: Array<{ voiceId: string; model: string }> = [],
    onStarted = vi.fn()
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    providers: {
      models: [{ providerId: 'stub', modelId: 'analyze', label: '분석 AI' }],
      selections: [{ stage: Stage.ANALYZE, providerId: 'stub', modelId: 'analyze' }],
    },
    voice: {
      samples: [{ id: 'source', label: '자료', body: '내가 쓴 글이에요.\n'.repeat(60) }],
      analyses,
      analysisEstimate: { credits: 7 },
      analysis: {
        sourceVersionsKnown: true,
        acceptedSources: [{ sampleId: 'source', contentRevision: 1n }],
        ai: { impression: '기존 말투' },
      },
    },
  })
  function Host() {
    const { profile } = useVoiceProfile('alice', 'voice-default')
    return (
      profile && (
        <>
          <p>{profile.analysis?.ai.impression}</p>
          <MakeVoiceButton
            ownerId="alice"
            voiceId="voice-default"
            profile={profile}
            onStarted={onStarted}
          />
        </>
      )
    )
  }
  render(<Host />, { wrapper: withProviders(transport, createTestQueryClient()) })
  const again = await screen.findByRole('button', { name: '다시 분석' })
  await waitFor(() => expect(again).toBeEnabled())
  await user.click(again)
  const first = await screen.findByRole('dialog')
  expect(first).toHaveTextContent('예상 7 크레딧')
  expect(first).toHaveTextContent('stub/analyze')
  expect(analyses).toEqual([])
  await user.click(within(first).getByRole('button', { name: '취소' }))
  expect(analyses).toEqual([])
  await user.click(again)
  await user.dblClick(
    within(await screen.findByRole('dialog')).getByRole('button', { name: '다시 분석 시작' }),
  )
  await waitFor(() => expect(onStarted).toHaveBeenCalledExactlyOnceWith('voice-job'))
  expect(analyses).toEqual([{ voiceId: 'voice-default', model: 'stub/analyze' }])
  expect(screen.getByText('기존 말투')).toBeVisible()
  expect(calls.filter((call) => call === 'EstimateVoiceAnalysis')).toHaveLength(2)
  expect(calls).not.toContain('SetDefaultVoice')
})

describe('analysis estimate lifetimes', () => {
  it('does not expose an old owner quote after the owner component is replaced', async () => {
    let release!: () => void
    const pending = new Promise<void>((resolve) => {
      release = resolve
    })
    const base = createFakeAuthTransport({
      user: { id: 'alice' },
      providers: {
        models: [{ providerId: 'stub', modelId: 'analyze', label: '분석 AI' }],
        selections: [{ stage: Stage.ANALYZE, providerId: 'stub', modelId: 'analyze' }],
      },
      voice: { samples: [{ id: 'source', label: '자료', body: '내 글이에요.\n'.repeat(60) }] },
    })
    const transport = new Proxy(base, {
      get(target, property) {
        if (property !== 'unary') return Reflect.get(target, property)
        return async (...args: unknown[]) => {
          if ((args[0] as { name: string }).name === 'EstimateVoiceAnalysis') await pending
          return Reflect.apply(target.unary, target, args)
        }
      },
    })
    const onStarted = vi.fn()
    function Host({ ownerId }: { ownerId: string }) {
      const { profile } = useVoiceProfile(ownerId, 'voice-default')
      return (
        profile && (
          <MakeVoiceButton
            ownerId={ownerId}
            voiceId="voice-default"
            profile={profile}
            onStarted={onStarted}
          />
        )
      )
    }
    const user = userEvent.setup()
    const view = render(<Host ownerId="alice" />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    const again = await screen.findByRole('button', { name: '다시 분석' })
    await waitFor(() => expect(again).toBeEnabled())
    await user.click(again)
    view.rerender(<Host ownerId="bob" />)
    release()
    await waitFor(() => expect(screen.getByRole('button', { name: '다시 분석' })).toBeEnabled())
    expect(screen.queryByRole('dialog')).toBeNull()
    expect(onStarted).not.toHaveBeenCalled()
  })
})
