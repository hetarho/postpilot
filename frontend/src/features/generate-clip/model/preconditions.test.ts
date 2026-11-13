import { describe, expect, it } from 'vitest'
import { emptyClipProject, type ClipProject, type ReadyClipBatch } from '@/entities/clip-project'
import type { CatalogModel, StageSelectionState } from '@/entities/model-catalog'
import {
  clipModelsReady,
  clipQuoteBinding,
  lacksClipObservation,
  renderClipSourcesAvailable,
  selectedClipStatus,
} from './preconditions'

const model = (id: string, extra: Partial<CatalogModel> = {}): CatalogModel => ({
  ref: { providerId: 'p', modelId: id },
  label: id,
  vision: true,
  videoInput: true,
  structuredOutput: true,
  stages: ['observe', 'write'],
  levels: {},
  disabled: false,
  disabledReason: '',
  contextTokens: 0n,
  requiredCredits: 1,
  affordable: true,
  ...extra,
})
const state = (id: string, extra: Partial<CatalogModel> = {}): StageSelectionState => ({
  models: [model(id, extra)],
  selected: { providerId: 'p', modelId: id },
  unavailable: undefined,
  isPending: false,
  isError: false,
})
const rows = (status: 'eligible' | 'price_ceiling_unavailable') => [
  { ref: { providerId: 'p', modelId: 'o' }, status },
]

describe('clip model readiness (T112)', () => {
  it('is ready only when the live answer says the observe model is eligible', () => {
    const write = state('w')
    expect(clipModelsReady(state('o'), write, { kind: 'ready', rows: rows('eligible') })).toBe(true)
    // The static-processing badge gates nothing either way.
    expect(
      clipModelsReady(state('o', { inlineStaticVideo: false }), write, {
        kind: 'ready',
        rows: rows('eligible'),
      }),
    ).toBe(true)
    expect(
      clipModelsReady(state('o'), write, {
        kind: 'ready',
        rows: rows('price_ceiling_unavailable'),
      }),
    ).toBe(false)
    expect(clipModelsReady(state('o'), write, { kind: 'ready', rows: [] })).toBe(false)
    expect(clipModelsReady(state('o'), write, { kind: 'loading' })).toBe(false)
    expect(clipModelsReady(state('o'), write, { kind: 'failed' })).toBe(false)
    expect(
      clipModelsReady(state('o', { disabled: true }), write, {
        kind: 'ready',
        rows: rows('eligible'),
      }),
    ).toBe(false)
  })
  it('binds a quote to the status it was read under, so a status change invalidates it', () => {
    const project = {
      ...emptyClipProject(),
      id: 'clip',
      createdAt: '',
      updatedAt: '',
      editPlanRevision: 0,
      renderedPlanRevision: 0,
    } as unknown as ClipProject
    const batch = {
      id: 'b',
      projectId: project.id,
      state: 'ready',
      expiresAt: '2099-01-01T00:00:00Z',
      sources: [],
    } as unknown as ReadyClipBatch
    const observe = { providerId: 'p', modelId: 'o' }
    const write = { providerId: 'p', modelId: 'w' }
    const eligible = clipQuoteBinding(project, batch, observe, write, 'eligible')
    expect(clipQuoteBinding(project, batch, observe, write, 'eligible')).toBe(eligible)
    expect(clipQuoteBinding(project, batch, observe, write, 'price_ceiling_unavailable')).not.toBe(
      eligible,
    )
    expect(clipQuoteBinding(project, batch, observe, write, undefined)).not.toBe(eligible)
    expect(selectedClipStatus({ kind: 'ready', rows: rows('eligible') }, observe)).toBe('eligible')
    expect(selectedClipStatus({ kind: 'loading' }, observe)).toBeUndefined()
    expect(selectedClipStatus({ kind: 'ready', rows: rows('eligible') }, null)).toBeUndefined()
  })
})

describe('render source admission', () => {
  const batch = (fingerprints: string[]) =>
    ({
      sources: fingerprints.map((fingerprint) => ({ metadata: { fingerprint } })),
    }) as ReadyClipBatch

  it('allows retained originals that the edited plan no longer uses', () => {
    expect(
      renderClipSourcesAvailable(batch(['first', 'second', 'unused']), ['first', 'second']),
    ).toBe(true)
  })

  it('refuses missing or duplicated retained fingerprints', () => {
    expect(renderClipSourcesAvailable(batch(['first', 'unused']), ['first', 'second'])).toBe(false)
    expect(
      renderClipSourcesAvailable(batch(['first', 'first', 'second']), ['first', 'second']),
    ).toBe(false)
  })
})

describe('analysis (CLIP-234)', () => {
  const observed = (id: string, fingerprint: string) => ({
    source: {
      id,
      fingerprint,
      filename: `${id}.mp4`,
      durationMs: 1000,
      width: 1,
      height: 1,
    },
    segments: [],
  })
  it('asks for 관찰하기 while a selected source has no observation of its current footage', () => {
    const observations = {
      status: 'available',
      sources: [observed('a', 'fa')],
    } as unknown as ClipProject['observations']
    expect(lacksClipObservation([{ sourceId: 'a', fingerprint: 'fa' }], observations)).toBe(false)
    expect(
      lacksClipObservation(
        [
          { sourceId: 'a', fingerprint: 'fa' },
          { sourceId: 'b', fingerprint: 'fb' },
        ],
        observations,
      ),
    ).toBe(true)
    // A replaced file under the same source is footage nothing observed.
    expect(lacksClipObservation([{ sourceId: 'a', fingerprint: 'other' }], observations)).toBe(true)
    expect(lacksClipObservation([{ sourceId: 'a', fingerprint: 'fa' }], undefined)).toBe(true)
  })

  it('binds 다시 관찰하기 quotes to the chosen sources in any order', () => {
    const project = {
      ...emptyClipProject(),
      id: 'clip',
      updatedAt: 'now',
    } as unknown as ClipProject
    const batch = { id: 'batch', sources: [] } as unknown as ReadyClipBatch
    const ref = { providerId: 'p', modelId: 'o' }
    const plain = clipQuoteBinding(project, batch, ref, ref, 'eligible')
    const chosen = clipQuoteBinding(project, batch, ref, ref, 'eligible', ['b', 'a'])
    expect(chosen).not.toBe(plain)
    expect(clipQuoteBinding(project, batch, ref, ref, 'eligible', ['a', 'b'])).toBe(chosen)
    expect(clipQuoteBinding(project, batch, ref, ref, 'eligible', ['a'])).not.toBe(chosen)
  })
})
