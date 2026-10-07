import {
  BrowserLocalComponents,
  BrowserCaptionScenePixi,
  evaluateBrowserFrame,
  type BrowserCompositionSnapshot,
} from '@/entities/clip-preview'
import { CLIP_DESIGN } from '@/entities/clip-design/@x/clip-preview'
import { WebGLRenderer, RenderTexture } from 'pixi.js'
import type { Fixture } from './ink-entry'

/** Diagnostic only: GPU finish/readback makes work completion measurable.
 * Neither synchronization nor per-frame readback enters the product path. */
export async function stressInk(
  fixture: Fixture,
  engine: 'canvas' | 'pixi',
  freeze: (fixture: Fixture) => Promise<BrowserCompositionSnapshot>,
) {
  const snapshot = await freeze(fixture),
    dimensions = CLIP_DESIGN.ratios[fixture.ratio].canvas
  const canvas = new OffscreenCanvas(dimensions.width, dimensions.height),
    context = canvas.getContext('2d')!
  const components = new BrowserLocalComponents(snapshot)
  let renderer: WebGLRenderer | undefined,
    scene: BrowserCaptionScenePixi | undefined,
    target: RenderTexture | undefined
  const init = async (version: 1 | 2 = 2) => {
    const value = new WebGLRenderer()
    await value.init({
      canvas: new OffscreenCanvas(dimensions.width, dimensions.height),
      ...dimensions,
      resolution: 1,
      backgroundAlpha: 0,
      preferWebGLVersion: version,
    })
    return value
  }
  if (engine === 'pixi') {
    renderer = await init()
    scene = new BrowserCaptionScenePixi(renderer, (owner) => components.dropGPU(owner))
    target = RenderTexture.create(dimensions)
  }
  const elapsed: number[] = [],
    hashes = new Map<number, string>()
  let peakCPU = 0,
    peakGPU = 0,
    peakScratch = 0,
    peakGeometry = 0,
    peakShapes = 0,
    peakShapeUniforms = 0,
    coldMs: number,
    warmRasters: number
  const render = async (frame: number, check = false) => {
    const begin = performance.now(),
      resources = await components.prepare(evaluateBrowserFrame(snapshot, frame))
    try {
      context.clearRect(0, 0, canvas.width, canvas.height)
      resources.forEach((resource, index) => {
        if (scene && resource.kind === 'scene') scene.render(resource.scene, target, index === 0)
        else resource.draw(context)
      })
      renderer?.gl.finish()
      const duration = performance.now() - begin
      const gpu = scene?.measurements(),
        cpu = components.measurements()
      peakCPU = Math.max(
        peakCPU,
        cpu.bytes + cpu.canvas.groupBytes + cpu.canvas.tintBytes + cpu.canvas.effectBytes,
      )
      peakGPU = Math.max(peakGPU, gpu?.managedTextureBytes ?? 0)
      peakScratch = Math.max(peakScratch, gpu?.filterScratchBytes ?? 0)
      peakGeometry = Math.max(peakGeometry, gpu?.geometryBytes ?? 0)
      peakShapes = Math.max(peakShapes, gpu?.dynamicShapes ?? 0)
      peakShapeUniforms = Math.max(peakShapeUniforms, gpu?.shapeUniformBytes ?? 0)
      if (check) {
        const pixels =
          renderer && target
            ? renderer.extract.pixels({ target }).pixels
            : context.getImageData(0, 0, canvas.width, canvas.height).data
        const bytes = new Uint8Array(
          await crypto.subtle.digest('SHA-256', new Uint8Array(pixels).buffer),
        )
        const hash = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
        if (hashes.has(frame) && hashes.get(frame) !== hash)
          throw new Error('CLIP_FILTER_STRESS_NONDETERMINISTIC')
        hashes.set(frame, hash)
      }
      return duration
    } finally {
      resources.forEach((resource) => resource.close())
    }
  }
  try {
    coldMs = await render(fixture.frame, true)
    const frames = Array.from({ length: 134 }, (_, i) => 32 + i)
    for (const frame of frames) await render(frame, frame % 15 === 0)
    warmRasters = components.measurements().rasters
    for (const frame of [...frames].reverse()) elapsed.push(await render(frame, frame % 15 === 0))
    if (components.measurements().rasters !== warmRasters || components.measurements().leases)
      throw new Error('CLIP_FILTER_STRESS_RASTER_OR_LEASE')
    const abort = new AbortController()
    abort.abort()
    let cancellation = false
    try {
      await components.prepare(evaluateBrowserFrame(snapshot, fixture.frame), abort.signal)
    } catch {
      cancellation = true
    }
    if (!cancellation || components.measurements().leases)
      throw new Error('CLIP_FILTER_STRESS_CANCEL')
    // More unique masks than the admitted cache capacity, preserving one owner.
    for (let i = 0; i < 72; i++) {
      const nextFixture = {
        ...fixture,
        plan: {
          ...fixture.plan,
          elements: fixture.plan.elements?.map((element) =>
            element.role === 'caption' ? { ...element, text: `갂 AV ${i}`, phrases: [] } : element,
          ),
        },
        design: { ...fixture.design, captionPace: 'steady' as const },
      }
      const next = await freeze(nextFixture)
      components.updateSnapshot(next)
      const resources = await components.prepare(evaluateBrowserFrame(next, fixture.frame))
      try {
        resources.forEach((resource, index) => {
          if (scene && resource.kind === 'scene') scene.render(resource.scene, target, index === 0)
          else resource.draw(context)
        })
        renderer?.gl.finish()
      } finally {
        resources.forEach((resource) => resource.close())
      }
      if (
        components.measurements().bytes > 64 * 1024 * 1024 ||
        components.measurements().entries > 64 ||
        components.measurements().leases
      )
        throw new Error('CLIP_FILTER_STRESS_EVICTION_BOUND')
      const gpu = scene?.measurements(),
        cpu = components.measurements()
      peakCPU = Math.max(
        peakCPU,
        cpu.bytes + cpu.canvas.groupBytes + cpu.canvas.tintBytes + cpu.canvas.effectBytes,
      )
      peakGPU = Math.max(peakGPU, gpu?.managedTextureBytes ?? 0)
      peakScratch = Math.max(peakScratch, gpu?.filterScratchBytes ?? 0)
      peakGeometry = Math.max(peakGeometry, gpu?.geometryBytes ?? 0)
      peakShapes = Math.max(peakShapes, gpu?.dynamicShapes ?? 0)
      peakShapeUniforms = Math.max(peakShapeUniforms, gpu?.shapeUniformBytes ?? 0)
    }
    let contextLoss = false,
      unsupported = false
    if (renderer && scene) {
      components.updateSnapshot(snapshot)
      const resources = await components.prepare(evaluateBrowserFrame(snapshot, fixture.frame))
      try {
        const drawing = resources.find((resource) => resource.kind === 'scene')
        if (!drawing || drawing.kind !== 'scene') throw new Error('CLIP_FILTER_STRESS_SCENE')
        const legacy = await init(1),
          legacyScene = new BrowserCaptionScenePixi(legacy, (owner) => components.dropGPU(owner))
        try {
          legacyScene.render(drawing.scene)
          throw new Error('CLIP_FILTER_STRESS_UNSUPPORTED_ACCEPTED')
        } catch (error) {
          unsupported = String(error).includes(
            'CLIP_INK_FILTER_UNSUPPORTED:' + fixture.design?.captionStyles?.[0],
          )
        } finally {
          legacyScene.destroy()
          legacy.destroy()
        }
        const extension = renderer.gl.getExtension('WEBGL_lose_context')
        if (!extension) throw new Error('CLIP_FILTER_STRESS_LOSE_EXTENSION')
        extension.loseContext()
        await new Promise((resolve) => setTimeout(resolve, 25))
        try {
          scene.render(drawing.scene)
        } catch (error) {
          contextLoss = String(error).includes(
            'CLIP_INK_CONTEXT_LOST:' + fixture.design?.captionStyles?.[0],
          )
        }
        if (!contextLoss || !unsupported) throw new Error('CLIP_FILTER_STRESS_REFUSAL')
      } finally {
        resources.forEach((resource) => resource.close())
      }
    }
    scene?.destroy()
    const after = scene?.measurements()
    if (
      after &&
      (after.nodes ||
        after.groupBytes ||
        after.effectBytes ||
        after.filters ||
        after.geometryBytes ||
        after.dynamicShapes ||
        after.shapeUniformBytes)
    )
      throw new Error('CLIP_FILTER_STRESS_OWNED_LEAK')
    components.destroy()
    if (
      components.measurements().bytes ||
      components.measurements().leases ||
      components.measurements().canvas.effectBytes
    )
      throw new Error('CLIP_FILTER_STRESS_CPU_LEAK')
    elapsed.sort((a, b) => a - b)
    return {
      id: fixture.id,
      engine,
      frames: 268,
      coldMs,
      warmP50Ms: elapsed[Math.floor(elapsed.length * 0.5)],
      warmP95Ms: elapsed[Math.floor(elapsed.length * 0.95)],
      warmRasters,
      peakCPUBytes: peakCPU,
      peakManagedGPUBytes: peakGPU,
      peakSharedFilterScratchBytes: peakScratch,
      peakGeometryBytes: peakGeometry,
      peakDynamicShapes: peakShapes,
      peakShapeUniformBytes: peakShapeUniforms,
      deterministicFrames: hashes.size,
      cancellation,
      eviction: true,
      contextLoss,
      unsupported,
      cleanup: true,
      rendererOwnedSharedPool: true,
      productionQualified: false,
    }
  } finally {
    scene?.destroy()
    target?.destroy(true)
    renderer?.destroy()
    components.destroy()
    canvas.width = canvas.height = 0
  }
}
