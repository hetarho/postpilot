import { Code, type Transport } from '@connectrpc/connect'
import { WritingTestService, PostService, ProviderService } from '@/shared/api'
import { createWritingTestStudioFixture } from '@/entities/writing-test/api/studio-fixture'
import { createFakeAuthTransport, type FakeAuthOptions } from '@/test/session'
import { renderAppAt } from '@/test/app'
import { connectAppError } from '@/test/app-error'

/** Real app routes and actors; the ordinary authenticated backend retains legacy paid reads. */
export function renderLegacyModelRoute(
  at: string,
  options: FakeAuthOptions = {},
  fixtureOptions: {
    source?: boolean
    readableTest?: boolean
    readGate?: Promise<void>
    listFails?: boolean
  } = {},
) {
  const fixture = createWritingTestStudioFixture(fixtureOptions)
  fixture.setOwner(options.user?.id ?? 'alice')
  const base = createFakeAuthTransport({ existingSetup: true, ...options })
  const procedures: string[] = []
  const transport: Transport = new Proxy(base, {
    get(target, property) {
      if (property !== 'unary') return Reflect.get(target, property)
      return async (...args: unknown[]) => {
        const method = args[0] as { name: string; parent: { typeName: string } }
        procedures.push(method.name)
        if (method.parent.typeName === WritingTestService.typeName) {
          if (method.name === 'ListWritingTests') {
            if (fixtureOptions.readGate) await fixtureOptions.readGate
            if (fixtureOptions.listFails)
              throw connectAppError('NETWORK_UNAVAILABLE', Code.Unavailable)
          }
          return Reflect.apply(fixture.transport.unary, fixture.transport, args)
        }
        if (
          (fixtureOptions.source && method.parent.typeName === PostService.typeName) ||
          (!options.providers && method.parent.typeName === ProviderService.typeName)
        )
          return Reflect.apply(fixture.transport.unary, fixture.transport, args)
        return Reflect.apply(target.unary, target, args)
      }
    },
  })
  return { ...renderAppAt(at, { transport }), fixture, procedures }
}

export function paidActions(procedures: string[]) {
  return procedures.filter((name) =>
    /^(Start|Analyze|Estimate|Decide|Publish|Apply|Adopt|Save|Create|SetDefault|Reset|Patch|Vote|Retry)/.test(
      name,
    ),
  )
}
