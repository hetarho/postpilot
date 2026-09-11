import type { ClipEditingState } from '@/entities/clip-project'

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
