import { create } from '@bufbuild/protobuf'
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect'
import { QueryClient } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { AdminListSpeechProfilesResponseSchema, SpeechProfileService } from '@/shared/api'
import { withProviders } from '@/test/session'
import { useAdminSpeechProfiles } from './useSpeechProfiles'

it('keeps a failed saved-list read unknown and recovers through metadata refresh', async () => {
  const requests: boolean[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(SpeechProfileService.method.adminListSpeechProfiles, (request) => {
      requests.push(request.refresh)
      if (!request.refresh) throw new ConnectError('private server details', Code.Unavailable)
      return create(AdminListSpeechProfilesResponseSchema, {
        fetchError: 'SPEECH_API_KEY_NOT_CONFIGURED',
      })
    })
  })
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = renderHook(useAdminSpeechProfiles, { wrapper: withProviders(transport, cache) })
  await waitFor(() => expect(view.result.current.isError).toBe(true))
  expect(view.result.current.hasData).toBe(false)
  expect(view.result.current.failure).toBeDefined()
  expect(JSON.stringify(view.result.current.failure)).not.toContain('private server details')
  act(() => view.result.current.refresh())
  await waitFor(() => expect(view.result.current.hasData).toBe(true))
  expect(view.result.current.isError).toBe(false)
  expect(view.result.current.failure).toBeUndefined()
  expect(view.result.current.browse.fetchError).toBe('SPEECH_API_KEY_NOT_CONFIGURED')
  expect(requests).toEqual([false, true])
})

it('retains saved profiles on a failed refresh and clears the failure after a successful retry', async () => {
  let failRefresh = true
  const transport = createRouterTransport(({ rpc }) => {
    rpc(SpeechProfileService.method.adminListSpeechProfiles, (request) => {
      if (request.refresh && failRefresh) throw new ConnectError('down', Code.Unavailable)
      return create(AdminListSpeechProfilesResponseSchema, {
        profiles: [{ id: 'saved', label: 'Saved Korean voice', revision: 1n }],
      })
    })
  })
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = renderHook(useAdminSpeechProfiles, { wrapper: withProviders(transport, cache) })
  await waitFor(() => expect(view.result.current.hasData).toBe(true))
  act(() => view.result.current.refresh())
  await waitFor(() => expect(view.result.current.isError).toBe(true))
  expect(view.result.current.browse.profiles[0].id).toBe('saved')
  expect(view.result.current.failure).toBeDefined()
  failRefresh = false
  act(() => view.result.current.refresh())
  await waitFor(() => expect(view.result.current.isError).toBe(false))
  expect(view.result.current.failure).toBeUndefined()
  expect(view.result.current.browse.profiles[0].id).toBe('saved')
})

it('sends lean registrations and invalidates both speech projections after common pricing changes', async () => {
  const registrations: unknown[] = []
  const tariffs: unknown[] = []
  let reads = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(SpeechProfileService.method.adminListSpeechProfiles, () => {
      reads++
      return create(AdminListSpeechProfilesResponseSchema, {
        combinations: [
          {
            label: 'Design → Synth',
            binding: {
              designModel: { providerId: 'speech', modelId: 'design' },
              speechModel: { providerId: 'speech', modelId: 'synth' },
            },
          },
        ],
        tariff: {
          revision: 2n,
          designUsdPerUnit: '0.000100001',
          speechUsdPerUnit: '0.000000001',
          confirmationUsd: '0',
          complete: true,
        },
      })
    })
    rpc(SpeechProfileService.method.registerSpeechCombination, (request) => {
      registrations.push(request)
      return { profile: { id: 'saved', revision: 1n, enabled: true } }
    })
    rpc(SpeechProfileService.method.saveSpeechTariff, (request) => {
      tariffs.push(request)
      if (!request.tariff) throw new Error('Tariff is required')
      return { tariff: { ...request.tariff, revision: 3n } }
    })
  })
  const cache = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const view = renderHook(useAdminSpeechProfiles, { wrapper: withProviders(transport, cache) })
  await waitFor(() => expect(view.result.current.hasData).toBe(true))
  expect(view.result.current.browse.combinations[0].label).toBe('Design → Synth')
  await act(() =>
    view.result.current.save({
      profileId: '',
      expectedRevision: 0n,
      designModel: { providerId: 'speech', modelId: 'design' },
      speechModel: { providerId: 'speech', modelId: 'synth' },
      enabled: true,
      grade: '',
    }),
  )
  expect(registrations).toHaveLength(1)
  expect(registrations[0]).not.toHaveProperty('prices')
  expect(registrations[0]).not.toHaveProperty('binding')
  await act(() =>
    view.result.current.saveTariff({
      ...view.result.current.browse.tariff!,
      source: 'https://example.com/account',
      checkedAt: '',
    }),
  )
  expect(tariffs).toEqual([
    expect.objectContaining({
      expectedRevision: 2n,
      tariff: expect.objectContaining({ speechUsdPerUnit: '0.000000001', confirmationUsd: '0' }),
    }),
  ])
  expect(reads).toBe(3)
})
