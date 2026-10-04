import { afterEach, beforeEach, expect, it, vi } from 'vitest'
import { CLIP_TIMELINE } from '@/entities/clip-design'
import { noRegionEdits } from './region-draft'
import {
  clipRegionsState,
  discardClipRegionQueues,
  peekClipRegions,
  queueClipRegions,
} from './region-queue'

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  discardClipRegionQueues()
  vi.useRealTimers()
})

/** An editor's send: it reads that editor's own edits when it goes out. */
function editor(name: string, sent: string[]) {
  return async () => {
    sent.push(name)
  }
}

it('keeps a project’s slot edits on its own editor after another project is edited', async () => {
  const sent: string[] = []
  queueClipRegions('a', noRegionEdits(), editor('a', sent))
  await vi.advanceTimersByTimeAsync(CLIP_TIMELINE.autosaveMs / 2)
  // B's editor registers while A's edits are still waiting out the beat.
  queueClipRegions('b', noRegionEdits(), editor('b', sent))
  expect(peekClipRegions('a')).toBeDefined()
  await vi.advanceTimersByTimeAsync(CLIP_TIMELINE.autosaveMs)
  // A's owed slots went out through A's editor, and A is saved only by its own send.
  expect(sent.sort()).toEqual(['a', 'b'])
  expect(clipRegionsState('a')).toBe('saved')
  expect(clipRegionsState('b')).toBe('saved')
})

it('sends nothing for one project through another project’s editor', async () => {
  const sent: string[] = []
  queueClipRegions('a', noRegionEdits(), editor('a', sent))
  queueClipRegions('b', noRegionEdits(), editor('b', sent))
  queueClipRegions('b', noRegionEdits(), editor('b again', sent))
  await vi.advanceTimersByTimeAsync(CLIP_TIMELINE.autosaveMs)
  expect(sent.sort()).toEqual(['a', 'b again'])
})
