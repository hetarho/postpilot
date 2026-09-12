import type { ClipEditingState } from '@/entities/clip-project'

export function clipTimelineFixture(): ClipEditingState {
  const state = clipEditingFixture()
  state.plan.nativeComposition = true
  state.plan.associations = []
  state.plan.elements = state.plan.cuts.map((cut, index) => ({
    instanceId: `caption-${cut.sourceId}`,
    elementId: 'caption',
    cutId: cut.id,
    kind: 'ai',
    role: 'caption',
    text: `caption ${cut.sourceId}`,
    rows: [],
    style: 'clean',
    position: 'bottom',
    align: 'center',
    basis: 'cut',
    startMs: 120,
    endMs: 3880,
    pace: 'steady',
    accent: '',
    keyword: '',
    resolvedStartMs: index * 9800 + 120,
    resolvedEndMs: index * 9800 + 3880,
    groupId: 'menu',
    itemId: cut.sourceId,
    evidence: [{ sourceId: cut.sourceId, fingerprint: cut.fingerprint, startMs: 0, endMs: 10000 }],
  }))
  state.plan.elements.push({
    instanceId: 'global',
    elementId: 'badge',
    cutId: '',
    kind: 'fixed',
    role: 'badge',
    text: '정확한 고정 문구',
    rows: [],
    style: 'auto',
    position: 'header',
    align: 'center',
    basis: 'whole',
    pace: 'steady',
    accent: '',
    keyword: '',
    resolvedStartMs: 0,
    resolvedEndMs: 19800,
    groupId: '',
    itemId: '',
  })
  return state
}

export function clipEditingFixture(): ClipEditingState {
  return {
    plan: {
      // A hook only the owner can ground; the fixture opens on the footage.
      hook: '',
      durationMs: 19800,
      cuts: ['a', 'b'].map((id, i) => ({
        id: `cut-${id}`,
        sourceId: id,
        fingerprint: id.repeat(64),
        startMs: 0,
        endMs: 10000,
        // The scene changes between the two, so the second leads in with
        // CDS-36's fade and the clip is 200 ms shorter than its footage.
        transitionMs: i === 0 ? 0 : 200,
        copies: [
          {
            text: `caption ${id}`,
            anchor: 'bottom' as const,
            align: 'center' as const,
            keyword: '',
            style: 'clean' as const,
            accent: '' as const,
            startMs: 0,
            endMs: 0,
          },
        ],
        chips: [],
        volumePermille: 1000,
      })),
    },
    sources: ['a', 'b'].map((id) => ({
      id,
      fingerprint: id.repeat(64),
      filename: `source-${id}.mp4`,
      durationMs: 40000,
      width: 1920,
      height: 1080,
    })),
    copyStyles: ['clean', 'memo', 'bold'],
    fadeMs: 200,
    maxCuts: 100,
    maxCopyRunes: 500,
    minDurationMs: 15000,
    maxDurationMs: 90000,
  }
}
