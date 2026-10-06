import {
  BrowserLocalComponents,
  freezeBrowserComposition,
  evaluateBrowserFrame,
  BrowserCaptionScenePixi,
} from '@/entities/clip-preview'
import type { ClipEditPlan } from '@/entities/clip-plan'
import type { ClipRatioId } from '@/entities/clip-design/@x/clip-preview'
import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import { stressInk } from './ink-stress'
import { WebGLRenderer, RenderTexture, TexturePool } from 'pixi.js'

export interface Fixture {
  id: string
  ratio: ClipRatioId
  plan: ClipEditPlan
  frame: number
  reference: string
  design: Parameters<typeof freezeBrowserComposition>[0]['design']
  renderer?: 'canvas' | 'pixi'
  nativeLayout?: { Text: string; X: number; Y: number; Width: number; Height: number }[]
}
// One isolated diagnostic realm. Tag actual core-pool sources without changing allocation or execution.
TexturePool.createTexture = new Proxy(TexturePool.createTexture, {
  apply(target, thisArg, args) {
    const texture = Reflect.apply(target, thisArg, args)
    texture.source.label = texture.label ?? 'texturePool_diagnostic'
    return texture
  },
})
declare global {
  interface Window {
    browserInkFixtures: {
      run: (fixture: Fixture) => Promise<unknown>
      stress: (fixture: Fixture, engine: 'canvas' | 'pixi') => Promise<unknown>
    }
  }
}
export async function freezeFixture(fixture: Fixture) {
  return freezeBrowserComposition({
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
}
window.browserInkFixtures = {
  stress: (fixture, engine) => stressInk(fixture, engine, freezeFixture),
  async run(fixture) {
    const snapshot = await freezeFixture(fixture)
    const dimensions = CLIP_DESIGN.ratios[fixture.ratio].canvas
    const canvas = new OffscreenCanvas(dimensions.width, dimensions.height),
      context = canvas.getContext('2d', { willReadFrequently: true })!
    const components = new BrowserLocalComponents(snapshot)
    let pixi: WebGLRenderer | undefined,
      sceneRenderer: BrowserCaptionScenePixi | undefined,
      target: RenderTexture | undefined
    if (fixture.renderer === 'pixi') {
      pixi = new WebGLRenderer()
      await pixi.init({
        canvas: new OffscreenCanvas(dimensions.width, dimensions.height),
        width: dimensions.width,
        height: dimensions.height,
        antialias: false,
        resolution: 1,
        backgroundAlpha: 0,
      })
      target = RenderTexture.create({ width: dimensions.width, height: dimensions.height })
      sceneRenderer = new BrowserCaptionScenePixi(pixi, (owner) => components.dropGPU(owner))
    }
    let reference: ImageBitmap | undefined
    try {
      const state = evaluateBrowserFrame(snapshot, fixture.frame)
      const start = performance.now()
      const resources = await components.prepare(state)
      resources.forEach((resource, index) => {
        if (sceneRenderer && resource.kind === 'scene')
          sceneRenderer.render(resource.scene, target, index === 0)
        else resource.draw(context)
      })
      const gpu = target && pixi?.extract.pixels({ target })
      if (gpu) {
        const pixels = new Uint8ClampedArray(gpu.pixels)
        for (let index = 0; index < pixels.length; index += 4) {
          const alpha = pixels[index + 3]!
          if (alpha)
            for (let channel = 0; channel < 3; channel++)
              pixels[index + channel] = Math.round((pixels[index + channel]! * 255) / alpha)
        }
        context.putImageData(new ImageData(pixels, canvas.width, canvas.height), 0, 0)
      }
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
            words: line.words.map((word) => ({ text: word.text, x: word.x, width: word.width })),
          })) ?? [],
      )
      if (fixture.nativeLayout) {
        if (glyphs.length !== fixture.nativeLayout.length)
          throw new Error('CLIP_INK_FIXTURE_LINE_COUNT')
        for (const [index, line] of glyphs.entries()) {
          const expected = fixture.nativeLayout[index]!
          if (
            line.text !== expected.Text ||
            [
              ['x', 'X'],
              ['y', 'Y'],
              ['width', 'Width'],
              ['height', 'Height'],
            ].some(
              ([a, b]) => Math.abs(Number(line[a as 'x']) - Number(expected[b as 'X'])) > 0.001,
            )
          )
            throw new Error('CLIP_INK_FIXTURE_LAYOUT')
          const words = (
            expected as typeof expected & { Words?: { Text: string; X: number; Width: number }[] }
          ).Words
          if (
            words &&
            (words.length !== line.words.length ||
              words.some(
                (word, i) =>
                  word.Text !== line.words[i]!.text ||
                  Math.abs(word.X - line.words[i]!.x) > 0.001 ||
                  Math.abs(word.Width - line.words[i]!.width) > 0.001,
              ))
          )
            throw new Error('CLIP_INK_FIXTURE_WORD_LAYOUT')
        }
      }
      resources.forEach((resource) => resource.close())
      const coldRasters = components.measurements().rasters
      const coldGPU = sceneRenderer?.measurements()
      const warm = await components.prepare(state)
      warm.forEach((resource, index) => {
        if (sceneRenderer && resource.kind === 'scene')
          sceneRenderer.render(resource.scene, target, index === 0)
      })
      if (sceneRenderer && JSON.stringify(sceneRenderer.measurements()) !== JSON.stringify(coldGPU))
        throw new Error('CLIP_INK_FIXTURE_GPU_RECREATION')
      warm.forEach((resource) => resource.close())
      if (components.measurements().rasters !== coldRasters || components.measurements().leases)
        throw new Error('CLIP_INK_FIXTURE_CACHE_OR_LEASE')
      reference = await createImageBitmap(
        await fetch(`/__ink-fixtures__/${fixture.reference}`).then((response) => response.blob()),
      )
      context.clearRect(0, 0, canvas.width, canvas.height)
      context.drawImage(reference, 0, 0)
      const native = context.getImageData(0, 0, canvas.width, canvas.height).data
      const wordExposure: { text: string; nativeAlpha: number; browserAlpha: number }[] = []
      if (fixture.id.includes('pop-long') && fixture.nativeLayout)
        for (const line of glyphs)
          for (const word of line.words) {
            let nativeAlpha = 0,
              browserAlpha = 0
            for (
              let y = Math.max(0, Math.floor(line.y - line.height));
              y < Math.min(canvas.height, Math.ceil(line.y + line.height));
              y++
            )
              for (
                let x = Math.max(0, Math.floor(word.x));
                x < Math.min(canvas.width, Math.ceil(word.x + word.width));
                x++
              ) {
                const offset = (y * canvas.width + x) * 4 + 3
                nativeAlpha += native[offset]!
                browserAlpha += actual[offset]!
              }
            if (nativeAlpha > 0 && browserAlpha === 0)
              throw new Error('CLIP_INK_FIXTURE_MISSING_WORD:' + word.text)
            wordExposure.push({ text: word.text, nativeAlpha, browserAlpha })
          }
      let changed = 0,
        total = 0,
        premultipliedTotal = 0,
        relevant = 0
      for (let i = 0; i < actual.length; i += 4) {
        if (actual[i + 3] || native[i + 3]) relevant++
        for (let channel = 0; channel < 4; channel++) {
          const delta = Math.abs(actual[i + channel]! - native[i + channel]!)
          if (delta) changed++
          total += delta
          premultipliedTotal +=
            channel === 3
              ? delta
              : Math.abs(
                  (actual[i + channel]! * actual[i + 3]!) / 255 -
                    (native[i + channel]! * native[i + 3]!) / 255,
                )
        }
      }
      let binary = ''
      const bytes = new Uint8Array(await png.arrayBuffer())
      for (let index = 0; index < bytes.length; index += 8192)
        binary += String.fromCharCode(...bytes.subarray(index, index + 8192))
      const measurements = components.measurements()
      const gpuMeasurements = sceneRenderer?.measurements()
      sceneRenderer?.destroy()
      if (
        sceneRenderer &&
        (sceneRenderer.measurements().nodes || sceneRenderer.measurements().groupBytes)
      )
        throw new Error('CLIP_INK_FIXTURE_GPU_RESOURCE_LEAK')
      components.destroy()
      if (components.measurements().bytes || components.measurements().entries)
        throw new Error('CLIP_INK_FIXTURE_RESOURCE_LEAK')
      return {
        id: fixture.id,
        changedBytes: changed,
        relevantPixels: relevant,
        normalizedError: relevant ? total / (relevant * 4 * 255) : 0,
        premultipliedError: relevant ? premultipliedTotal / (relevant * 4 * 255) : 0,
        glyphs,
        wordExposure,
        resources: measurements,
        elapsedMs: performance.now() - start,
        cleanupVerified: true,
        renderer: fixture.renderer ?? 'canvas',
        gpu: gpuMeasurements,
        scene: resources.flatMap((resource) =>
          resource.kind === 'scene'
            ? resource.scene.nodes.map((node) => ({
                id: node.id,
                pose: node.pose,
                bounds: node.document?.bounds,
                phase: node.ink?.offset,
              }))
            : [],
        ),
        png: btoa(binary),
      }
    } finally {
      reference?.close()
      sceneRenderer?.destroy()
      target?.destroy(true)
      pixi?.destroy()
      components.destroy()
      canvas.width = 0
      canvas.height = 0
    }
  },
}
