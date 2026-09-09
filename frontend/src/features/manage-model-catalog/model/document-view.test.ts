import { describe, expect, it } from 'vitest'
import type { CatalogDocumentPlan } from '@/entities/model-catalog'
import { canApply, documentDiff, isKnownCause } from './document-view'

function plan(over: Partial<CatalogDocumentPlan> = {}): CatalogDocumentPlan {
  return { purposes: [], issues: [], fetchError: '', applied: false, ...over }
}

describe('documentDiff', () => {
  it('orders the named purposes and names the ones left alone', () => {
    const diff = documentDiff(
      plan({
        purposes: [
          { purpose: 'writing', register: ['b/one'], deregister: [], unchanged: [] },
          {
            purpose: 'photo-analysis',
            register: [],
            deregister: ['a/two'],
            unchanged: ['a/three'],
          },
        ],
      }),
    )
    // Tab order, not the order the document happened to list them in.
    expect(diff.rows.map((row) => row.purpose)).toEqual(['photo-analysis', 'writing'])
    expect(diff.untouched).toEqual(['style-analysis', 'image-generation', 'video-generation'])
    expect(diff.registerCount).toBe(1)
    expect(diff.deregisterCount).toBe(1)
  })

  it('marks a section that changes nothing as untouched rather than empty', () => {
    const diff = documentDiff(
      plan({
        purposes: [{ purpose: 'writing', register: [], deregister: [], unchanged: ['a/one'] }],
      }),
    )
    expect(diff.rows[0]?.touched).toBe(false)
    // It is still a named section, so it is NOT in the untouched list — the document did
    // speak about it, and re-stating the same members is a different thing from silence.
    expect(diff.untouched).not.toContain('writing')
  })
})

describe('canApply', () => {
  const change = [
    { purpose: 'writing' as const, register: ['a/one'], deregister: [], unchanged: [] },
  ]

  it('refuses a document nobody has previewed', () => {
    expect(canApply(undefined)).toBe(false)
  })

  it('refuses a plan with any rejected line', () => {
    expect(
      canApply(
        plan({ purposes: change, issues: [{ line: 3, text: 'x', cause: 'malformed_line' }] }),
      ),
    ).toBe(false)
  })

  it('refuses a plan whose catalog could not be read', () => {
    expect(canApply(plan({ purposes: change, fetchError: 'unreadable' }))).toBe(false)
  })

  it('refuses a plan that would change nothing', () => {
    expect(
      canApply(
        plan({
          purposes: [{ purpose: 'writing', register: [], deregister: [], unchanged: ['a/one'] }],
        }),
      ),
    ).toBe(false)
  })

  it('allows a clean plan that changes something', () => {
    expect(canApply(plan({ purposes: change }))).toBe(true)
  })
})

describe('isKnownCause', () => {
  it('rejects a slug a newer server invented, so it renders as the fallback', () => {
    expect(isKnownCause('malformed_line')).toBe(true)
    expect(isKnownCause('some_future_cause')).toBe(false)
  })
})
