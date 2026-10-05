import { webcrypto } from 'node:crypto'
import { createRouterTransport } from '@connectrpc/connect'
import { renderHook } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { ClipSpeechService } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import type { ClipSpeechRef } from '../model/spoken'
import { useClipSpeechCalls } from './speech'

afterEach(() => vi.unstubAllGlobals())
it('refreshes expired access once, verifies immutable bytes and invokes no generation RPC', async () => {
  const bytes = new Uint8Array([1, 2, 3, 4])
  const hash = Buffer.from(await webcrypto.subtle.digest('SHA-256', bytes)).toString('hex')
  vi.stubGlobal('crypto', webcrypto)
  let reads = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ClipSpeechService.method.getClipSpeechAccess, (request) => {
      expect(request).toMatchObject({ projectId: 'clip', assetId: 'asset' })
      reads++
      return { url: '/clip/speech/' + String(reads).repeat(32), audioHash: hash, bytes: 4n }
    })
  })
  const fetch = vi
    .fn()
    .mockResolvedValueOnce(new Response(null, { status: 404 }))
    .mockResolvedValueOnce(new Response(bytes, { headers: { 'content-type': 'audio/mpeg' } }))
  vi.stubGlobal('fetch', fetch)
  const hook = renderHook(() => useClipSpeechCalls(), {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  const ref = { assetId: 'asset', audioHash: hash } as ClipSpeechRef
  expect(
    new Uint8Array(await hook.result.current.load('clip', ref, new AbortController().signal)),
  ).toEqual(bytes)
  expect(reads).toBe(2)
  expect(fetch).toHaveBeenCalledTimes(2)
  expect(fetch.mock.calls[1]![1]).toMatchObject({ credentials: 'include', cache: 'no-store' })
  fetch.mockResolvedValueOnce(
    new Response(new Uint8Array([0, 0, 0, 0]), { headers: { 'content-type': 'audio/mpeg' } }),
  )
  await expect(hook.result.current.load('clip', ref, new AbortController().signal)).rejects.toThrow(
    'provenance',
  )
  fetch.mockResolvedValueOnce(
    new Response(new Uint8Array(5), { headers: { 'content-type': 'audio/mpeg' } }),
  )
  await expect(hook.result.current.load('clip', ref, new AbortController().signal)).rejects.toThrow(
    'size',
  )
  hook.unmount()
})
