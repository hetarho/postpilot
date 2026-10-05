import { expect, it } from 'vitest'
import { clipNarrationFixture } from '@/test/clip-editing'
import {
  applyTimelineEdit,
  clipTimelineReducer,
  createClipTimeline,
  nativeTextErrors,
} from './timeline'
import { captionRefresh, captionWords, refreshCaptionWording } from './caption-refresh'

function derived() {
  const plan = clipNarrationFixture().plan
  plan.narration = {
    enabled: true,
    confirmedVoiceId: 'voice',
    bindingDigest: 'binding',
    volumePermille: 1000,
    segments: [
      { id: 's', text: '변경된 대본', inputHash: 'new', textRevision: 2, startMs: 0, endMs: 11000 },
    ],
  }
  plan.elements![0].derivedCaption = {
    segmentId: 's',
    textRevision: 1,
    textEdited: false,
    timingEdited: true,
  }
  return plan
}
it('refreshes eligible wording alone, retaining protected timing, placement, style and speech identity', () => {
  const plan = derived(),
    caption = plan.elements![0]
  caption.ownerPosition = { x: 80, y: 1200 }
  caption.ownerSizePx = 72
  caption.ownerStyle = 'film'
  const result = refreshCaptionWording(plan)
  expect(result.elements![0]).toEqual({
    ...caption,
    text: '변경된 대본',
    phrases: [],
    ownerEdited: true,
    derivedCaption: { ...caption.derivedCaption!, textRevision: 2 },
  })
  expect(result.narration).toBe(plan.narration)
  expect(result.elements![1]).toBe(plan.elements![1])
  expect(result.refreshDerivedCaptions).toBe(true)
})
it('protects unsaved owner wording independently of style and timing edits and supports undo once', () => {
  const plan = derived()
  const positioned = applyTimelineEdit(plan, {
    type: 'text',
    id: plan.elements![0].instanceId,
    patch: { ownerPosition: { x: 70, y: 80 }, startMs: 500 },
  })
  expect(positioned.elements![0].derivedCaption).toMatchObject({
    textEdited: false,
    timingEdited: true,
  })
  const edited = applyTimelineEdit(positioned, {
    type: 'text',
    id: plan.elements![0].instanceId,
    patch: { text: '내 자막' },
  })
  expect(refreshCaptionWording(edited).elements![0].text).toBe('내 자막')
  let state = createClipTimeline(plan)
  state = clipTimelineReducer(state, { type: 'edit', edit: { type: 'refreshCaptions' }, at: 1 })
  expect(state.past).toHaveLength(1)
  state = clipTimelineReducer(state, { type: 'undo' })
  expect(state.plan.elements![0].text).toBe(plan.elements![0].text)
})
it('retains orphaned captions and offers missing/split segment additions explicitly', () => {
  const plan = derived()
  plan.narration!.segments[0].id = 'split-new'
  const proposal = captionRefresh(plan)
  expect(proposal.orphaned).toEqual([plan.elements![0].instanceId])
  expect(proposal.pending.map((s) => s.id)).toEqual(['split-new'])
  expect(refreshCaptionWording(plan)).toBe(plan)
  const added = applyTimelineEdit(plan, {
    type: 'addScriptCaption',
    id: 'local',
    segmentId: 'split-new',
    startMs: 500,
    endMs: 1000,
  })
  expect(added.elements?.at(-1)).toMatchObject({
    text: '변경된 대본',
    startMs: 500,
    endMs: 1000,
    derivedCaption: { segmentId: 'split-new', textEdited: false, timingEdited: false },
  })
  expect(added.narration).toEqual(plan.narration)
})
it('partitions Korean and supplementary Unicode without dropping spoken content', () => {
  expect(captionWords('서울😀에서 만나요', 2)).toEqual(['서울😀에서', '만나요'])
  const plan = derived(),
    second = {
      ...plan.elements![0],
      instanceId: 'second',
      resolvedStartMs: 12000,
      derivedCaption: { ...plan.elements![0].derivedCaption! },
    }
  plan.elements!.push(second)
  const refreshed = refreshCaptionWording(plan)
  expect([refreshed.elements![0].text, refreshed.elements!.at(-1)!.text].join('')).toBe(
    '변경된대본',
  )
})

it('keeps every rapid phrase interval when wording changes and reports protected-window overflow', () => {
  const plan = derived(),
    caption = plan.elements![0]
  caption.pace = 'rapid'
  caption.phrases = [
    { text: '첫말', startMs: 500, endMs: 1500 },
    { text: '끝말', startMs: 1600, endMs: 2600 },
  ]
  caption.derivedCaption!.timingEdited = false
  const after = refreshCaptionWording(plan)
  expect(after.elements![0].phrases).toEqual([
    { text: '변경된', startMs: 500, endMs: 1500 },
    { text: '대본', startMs: 1600, endMs: 2600 },
  ])
  expect(after.elements![0].derivedCaption).toMatchObject({
    textEdited: false,
    timingEdited: false,
  })
  const manuallyTyped = applyTimelineEdit(after, {
    type: 'text',
    id: caption.instanceId,
    patch: { phrases: [{ ...caption.phrases[0], text: '내말' }, caption.phrases[1]] },
  })
  expect(manuallyTyped.elements![0].derivedCaption).toMatchObject({
    textEdited: true,
    timingEdited: false,
  })
  const short = derived()
  short.elements![0].startMs = 500
  short.elements![0].endMs = 600
  const proposal = refreshCaptionWording(short)
  expect(nativeTextErrors(proposal)[0].exposure).toBe(true)
  expect(proposal.elements![0]).toMatchObject({ startMs: 500, endMs: 600, text: '변경된 대본' })
})
