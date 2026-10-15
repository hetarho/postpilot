import { createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { renderHook, act } from '@testing-library/react'
import { expect, it } from 'vitest'
import { ClipGenerationService, type ProtoAdvertiserGuide } from '@/shared/api'
import { withProviders } from '@/test/session'
import { useClipProjectMutations } from './clip-project'

it('preserves omitted campaign fields, explicit false and an explicit empty guide', async () => {
  const seen: { campaign?: boolean; advertiserGuide?: ProtoAdvertiserGuide }[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipGenerationService.method.updateClipProject, (request) => {
      seen.push(request)
      return { project: { id: 'project', title: request.title, ratio: 'vertical', language: 1 } }
    })
    rpc(ClipGenerationService.method.createClipProject, (request) => {
      seen.push(request)
      return { project: { id: 'new', title: request.title, ratio: 'vertical', language: 1 } }
    })
  })
  const view = renderHook(() => useClipProjectMutations('alice'), {
    wrapper: withProviders(transport, new QueryClient()),
  })
  const draft = {
    title: 'clip',
    videoTemplateId: '',
    ratio: 'vertical' as const,
    targetDurationMs: 15000,
    disclosure: '' as const,
  }
  await act(() => view.result.current.save.mutateAsync({ id: 'project', draft }))
  expect(seen[0]?.campaign).toBeUndefined()
  expect(seen[0]?.advertiserGuide).toBeUndefined()
  await act(() =>
    view.result.current.save.mutateAsync({
      id: 'project',
      draft: {
        ...draft,
        campaign: false,
        advertiserGuide: { text: '', keywords: [], hashtags: [] },
      },
    }),
  )
  expect(seen[1]?.campaign).toBe(false)
  expect(seen[1]?.advertiserGuide).toMatchObject({ text: '', keywords: [], hashtags: [] })
  await act(() => view.result.current.save.mutateAsync({ draft }))
  expect(seen[2]?.campaign).toBe(true)
  await act(() => view.result.current.save.mutateAsync({ draft: { ...draft, campaign: false } }))
  expect(seen[3]?.campaign).toBe(false)
})
