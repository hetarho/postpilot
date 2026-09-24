import { createRouterTransport } from '@connectrpc/connect'
import { act, renderHook } from '@testing-library/react'
import { expect, it } from 'vitest'
import { ProtoBlogField } from '@/shared/api'
import {
  FAKE_GUIDELINE_PRESET_TEXT,
  registerGuidelineService,
  type FakeGuidelinesOptions,
} from '@/test/guidelines'
import { createTestQueryClient, withProviders } from '@/test/session'
import type { Guideline, GuidelinePreset } from '../model/types'
import { useUpdateGuidelinePresetCall } from './guideline-mutations'
import { guidelinesQueryKey } from './guideline-queries'

// GUIDE-38: the answer IS the preset, so it is in the list entry the moment the save lands — the
// switch never shows the old state while the refetch is out — and each half goes on its own.
it('writes the answered preset into the list and sends each half alone', async () => {
  const presetUpdates: NonNullable<FakeGuidelinesOptions['presetUpdates']> = []
  const transport = createRouterTransport((router) =>
    registerGuidelineService(router, { presetUpdates }),
  )
  const queryClient = createTestQueryClient()
  const key = guidelinesQueryKey(transport, 'alice')
  queryClient.setQueryData<{ guidelines: Guideline[]; preset: GuidelinePreset }>(key, {
    guidelines: [],
    preset: { text: FAKE_GUIDELINE_PRESET_TEXT, enabled: false, fields: [] },
  })
  const { result } = renderHook(() => useUpdateGuidelinePresetCall('alice'), {
    wrapper: withProviders(transport, queryClient),
  })
  const preset = () =>
    queryClient.getQueryData<{ guidelines: Guideline[]; preset: GuidelinePreset }>(key)?.preset

  await act(async () => {
    await result.current.setEnabled(true)
  })
  expect(preset()).toEqual({ text: FAKE_GUIDELINE_PRESET_TEXT, enabled: true, fields: [] })
  expect(queryClient.getQueryState(key)?.isInvalidated).toBe(true)

  await act(async () => {
    await result.current.setFields(['restaurant', 'cafe'])
  })
  expect(preset()).toEqual({
    text: FAKE_GUIDELINE_PRESET_TEXT,
    enabled: true,
    fields: ['restaurant', 'cafe'],
  })
  expect(presetUpdates).toEqual([
    { enabled: true, fields: undefined },
    { enabled: undefined, fields: [ProtoBlogField.RESTAURANT, ProtoBlogField.CAFE] },
  ])
})
