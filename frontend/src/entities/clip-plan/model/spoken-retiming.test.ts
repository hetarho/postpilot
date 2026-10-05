import { acknowledgeClipCuts } from './timeline'
import { describe, expect, it } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import {
  applyTimelineEdit,
  clipTimelineReducer,
  createClipTimeline,
  textInterval,
  validateTimelinePlan,
} from './timeline'
import { proposeSpokenRetiming } from './spoken-retiming'
import { spokenState, type ClipNarration } from './spoken'
function fixture() {
  const state = clipTimelineFixture(),
    n: ClipNarration = {
      enabled: true,
      confirmedVoiceId: 'voice',
      bindingDigest: 'binding',
      volumePermille: 1000,
      segments: [],
    }
  const speech = {
    assetId: 'asset',
    voiceId: 'voice',
    bindingDigest: 'binding',
    inputHash: 'input',
    settingsHash: 'settings',
    audioHash: 'audio',
    profileId: 'profile',
    profileRevision: 1,
    samples: 44100 * 22,
    sampleRate: 44100,
    channels: 2,
    timing: [],
  }
  n.segments = [
    {
      id: 'spoken-1',
      text: '말하는 원문',
      textRevision: 1,
      inputHash: 'input',
      startMs: 0,
      endMs: 10000,
      speech,
    },
  ]
  state.plan.narration = n
  const evidence = {
    status: 'available',
    sources: state.sources.map((source) => ({
      source,
      segments: [{ usability: 'usable', startMs: 0, endMs: 60000 }],
    })),
  }
  return { state, evidence, n }
}
describe('explicit script edits and natural-speed retiming', () => {
  it('stales exactly changed script, while caption and gain edits preserve every speech reference', () => {
    const { state, n } = fixture(),
      p = state.plan
    const muted = applyTimelineEdit(p, { type: 'sourceGain', volumePermille: 0 })
    expect(muted.narration).toEqual(n)
    const changed = applyTimelineEdit(p, {
      type: 'spokenPatch',
      id: 'spoken-1',
      patch: { text: '수정한 원문' },
    })
    expect(changed.elements).toEqual(p.elements)
    expect(changed.narration!.segments[0].speech).toEqual(n.segments[0].speech)
    expect(spokenState(changed.narration!, changed.narration!.segments[0])).toBe('stale')
    const voice = applyTimelineEdit(p, { type: 'narrationOptions', confirmedVoiceId: 'another' })
    expect(spokenState(voice.narration!, voice.narration!.segments[0])).toBe('stale')
    const off = applyTimelineEdit(p, { type: 'narrationOptions', enabled: false })
    expect(off.narration!.segments).toEqual(n.segments)
    expect(applyTimelineEdit(off, { type: 'narrationOptions', enabled: true }).narration).toEqual(n)
  })
  it('proposes eligible footage extension, preserves displayed words and output times, and applies with one undo', () => {
    const { state, evidence } = fixture(),
      before = state.plan,
      proposal = proposeSpokenRetiming(before, state, evidence, 30000)
    expect(proposal.available).toBe(true)
    if (!proposal.available) return
    expect(proposal.plan.durationMs).toBe(22000)
    expect(proposal.plan.narration!.segments[0].endMs).toBe(22000)
    for (const caption of before.elements!.filter((e) => e.role === 'caption')) {
      const next = proposal.plan.elements!.find((e) => e.instanceId === caption.instanceId)!
      expect(next.text).toBe(caption.text)
      const prior = textInterval(before, caption),
        after = textInterval(proposal.plan, next)
      expect([after.startMs, after.endMs]).toEqual([prior.startMs, prior.endMs])
    }
    expect(before.durationMs).toBe(19800)
    const applied = clipTimelineReducer(createClipTimeline(before), {
      type: 'edit',
      edit: { type: 'replacePlan', plan: proposal.plan, expectedKey: proposal.expectedKey },
      at: 1,
    })
    expect(applied.past).toHaveLength(1)
    expect(clipTimelineReducer(applied, { type: 'undo' }).plan).toEqual(before)
  })
  it('refuses insufficient source, owner duration overrun and stale speech without changing the draft', () => {
    const { state, evidence } = fixture()
    expect(proposeSpokenRetiming(state.plan, state, evidence, 20000)).toEqual({
      available: false,
      reason: 'duration',
    })
    expect(proposeSpokenRetiming(state.plan, state, undefined, 30000)).toEqual({
      available: false,
      reason: 'footage',
    })
    state.plan.narration!.segments[0].inputHash = 'changed'
    expect(proposeSpokenRetiming(state.plan, state, evidence, 30000)).toEqual({
      available: false,
      reason: 'speech',
    })
  })
  it('keeps original identity on split and undoes removal without changing captions', () => {
    const { state } = fixture()
    const p = applyTimelineEdit(state.plan, {
      type: 'splitSpoken',
      id: 'spoken-1',
      newId: 'new-segment',
      character: 2,
    })
    expect(p.narration!.segments.map((s) => s.id)).toEqual(['spoken-1', 'new-segment'])
    expect(p.narration!.segments[1].creation).toBe(true)
    expect(p.elements).toEqual(state.plan.elements)
    const s = clipTimelineReducer(createClipTimeline(p), {
      type: 'edit',
      edit: { type: 'removeSpoken', id: 'spoken-1' },
      at: 1,
    })
    expect(clipTimelineReducer(s, { type: 'undo' }).plan).toEqual(p)
  })
  it('acknowledges newly minted segment ids through changed undo history without reminting', () => {
    const { state } = fixture()
    const submitted = applyTimelineEdit(state.plan, {
      type: 'addSpoken',
      id: 'temporary',
      startMs: 22000,
      endMs: 24000,
    })
    submitted.narration!.segments[1].text = 'saved words'
    const accepted = structuredClone(submitted)
    accepted.narration!.segments[1] = {
      ...accepted.narration!.segments[1],
      id: 'spoken-2',
      creation: undefined,
      inputHash: 'saved-hash',
    }
    const history = structuredClone(submitted)
    history.narration!.segments[1].text = 'earlier words'
    const known = acknowledgeClipCuts(history, accepted, submitted)
    expect(known.narration!.segments[1].id).toBe('spoken-2')
    expect(known.narration!.segments[1].creation).toBeUndefined()
    expect(known.narration!.segments[1].text).toBe('earlier words')
  })
  it('saves conflicting narration for correction but blocks current export even at zero narration gain', () => {
    const { state, evidence } = fixture()
    state.plan.narration!.volumePermille = 0
    const check = validateTimelinePlan(state.plan, state, evidence)
    expect(check.saveable).toBe(true)
    expect(check.valid).toBe(false)
    expect(check.speechReady).toBe(false)
    const off = applyTimelineEdit(state.plan, { type: 'narrationOptions', enabled: false })
    expect(validateTimelinePlan(off, state, evidence).valid).toBe(true)
  })
})
