import { describe, expect, it } from 'vitest'
import { REASONING_EFFORTS, type AdminCatalogEntry } from '@/entities/model-catalog'
import { offersReasoningControl, reasoningOptionsFor, sortEntries } from './catalog-view'

function entry(over: Partial<AdminCatalogEntry> = {}): AdminCatalogEntry {
  return {
    modelId: 'vendor/model',
    providerSlug: 'vendor',
    label: 'Model',
    description: '',
    vision: false,
    videoInput: false,
    structuredOutput: false,
    contextTokens: 0n,
    inputUsdPerMillion: '',
    outputUsdPerMillion: '',
    curated: true,
    purposes: ['writing'],
    imageOutput: false,
    videoOutput: false,
    listed: true,
    reasoningEffort: '',
    sourceCreatedAt: 0n,
    reasoning: {
      reasons: true,
      efforts: [],
      defaultEffort: '',
      mandatory: false,
      nativeEffort: false,
      maxTokens: false,
      drifted: false,
      known: true,
    },
    ...over,
  }
}

describe('sortEntries', () => {
  const cheap = entry({
    modelId: 'b/cheap',
    providerSlug: 'b',
    inputUsdPerMillion: '0.5',
    outputUsdPerMillion: '2',
  })
  const dear = entry({
    modelId: 'a/dear',
    providerSlug: 'a',
    inputUsdPerMillion: '10',
    outputUsdPerMillion: '50',
  })
  // Lexically "9.5" sorts after "10"; numerically it is the smaller.
  const nine = entry({
    modelId: 'c/nine',
    providerSlug: 'c',
    inputUsdPerMillion: '1',
    outputUsdPerMillion: '9.5',
  })
  const ten = entry({
    modelId: 'c/ten',
    providerSlug: 'c',
    inputUsdPerMillion: '2',
    outputUsdPerMillion: '10',
  })
  // Same output price as `ten`; a cheaper input breaks the tie.
  const tenCheapIn = entry({
    modelId: 'c/ten-cheap-in',
    providerSlug: 'c',
    inputUsdPerMillion: '1',
    outputUsdPerMillion: '10',
  })
  const video = entry({ modelId: 'd/video', providerSlug: 'd', videoOutput: true })
  const all = [video, ten, dear, nine, cheap, tenCheapIn]
  const ids = (entries: AdminCatalogEntry[]) => entries.map((e) => e.modelId)

  it('keeps the provider/newest order by default', () => {
    expect(ids(sortEntries(all))).toEqual(ids(sortEntries(all, 'default')))
    // Non-featured vendors alphabetically, ids within one when created at the same time.
    expect(ids(sortEntries(all, 'default'))).toEqual([
      'a/dear',
      'b/cheap',
      'c/nine',
      'c/ten',
      'c/ten-cheap-in',
      'd/video',
    ])
  })

  it('orders by output price numerically, input as the tie-break, unpriced last', () => {
    expect(ids(sortEntries(all, 'price-asc'))).toEqual([
      'b/cheap',
      'c/nine',
      'c/ten-cheap-in',
      'c/ten',
      'a/dear',
      'd/video',
    ])
  })

  it('reverses the priced order and still leaves unpriced models last', () => {
    expect(ids(sortEntries(all, 'price-desc'))).toEqual([
      'a/dear',
      'c/ten',
      'c/ten-cheap-in',
      'c/nine',
      'b/cheap',
      'd/video',
    ])
  })

  it('does not mutate its input', () => {
    const input = [...all]
    sortEntries(input, 'price-asc')
    expect(ids(input)).toEqual(ids(all))
  })
})

describe('reasoningOptionsFor', () => {
  // A3, against a real model: deepseek/deepseek-v4-pro-0813 accepts max·high·low and NOT
  // medium, and is disable-able yet lists no `none`.
  it("offers the model's own list plus unset, and no value it does not publish", () => {
    const options = reasoningOptionsFor(
      entry({ reasoning: { ...entry().reasoning, efforts: ['max', 'high', 'low'] } }),
    )
    expect(options).toEqual(['', 'unset', 'max', 'high', 'low'])
    for (const absent of ['medium', 'xhigh', 'minimal', 'none']) {
      expect(options).not.toContain(absent)
    }
  })

  // A4 first half: a mandatory model never offers `none`, even where the list contains it.
  it('withholds none from a mandatory model', () => {
    const options = reasoningOptionsFor(
      entry({
        reasoning: { ...entry().reasoning, efforts: ['high', 'none'], mandatory: true },
      }),
    )
    expect(options).not.toContain('none')
    expect(options).toContain('high')
  })

  it('withholds none from a mandatory model that publishes no list at all', () => {
    const options = reasoningOptionsFor(
      entry({ reasoning: { ...entry().reasoning, mandatory: true } }),
    )
    expect(options).not.toContain('none')
    // Everything else survives: mandatory says reasoning cannot be OFF, not that the model
    // accepts fewer efforts.
    expect(options).toContain('max')
    expect(options).toContain('minimal')
  })

  // A4 second half: no published list means unknown, so all eight stay on offer.
  it('falls back to the full vocabulary when the source publishes no list', () => {
    expect(reasoningOptionsFor(entry())).toEqual([...REASONING_EFFORTS])
  })

  // A7: the control has to be able to SHOW the value it is warning about. A Listbox whose
  // value is not among its options renders empty, which would hide the drift entirely.
  it('keeps a drifted override on offer so the row can display it', () => {
    const options = reasoningOptionsFor(
      entry({
        reasoningEffort: 'medium',
        reasoning: { ...entry().reasoning, efforts: ['max', 'high', 'low'], drifted: true },
      }),
    )
    expect(options).toContain('medium')
    expect(options).toEqual(['', 'unset', 'max', 'high', 'low', 'medium'])
  })
})

describe('offersReasoningControl', () => {
  // A8: a model the source says does not reason has no effort to choose.
  it('withholds the control from a listed model that does not reason', () => {
    expect(
      offersReasoningControl(entry({ reasoning: { ...entry().reasoning, reasons: false } })),
    ).toBe(false)
  })

  it('keeps the control for a model that reasons', () => {
    expect(offersReasoningControl(entry())).toBe(true)
  })

  // An entry served from storage — a row written before this data existed, or any row on a
  // screen whose provider fetch failed — says `known: false`. Hiding the control there would
  // take away an override that is still being sent on every call.
  it('keeps the control when the capability is unknown', () => {
    expect(
      offersReasoningControl(
        entry({ reasoning: { ...entry().reasoning, reasons: false, known: false } }),
      ),
    ).toBe(true)
  })
})
