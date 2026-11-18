import { beforeEach, expect, it } from 'vitest'
import {
  forgetLastTemplates,
  lastTemplateKey,
  listedLastTemplate,
  readLastTemplate,
  rememberLastTemplate,
  type LastTemplateStorage,
} from './last-template'

function memoryStorage(): LastTemplateStorage & { entries: Map<string, string> } {
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

beforeEach(() => forgetLastTemplates(null))

it('keeps the last specified template per account, 없음 included', () => {
  const storage = memoryStorage()
  expect(readLastTemplate('alice', storage)).toBeUndefined()
  rememberLastTemplate('alice', 'template-diary', storage)
  rememberLastTemplate('bob', '', storage)
  expect(readLastTemplate('alice', storage)).toBe('template-diary')
  expect(readLastTemplate('bob', storage)).toBe('')
  expect(storage.entries.get(lastTemplateKey('alice'))).toBe('template-diary')
})

it('falls back to 없음 for a template no longer in the directory, and to nothing when never set', () => {
  const templates = [{ id: 'template-review' }]
  expect(listedLastTemplate('template-review', templates)).toBe('template-review')
  expect(listedLastTemplate('template-deleted', templates)).toBe('')
  expect(listedLastTemplate('', templates)).toBe('')
  expect(listedLastTemplate(undefined, templates)).toBeUndefined()
})

it('keeps the answer for the session where browser storage refuses it', () => {
  const refusing: LastTemplateStorage = {
    getItem: () => {
      throw new Error('denied')
    },
    setItem: () => {
      throw new Error('denied')
    },
  }
  rememberLastTemplate('alice', 'template-diary', refusing)
  expect(readLastTemplate('alice', refusing)).toBe('template-diary')
})
