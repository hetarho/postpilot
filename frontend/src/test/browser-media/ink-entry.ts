import {
  BrowserLocalComponents,
  freezeBrowserComposition,
  evaluateBrowserFrame,
} from '@/entities/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan'
import type { ClipRatioId } from '@/entities/clip-design/@x/clip-preview'
import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'

interface Fixture {
  id: string
  ratio: ClipRatioId
  plan: ClipEditPlan
  frame: number
  reference: string
  design: Parameters<typeof freezeBrowserComposition>[0]['design']
}
declare global {
  interface Window {
    browserInkFixtures: { run: (fixture: Fixture) => Promise<unknown> }
  }
}
window.browserInkFixtures = {
  async run(fixture) {
    const snapshot = await freezeBrowserComposition({
      ownerId: 'synthetic',
      projectId: fixture.id,
      projectRevision: 1,
      planRevision: 1,
      plan: fixture.plan,
      ratio: fixture.ratio,
      design: fixture.design,
      sources: [
        {
          sourceId: 'source',
          fingerprint: 'a'.repeat(64),
          durationMs: 15000,
          width: 1920,
          height: 1080,
          hasAudio: false,
          allowedRatePermille: [1000],
        },
      ],
    })
    const dimensions = CLIP_DESIGN.ratios[fixture.ratio].canvas
    const canvas = new OffscreenCanvas(dimensions.width, dimensions.height),
      context = canvas.getContext('2d', { willReadFrequently: true })!
    const components = new BrowserLocalComponents(snapshot)
    let reference: ImageBitmap | undefined
    try {
      const state = evaluateBrowserFrame(snapshot, fixture.frame)
      const start = performance.now()
      const resources = await components.prepare(state)
      resources.forEach((resource) => resource.draw(context))
      const actual = context.getImageData(0, 0, canvas.width, canvas.height).data
      const png = await canvas.convertToBlob({ type: 'image/png' })
      const glyphs = resources.flatMap(
        (resource) =>
          resource.caption?.lines.map((line) => ({
            text: line.text,
            x: line.x,
            y: line.y,
            width: line.width,
            height: line.height,
          })) ?? [],
      )
      resources.forEach((resource) => resource.close())
      const coldRasters = components.measurements().rasters
      const warm = await components.prepare(state)
      warm.forEach((resource) => resource.close())
      if (components.measurements().rasters !== coldRasters || components.measurements().leases)
        throw new Error('CLIP_INK_FIXTURE_CACHE_OR_LEASE')
      reference = await createImageBitmap(
        await fetch(`/__ink-fixtures__/${fixture.reference}`).then((response) => response.blob()),
      )
      context.clearRect(0, 0, canvas.width, canvas.height)
      context.drawImage(reference, 0, 0)
      const native = context.getImageData(0, 0, canvas.width, canvas.height).data
      let changed = 0,
        total = 0,
        relevant = 0
      for (let i = 0; i < actual.length; i += 4) {
        if (actual[i + 3] || native[i + 3]) relevant++
        for (let channel = 0; channel < 4; channel++) {
          const delta = Math.abs(actual[i + channel]! - native[i + channel]!)
          if (delta) changed++
          total += delta
        }
      }
      let binary = ''
      const bytes = new Uint8Array(await png.arrayBuffer())
      for (let index = 0; index < bytes.length; index += 8192)
        binary += String.fromCharCode(...bytes.subarray(index, index + 8192))
      const measurements = components.measurements()
      components.destroy()
      if (components.measurements().bytes || components.measurements().entries)
        throw new Error('CLIP_INK_FIXTURE_RESOURCE_LEAK')
      return {
        id: fixture.id,
        changedBytes: changed,
        relevantPixels: relevant,
        normalizedError: relevant ? total / (relevant * 4 * 255) : 0,
        glyphs,
        resources: measurements,
        elapsedMs: performance.now() - start,
        cleanupVerified: true,
        png: btoa(binary),
      }
    } finally {
      reference?.close()
      components.destroy()
      canvas.width = 0
      canvas.height = 0
    }
  },
}
