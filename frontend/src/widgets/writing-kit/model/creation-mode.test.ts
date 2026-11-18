import { beforeEach, expect, it } from 'vitest'
import {
  creationModeKey,
  forgetCreationModes,
  readCreationMode,
  rememberCreationMode,
  type CreationModeStorage,
} from './creation-mode'

function memoryStorage(): CreationModeStorage & { entries: Map<string, string> } {
  const entries = new Map<string, string>()
  return {
    entries,
    getItem: (key) => entries.get(key) ?? null,
    setItem: (key, value) => void entries.set(key, value),
    removeItem: (key) => void entries.delete(key),
    get length() {
      return entries.size
    },
    key: (index) => [...entries.keys()][index] ?? null,
  }
}

beforeEach(() => forgetCreationModes(null))

it('keeps the centre’s last choice per account, 글쓰기 until one is made', () => {
  const storage = memoryStorage()
  expect(readCreationMode('alice', storage)).toBe('post')
  rememberCreationMode('alice', 'video', storage)
  expect(readCreationMode('alice', storage)).toBe('video')
  expect(readCreationMode('bob', storage)).toBe('post')
  storage.entries.set(creationModeKey('bob'), 'podcast')
  expect(readCreationMode('bob', storage)).toBe('post')
})

it('keeps the choice for the session where browser storage refuses it', () => {
  const refusing: CreationModeStorage = {
    getItem: () => {
      throw new Error('denied')
    },
    setItem: () => {
      throw new Error('denied')
    },
  }
  rememberCreationMode('alice', 'video', refusing)
  expect(readCreationMode('alice', refusing)).toBe('video')
})
