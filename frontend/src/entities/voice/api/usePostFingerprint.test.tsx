import { createRouterTransport } from '@connectrpc/connect'
import { renderHook, waitFor } from '@testing-library/react'
import { expect, it } from 'vitest'
import { VoiceService } from '@/shared/api'
import { AUTOSAVE_DEBOUNCE_MS } from '@/shared/config'
import { createTestQueryClient, withProviders } from '@/test/session'
import { usePostFingerprint } from './usePostFingerprint'

// POST-102: ② autosaves a block a beat after each pause, and every save moves the content
// revision. The fingerprint is read for the revision that settles, not once per save.
it('reads once for a burst of autosaves, a beat after the last', async () => {
  const reads: string[] = []
  const transport = createRouterTransport(({ rpc }) => {
    rpc(VoiceService.method.getPostFingerprint, (request) => {
      reads.push(request.postSlug)
      return { applicable: true, revision: BigInt(reads.length), items: [] }
    })
  })
  const view = renderHook(
    ({ revision }: { revision: bigint }) => usePostFingerprint('alice', 'jeju', revision),
    { initialProps: { revision: 1n }, wrapper: withProviders(transport, createTestQueryClient()) },
  )
  // The revision on screen when the row mounts is read at once.
  await waitFor(() => expect(view.result.current.fingerprint?.revision).toBe(1n))
  expect(reads).toHaveLength(1)

  // Three saves land inside one beat.
  for (const revision of [2n, 3n, 4n]) {
    view.rerender({ revision })
    await new Promise((resolve) => setTimeout(resolve, AUTOSAVE_DEBOUNCE_MS / 4))
  }
  expect(reads).toHaveLength(1)
  // The previous reading stays on screen meanwhile.
  expect(view.result.current.fingerprint?.revision).toBe(1n)

  await waitFor(() => expect(reads).toHaveLength(2))
  await waitFor(() => expect(view.result.current.fingerprint?.revision).toBe(2n))
  await new Promise((resolve) => setTimeout(resolve, AUTOSAVE_DEBOUNCE_MS * 1.5))
  expect(reads).toHaveLength(2)
})
