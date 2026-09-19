import i18next from 'i18next'
import { describe, expect, it } from 'vitest'
import { appFailureSpecs } from '@/shared/api'
import { FRAGMENTS, RESOURCE_NAMESPACES, resources } from '.'

interface Leaf {
  key: string
  value: string
}

function leaves(value: unknown, prefix = ''): Leaf[] {
  if (typeof value === 'string') return [{ key: prefix, value }]
  if (!value || typeof value !== 'object') return []
  return Object.entries(value).flatMap(([key, child]) =>
    leaves(child, prefix ? `${prefix}.${key}` : key),
  )
}

function placeholders(value: string): string[] {
  return [...value.matchAll(/{{\s*([^},\s]+).*?}}/g)].map((match) => match[1]!).sort()
}

describe('bundled locale resources', () => {
  it('registers exactly the product namespaces', () => {
    expect(Object.keys(resources.ko)).toEqual(RESOURCE_NAMESPACES)
    expect(Object.keys(resources.en)).toEqual(RESOURCE_NAMESPACES)
  })

  it('assembles the namespaces from slice fragments with no key claimed twice', () => {
    // A namespace is built by spreading its fragments, so two fragments claiming the same key
    // would silently take turns depending on import order. Every key belongs to ONE slice.
    const claims = new Map<string, string>()
    for (const fragment of FRAGMENTS) {
      expect(RESOURCE_NAMESPACES).toContain(fragment.namespace)
      expect(Object.keys(fragment.ko)).toEqual(Object.keys(fragment.en))
      for (const key of Object.keys(fragment.ko)) {
        const at = `${fragment.namespace}.${key}`
        expect(claims.has(at), `${at} is claimed twice`).toBe(false)
        claims.set(at, fragment.namespace)
      }
    }
    // Nothing is lost in the assembly: every fragment key reaches the namespace it names.
    for (const fragment of FRAGMENTS)
      for (const key of Object.keys(fragment.ko))
        expect(resources.ko[fragment.namespace as keyof typeof resources.ko]).toHaveProperty(key)
  })

  it('keeps recursive keys and interpolation placeholders identical', () => {
    for (const namespace of RESOURCE_NAMESPACES) {
      const ko = leaves(resources.ko[namespace])
      const en = leaves(resources.en[namespace])
      expect(
        en.map(({ key }) => key),
        `${namespace} keys`,
      ).toEqual(ko.map(({ key }) => key))

      for (let index = 0; index < ko.length; index += 1) {
        expect(
          placeholders(en[index]!.value),
          `${namespace}.${ko[index]!.key} placeholders`,
        ).toEqual(placeholders(ko[index]!.value))
      }
    }
  })

  it('covers every registered application failure in both locales', () => {
    expect(Object.keys(resources.ko.errors).sort()).toEqual(Object.keys(appFailureSpecs).sort())
    expect(Object.keys(resources.en.errors).sort()).toEqual(Object.keys(appFailureSpecs).sort())
  })

  it('exposes typed translation keys from the canonical resource shape', () => {
    const translated: string = i18next.t('metadata.title')
    expect(translated).toBeTruthy()
  })

  it('selects singular and plural English product copy from count', async () => {
    await i18next.changeLanguage('en')
    try {
      expect(i18next.t('count.remaining', { ns: 'common', count: 1 })).toBe('1 character remaining')
      expect(i18next.t('count.remaining', { ns: 'common', count: 2 })).toBe(
        '2 characters remaining',
      )
      expect(i18next.t('postCount', { ns: 'templates', count: 1 })).toBe('1 post')
      expect(i18next.t('postCount', { ns: 'templates', count: 2 })).toBe('2 posts')
      expect(i18next.t('upload.failedCount', { ns: 'posts', count: 1 })).toBe(
        '1 photo could not be uploaded',
      )
      expect(i18next.t('upload.failedCount', { ns: 'posts', count: 2 })).toBe(
        '2 photos could not be uploaded',
      )
      expect(i18next.t('profile.finalizedCount', { ns: 'voices', version: '3', count: 1 })).toBe(
        'v3 · 1 finalized post',
      )
      expect(i18next.t('profile.finalizedCount', { ns: 'voices', version: '3', count: 2 })).toBe(
        'v3 · 2 finalized posts',
      )
    } finally {
      await i18next.changeLanguage('ko')
    }
  })
})
