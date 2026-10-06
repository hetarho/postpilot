import { beforeEach, expect, it } from 'vitest'
import { emptySetupProgress } from './setup-machine'
import {
  readSetupProgress,
  writeSetupProgress,
  setupProgressKey,
  type SetupStorage,
  resetSetupProgressMemory,
} from './setup-progress'
beforeEach(resetSetupProgressMemory)
function memory() {
  const data = new Map<string, string>()
  const storage: SetupStorage = {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => {
      data.set(key, value)
    },
  }
  return { data, storage }
}
it('scopes validated progress to an owner and never copies another account acknowledgement', () => {
  const { data, storage } = memory()
  const progress = {
    ...emptySetupProgress(),
    skipped: ['models' as const],
    resume: 'voice' as const,
  }
  writeSetupProgress('alice', progress, storage)
  expect(readSetupProgress('alice', storage)).toEqual(progress)
  expect(readSetupProgress('bob', storage)).toEqual(emptySetupProgress())
  data.set(setupProgressKey('bob'), data.get(setupProgressKey('alice'))!)
  expect(readSetupProgress('bob', storage)).toEqual(emptySetupProgress())
})
it.each([
  'broken',
  '{}',
  '{"version":2}',
  '{"version":1,"ownerId":"alice","completed":true,"skipped":["bad"],"resume":"ready"}',
])('treats malformed or incompatible progress as unknown: %s', (raw) => {
  const { data, storage } = memory()
  data.set(setupProgressKey('alice'), raw)
  expect(readSetupProgress('alice', storage)).toEqual(emptySetupProgress())
})
it('stores only step acknowledgements and a safe destination, excluding arbitrary content', () => {
  const { data, storage } = memory()
  writeSetupProgress(
    'alice',
    {
      ...emptySetupProgress(),
      target: 'https://evil.test' as '/',
      sample: 'private writing',
      credential: 'secret',
    } as ReturnType<typeof emptySetupProgress>,
    storage,
  )
  const raw = data.get(setupProgressKey('alice'))!
  expect(Object.keys(JSON.parse(raw))).toEqual([
    'version',
    'ownerId',
    'completed',
    'skipped',
    'resume',
    'target',
  ])
  expect(raw).not.toMatch(/private writing|secret|evil/)
})
it('remains usable with absent or throwing storage and never stores an empty owner', () => {
  const storage = {
    getItem: () => {
      throw Error('blocked')
    },
    setItem: () => {
      throw Error('blocked')
    },
  }
  expect(readSetupProgress('alice', storage)).toEqual(emptySetupProgress())
  expect(() =>
    writeSetupProgress('alice', { ...emptySetupProgress(), completed: true }, storage),
  ).not.toThrow()
  expect(readSetupProgress('alice', null).completed).toBe(true)
  const { data, storage: readable } = memory()
  writeSetupProgress('', emptySetupProgress(), readable)
  expect(data.size).toBe(0)
})

it('retains an in-memory completion across navigation when writes are refused', () => {
  const storage = {
    getItem: () => null,
    setItem: () => {
      throw Error('blocked')
    },
  }
  writeSetupProgress('alice', { ...emptySetupProgress(), completed: true }, storage)
  expect(readSetupProgress('alice', storage).completed).toBe(true)
  expect(readSetupProgress('bob', storage).completed).toBe(false)
})
