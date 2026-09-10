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
          { purpose: 'writing', register: ['b/one'], deregister: [], unchanged: [], relevel: [] },
          {
            purpose: 'photo-analysis',
            register: [],
            deregister: ['a/two'],
            unchanged: ['a/three'],
            relevel: [],
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
        purposes: [
          { purpose: 'writing', register: [], deregister: [], unchanged: ['a/one'], relevel: [] },
        ],
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
    {
      purpose: 'writing' as const,
      register: ['a/one'],
      deregister: [],
      unchanged: [],
      relevel: [],
    },
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
          purposes: [
            { purpose: 'writing', register: [], deregister: [], unchanged: ['a/one'], relevel: [] },
          ],
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

describe('documentDiff relevel (T094/MODEL-59)', () => {
  const relevelOnly = [
    {
      purpose: 'writing' as const,
      register: [],
      deregister: [],
      unchanged: ['a/kept'],
      relevel: [{ modelId: 'a/regraded', from: 'top' as const, to: '' as const }],
    },
  ]

  it('counts a re-grade and marks the section touched', () => {
    const diff = documentDiff(plan({ purposes: relevelOnly }))
    expect(diff.relevelCount).toBe(1)
    // A section that only re-grades has still changed something: rendering it as "no change"
    // would hide a paste that clears every grade in it.
    expect(diff.rows[0]?.touched).toBe(true)
    expect(diff.registerCount).toBe(0)
    expect(diff.deregisterCount).toBe(0)
  })

  it('lets a relevel-only document be applied', () => {
    // MODEL-59: a document that only moves grades changes the catalog as much as one that
    // moves registrations. Leaving it uncommittable would make the paste path unable to
    // express what the export can already write.
    expect(canApply(plan({ purposes: relevelOnly }))).toBe(true)
  })

  it('knows unknown_level as a cause of its own', () => {
    expect(isKnownCause('unknown_level')).toBe(true)
  })
})
