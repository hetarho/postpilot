import { act, renderHook } from '@testing-library/react'
import { createRouterTransport, Code, ConnectError } from '@connectrpc/connect'
import { expect, it } from 'vitest'
import { ClipService } from '@/shared/api'
import { type GenerationJob } from '@/entities/generation-job'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useCancelClip } from './useCancelClip'

it.each(['running', 'done', 'cancelled'])(
  'reconciles a lost cancel reply with %s and never replays an acknowledged cancel',
  async (status) => {
    let offline = true,
      requests = 0
    const job: GenerationJob = {
      id: 'job',
      kind: 'generate_clip',
      status: 'running',
      stage: 'analyze',
      progressDone: 0,
      progressTotal: 1,
      postSlug: '',
      failure: undefined,
      observeModel: undefined,
      writeModel: undefined,
      createdAt: '',
      updatedAt: '',
      targetLanguage: undefined,
      canCancel: true,
      cancellationPolicyVersion: 1,
    }
    const transport = createRouterTransport((router) => {
      router.rpc(ClipService.method.cancelClipJob, () => {
        requests++
        throw new ConnectError('lost', Code.Unavailable)
      })
      router.rpc(ClipService.method.getClipProject, () => {
        if (offline) throw new ConnectError('offline', Code.Unavailable)
        return {
          project: {
            id: 'clip',
            ratio: 'vertical',
            latestJob: {
              id: 'job',
              kind: 'generate_clip',
              status,
              canCancel: false,
              cancelRequestedAt: status === 'done' ? '' : '2026-09-13T00:00:00Z',
            },
          },
        }
      })
    })
    const view = renderHook(() => useCancelClip('alice', 'clip', job), {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    await act(() => view.result.current.cancel())
    expect(view.result.current.uncertain).toBe(true)
    await act(() => view.result.current.cancel())
    expect(requests).toBe(1)
    offline = false
    await act(() => view.result.current.checkAgain())
    expect(view.result.current.uncertain).toBe(false)
    await act(() => view.result.current.cancel())
    expect(requests).toBe(1)
  },
)
