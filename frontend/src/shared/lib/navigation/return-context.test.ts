import {
  createReturnContextStore,
  safeInternalPath,
  validateReturnContext,
  type ReturnContext,
} from './return-context'

const entry: ReturnContext = {
  version: 1,
  ownerKey: 'alice',
  path: '/work?status=failed',
  section: 'work',
  filters: { status: 'failed' },
  scrollY: 321,
  targetId: 'owned',
}
const ports = {
  isAccessible: (path: string, target?: string) => path.startsWith('/work') && target !== 'deleted',
}
describe('safe retained return context', () => {
  it.each([
    'https://evil.test/',
    '//evil.test',
    '\\evil.test',
    '/\\evil',
    '/%5cevil',
    '/%255cevil',
    '/%2f%2fevil.test',
    '/%252f%252fevil.test',
    '/%0aevil',
    '/%',
    'javascript:alert(1)',
  ])('rejects unsafe origin %s', (path) => {
    expect(safeInternalPath(path)).toBe(false)
    expect(validateReturnContext({ ...entry, path }, 'alice', ports)).toBeUndefined()
  })
  it('retains safely encoded search text', () => {
    expect(safeInternalPath('/work?q=coffee%20shop')).toBe(true)
    expect(safeInternalPath('/work?q=50%25')).toBe(true)
    expect(safeInternalPath('/%252f%252fevil.test?q=50%25')).toBe(false)
  })
  it('preserves section, filters, target and scroll without restoring another owner or deleted target', () => {
    expect(validateReturnContext(entry, 'alice', ports)).toEqual(entry)
    expect(validateReturnContext(entry, 'bob', ports)).toBeUndefined()
    expect(validateReturnContext({ ...entry, targetId: 'deleted' }, 'alice', ports)).toBeUndefined()
    expect(validateReturnContext({ ...entry, path: '/missing' }, 'alice', ports)).toBeUndefined()
    expect(validateReturnContext({ ...entry, scrollY: Infinity }, 'alice', ports)).toBeUndefined()
  })
  it('isolates owner storage, survives reload and safely handles corrupt/unavailable storage', () => {
    const values = new Map<string, string>()
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => {
        values.set(key, value)
      },
      removeItem: (key: string) => {
        values.delete(key)
      },
    }
    const alice = createReturnContextStore('alice', storage, ports)
    expect(alice.write(entry)).toBe(true)
    expect(createReturnContextStore('alice', storage, ports).read()).toEqual(entry)
    expect(createReturnContextStore('bob', storage, ports).read()).toBeUndefined()
    values.set('postpilot.return.alice', 'broken')
    expect(alice.read()).toBeUndefined()
    const unavailable = createReturnContextStore(
      'alice',
      {
        getItem() {
          throw Error()
        },
        setItem() {
          throw Error()
        },
        removeItem() {
          throw Error()
        },
      },
      ports,
    )
    expect(unavailable.read()).toBeUndefined()
    expect(unavailable.write(entry)).toBe(false)
    expect(() => unavailable.clear()).not.toThrow()
  })
  it('fails closed when the injected ownership lookup fails', () => {
    expect(
      validateReturnContext(entry, 'alice', {
        isAccessible() {
          throw Error()
        },
      }),
    ).toBeUndefined()
  })
})
