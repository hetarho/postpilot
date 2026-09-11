import type { ClipEditingState } from '@/entities/clip-project'

export function clipEditingFixture(): ClipEditingState {
  return {
    plan: {
      durationMs: 19800,
      cuts: ['a', 'b'].map((id) => ({
        id: `cut-${id}`,
        sourceId: id,
        fingerprint: id.repeat(64),
        startMs: 0,
        endMs: 10000,
        copy: {
          text: `caption ${id}`,
          anchor: 'bottom',
          align: 'center',
          keyword: '',
          style: 'clean',
          accent: '',
          startMs: 0,
          endMs: 0,
        },
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
