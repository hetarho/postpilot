import { act, cleanup, screen, waitFor, within } from '@testing-library/react'
import { Code, ConnectError, type Transport } from '@connectrpc/connect'
import { afterEach, beforeEach, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { PostService, ProviderService, Stage, WritingTestService } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { paidActions } from '@/test/legacy-model-routes'
import { createFakeAuthTransport } from '@/test/session'

beforeEach(() => {
  sessionStorage.clear()
  initializeI18n('ko')
})
afterEach(() => {
  cleanup()
  sessionStorage.clear()
})

it.each([Code.NotFound, Code.PermissionDenied])(
  'removes a cached source return after its owner read fails with code %s',
  async (failureCode) => {
    const fixture = createWritingTestStudioFixture({ source: true })
    const base = createFakeAuthTransport({
      existingSetup: true,
      user: { id: 'alice' },
      experiments: { history: [{ id: 'retained', stage: Stage.WRITE }] },
    })
    const procedures: string[] = []
    let inaccessible = false
    const transport: Transport = new Proxy(base, {
      get(target, property) {
        if (property !== 'unary') return Reflect.get(target, property)
        return async (...args: unknown[]) => {
          const method = args[0] as { name: string; parent: { typeName: string } }
          procedures.push(method.name)
          if (method.parent.typeName === PostService.typeName) {
            if (inaccessible && method.name === 'GetPost')
              throw new ConnectError('Source is no longer accessible', failureCode)
            return Reflect.apply(fixture.transport.unary, fixture.transport, args)
          }
          if (
            method.parent.typeName === ProviderService.typeName ||
            method.parent.typeName === WritingTestService.typeName
          )
            return Reflect.apply(fixture.transport.unary, fixture.transport, args)
          return Reflect.apply(target.unary, target, args)
        }
      },
    })
    const view = renderAppAt('/tests/records/retained?source=owned-source', { transport })
    await screen.findByRole('heading', { name: '이전 유료 비교 기록', level: 1 })
    const initialReturn = await within(screen.getByRole('main')).findByRole('link', {
      name: '글 작성',
    })
    expect(initialReturn).toHaveAttribute('href', '/posts/owned-source')
    const sourceReadsBeforeFailure = procedures.filter((name) => name === 'GetPost').length

    inaccessible = true
    await act(() =>
      view.queryClient.invalidateQueries({
        predicate: (query) => query.queryKey.includes('writing-test-source'),
      }),
    )
    await waitFor(() => {
      const sourceQuery = view.queryClient
        .getQueryCache()
        .findAll({ predicate: (query) => query.queryKey.includes('writing-test-source') })
        .find((query) => query.state.data !== undefined)
      // A failed refetch retains the previous owner data. It must no longer prove a return.
      expect(sourceQuery?.state.status).toBe('error')
      expect(sourceQuery?.state.data).toBeDefined()
      expect(screen.queryByRole('link', { name: '글 작성' })).not.toBeInTheDocument()
      expect(
        within(screen.getByRole('main')).getByRole('link', { name: '테스트 기록' }),
      ).toHaveAttribute('href', '/tests/history')
    })
    expect(procedures.filter((name) => name === 'GetPost').length).toBeGreaterThan(
      sourceReadsBeforeFailure,
    )
    expect(paidActions(procedures)).toEqual([])
  },
)
