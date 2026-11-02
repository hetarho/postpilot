import { expect, it } from 'vitest'
import readiness from './browser-readiness.json'
import { CLIP_BROWSER_ANALYSIS_PREPARATION } from './browser-analysis'
import { CLIP_BROWSER_COMPOSITION } from './browser-composition'

// ARCH-68: the shipped record opens final rendering and keeps analysis preparation closed.
it('activates browser final rendering independently of analysis preparation', () => {
  expect(CLIP_BROWSER_COMPOSITION.qualified).toBe(true)
  expect(readiness.finalRender.followUpGates.length).toBeGreaterThan(0)
  expect(CLIP_BROWSER_ANALYSIS_PREPARATION.qualified).toBe(false)
})
