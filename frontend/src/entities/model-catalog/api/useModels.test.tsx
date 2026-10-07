import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { render, screen, waitFor, cleanup } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'
import { useRef } from 'react'
import { ProviderService, PlanService, Stage } from '@/shared/api'
import { createTestQueryClient, withProviders } from '@/test/session'
import { myPlanQueryKey } from '@/entities/plan/@x/model-catalog'
import { useModels } from './useModels'
afterEach(cleanup)
it('reprojects a changed plan without synchronously updating the parent from a mounting child', async () => {
  let reads = 0
  const transport = createRouterTransport(({ rpc }) => {
    rpc(ProviderService.method.listModels, () => {
      reads++
      return create(ProviderService.method.listModels.output, {
        models: [
          {
            ref: { providerId: 'p', modelId: 'writer' },
            label: 'Writer',
            stages: [Stage.WRITE],
            affordable: true,
          },
        ],
      })
    })
  })
  const cache = createTestQueryClient(),
    key = myPlanQueryKey(transport)
  cache.setQueryData(key, create(PlanService.method.getMyPlan.output, {}), { updatedAt: 1 })
  const warning = vi.spyOn(console, 'error').mockImplementation(() => {})
  function Child() {
    const written = useRef(false)
    if (!written.current) {
      written.current = true
      cache.setQueryData(key, create(PlanService.method.getMyPlan.output, {}))
    }
    return null
  }
  function Parent({ mount }: { mount: boolean }) {
    const models = useModels()
    return (
      <>
        {models.models.map((model) => (
          <span key={model.ref.modelId}>{model.label}</span>
        ))}
        {mount && <Child />}
      </>
    )
  }
  const view = render(<Parent mount={false} />, { wrapper: withProviders(transport, cache) })
  await screen.findByText('Writer')
  expect(reads).toBe(1)
  view.rerender(<Parent mount />)
  await waitFor(() => expect(reads).toBe(2))
  expect(
    warning.mock.calls.some((args) => String(args[0]).includes('Cannot update a component')),
  ).toBe(false)
  warning.mockRestore()
})
