import { createRouterTransport } from '@connectrpc/connect'
import { act, renderHook } from '@testing-library/react'
import { expect, it } from 'vitest'
import { GenerationService } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { useStartObservation } from './useStartObservation'

function observationBackend() {
  const requests: { postSlug: string; files?: string[]; observeModel?: string }[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(GenerationService.method.startObservation, (request) => {
      requests.push({
        postSlug: request.postSlug,
        files: request.reobserve ? [...request.reobserve.files] : undefined,
        observeModel: request.observeModel
          ? `${request.observeModel.providerId}/${request.observeModel.modelId}`
          : undefined,
      })
      return { jobId: 'observe-job' }
    })
  })
  return { requests, transport }
}

// GEN-9: the selection keeps its presence on the wire — 관찰하기 sends an empty selection, the
// picker its answer, and an absent one observes every attachment.
it('starts an observation job carrying the selection with its presence', async () => {
  const { requests, transport } = observationBackend()
  const view = renderHook(() => useStartObservation(), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  const model = { providerId: 'p', modelId: 'eye' }

  let answer: { jobId: string } | undefined
  await act(async () => {
    answer = await view.result.current.start('post-a', model, [])
    await view.result.current.start('post-a', model, ['IMG_2.jpg'])
    await view.result.current.start('post-a', undefined)
  })

  expect(answer?.jobId).toBe('observe-job')
  expect(requests).toEqual([
    { postSlug: 'post-a', files: [], observeModel: 'p/eye' },
    { postSlug: 'post-a', files: ['IMG_2.jpg'], observeModel: 'p/eye' },
    { postSlug: 'post-a', files: undefined, observeModel: undefined },
  ])
})
