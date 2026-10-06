import { expect, it, vi } from 'vitest'
import { clipTimelineFixture } from '@/test/clip-editing'
import { renderBrowserAudio } from './render-audio'

it('returns no audio track, decoder or source access when every source is off', async () => {
  const plan = clipTimelineFixture().plan
  plan.sourceAudio = plan.cuts.map((cut) => ({
    sourceId: cut.sourceId,
    fingerprint: cut.fingerprint,
    retainOriginalAudio: false,
  }))
  const originals = { source: vi.fn() }
  expect(
    await renderBrowserAudio(plan, 'vertical', originals, new AbortController().signal),
  ).toBeUndefined()
  expect(originals.source).not.toHaveBeenCalled()
})
