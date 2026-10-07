import { NativeFadeBlackSurface } from '@/entities/clip-preview/model/native-fadeblack'
import { nativeFadeBlackPixel } from '@/entities/clip-preview/model/background-math'
import { freezeBrowserComposition } from '@/entities/clip-preview/model/browser-composition'
import { BrowserLocalComponents } from '@/entities/clip-preview/model/local-components'
import { evaluateBrowserFrame } from '@/entities/clip-preview/model/browser-composition'
import { drawMeasuredBrowserComponents } from '@/entities/clip-preview/model/background-paint'
import { CLIP_DESIGN } from '@/entities/clip-design'
import {
  measureBrowserBackground,
  type BrowserBackgroundDiagnostics,
} from '@/entities/clip-preview/model/background-sampling'
import type { ClipRatio } from '@/entities/clip-project'
import type { ClipEditableText } from '@/entities/clip-plan'

declare global {
  interface Window {
    backgroundFixture: () => unknown
    measureBackgroundFixture: (
      inputs: { url: string; fingerprint: string }[],
      ratio: ClipRatio,
      transitionMs?: number,
      cancel?: boolean,
      style?: string,
      references?: { frame: number; url: string }[],
    ) => Promise<unknown>
  }
}
window.measureBackgroundFixture = async (
  inputs,
  ratio,
  transitionMs = 0,
  cancel = false,
  style,
  references,
) => {
  const controller = new AbortController()
  const durationMs = inputs.length * (style ? 2000 : 1000) - transitionMs
  const startMs = inputs.length > 1 ? 1000 - transitionMs : 0
  const endMs = inputs.length > 1 ? 1000 : durationMs
  const base = {
    kind: 'fixed',
    position: 'bottom',
    align: 'center',
    basis: 'output-start',
    startMs,
    endMs,
    resolvedStartMs: startMs,
    resolvedEndMs: endMs,
    pace: 'steady',
    accent: 'cyan',
    keyword: '',
    groupId: '',
    itemId: '',
    cutId: '',
  }
  const roles = style ? ['caption'] : ['caption', 'info', 'hook', 'ending']
  const elements = roles.map((role, i) => ({
    ...base,
    instanceId: `${role}-instance`,
    elementId: role,
    role,
    text: style ? '지금 보는 화면 기록' : '화면 기록',
    rows:
      role === 'info'
        ? [
            { role: 'label', text: '정보' },
            { role: 'caption', text: '현재 화면' },
          ]
        : role === 'hook'
          ? [
              { role: 'caption', text: '시작' },
              { role: 'caption', text: '기록' },
            ]
          : role === 'ending'
            ? [
                { role: 'caption', text: '평점' },
                { role: 'caption', text: '4.5' },
                { role: 'caption', text: '다음 기록' },
              ]
            : [],
    style: style ?? 'bold',
    keyword: style ? '화면' : '',
    position: i === 1 ? 'header' : 'bottom',
  })) as ClipEditableText[]
  const plan = {
    durationMs,
    nativeComposition: true,
    cuts: inputs.map((source, i) => ({
      id: `cut-${i}`,
      sourceId: `source-${i}`,
      fingerprint: source.fingerprint,
      startMs: 0,
      endMs: style ? 2000 : 1000,
      transitionMs: i ? transitionMs : 0,
      playbackRatePermille: 1000,
      volumePermille: 0,
      copies: [],
      focal: { x: i ? 1 : 0, y: 0.5 },
    })),
    elements,
  }
  const snapshot = await freezeBrowserComposition({
    ownerId: 'fixture',
    projectId: 'fixture',
    projectRevision: 1,
    planRevision: 1,
    plan,
    ratio,
    authoritativeFingerprint: 'a'.repeat(64),
    design: { hideDisclosure: true, accent: style ? 'blue' : '', captionStyles: [style ?? 'bold'] },
    sources: inputs.map((source, i) => ({
      sourceId: `source-${i}`,
      fingerprint: source.fingerprint,
      durationMs: 2000,
      width: 64,
      height: 32,
      hasAudio: false,
      allowedRatePermille: [1000],
    })),
  })
  const components = new BrowserLocalComponents(snapshot)
  let diagnostics: BrowserBackgroundDiagnostics | undefined
  try {
    const evidence = await measureBrowserBackground(
      snapshot,
      components,
      async (sourceId, fingerprint, signal) => {
        signal.throwIfAborted()
        const index = Number(sourceId.slice('source-'.length))
        if (inputs[index]?.fingerprint !== fingerprint) throw new Error('CLIP_SOURCE_UNAVAILABLE')
        if (cancel) controller.abort(new DOMException('Fixture cancel', 'AbortError'))
        return { kind: 'url', url: inputs[index]!.url }
      },
      controller.signal,
      (value) => {
        diagnostics = value
      },
    )
    const scenes = []
    if (style) {
      const canvasSize = CLIP_DESIGN.ratios[ratio].canvas
      const canvas = new OffscreenCanvas(canvasSize.width, canvasSize.height)
      const context = canvas.getContext('2d', { alpha: false })!
      try {
        for (const index of [1, 30, 58]) {
          const frame = evaluateBrowserFrame(snapshot, index)
          const resources = await components.prepare(frame, controller.signal, () => ({
            accent: CLIP_DESIGN.accent.blue,
            accentWhite: evidence.measurements[0]!.accentWhite,
          }))
          const brightness = evidence.measurements[0]!.ground.rgb.map((v) => Math.round(v * 255))
          context.fillStyle = `rgb(${brightness.join(',')})` // style-escape: actual measured synthetic footage colour
          context.fillRect(0, 0, canvas.width, canvas.height)
          try {
            drawMeasuredBrowserComponents(context, snapshot, frame, resources, evidence)
            const data = context.getImageData(0, 0, canvas.width, canvas.height).data
            let ink = 0,
              cyan = 0,
              changed = 0
            let comparison: unknown
            for (let at = 0; at < data.length; at += 4) {
              if (
                data[at] !== brightness[0] ||
                data[at + 1] !== brightness[1] ||
                data[at + 2] !== brightness[2]
              )
                changed++
              if (data[at]! > 240 && data[at + 1]! > 240 && data[at + 2]! > 240) ink++
              if (data[at + 1]! > data[at]! + 40 && data[at + 2]! > data[at]! + 40) cyan++
            }
            const reference = references?.find((r) => r.frame === index)
            if (reference) {
              const response = await fetch(reference.url)
              if (!response.ok) throw new Error('Native reference missing')
              const bitmap = await createImageBitmap(await response.blob())
              const expected = new OffscreenCanvas(canvas.width, canvas.height)
              try {
                const ctx = expected.getContext('2d', { willReadFrequently: true })!
                ctx.drawImage(bitmap, 0, 0)
                const pixels = ctx.getImageData(0, 0, canvas.width, canvas.height).data
                const box = evidence.measurements[0]!.geometry.caption!.region
                let total = 0,
                  captionTotal = 0,
                  captionPixels = 0,
                  max = 0
                for (let at = 0; at < data.length; at += 4) {
                  const x = (at / 4) % canvas.width,
                    y = Math.floor(at / 4 / canvas.width)
                  let delta = 0
                  for (let c = 0; c < 3; c++) {
                    const d = Math.abs(data[at + c]! - pixels[at + c]!)
                    delta += d
                    max = Math.max(max, d)
                  }
                  total += delta
                  if (
                    x >= box.x - 40 &&
                    x < box.x + box.width + 40 &&
                    y >= box.y - 40 &&
                    y < box.y + box.height + 40
                  ) {
                    captionTotal += delta
                    captionPixels++
                  }
                }
                comparison = {
                  mean: total / (canvas.width * canvas.height * 3),
                  captionMean: captionTotal / (captionPixels * 3),
                  max,
                }
              } finally {
                bitmap.close()
                expected.width = 0
                expected.height = 0
              }
            }
            scenes.push({
              frame: index,
              kinds: resources.map((r) => r.kind),
              nodes: resources.flatMap((r) =>
                r.kind === 'scene'
                  ? r.scene.nodes.map((n) => ({
                      id: n.id,
                      tint: n.pose.tint,
                      opacity: n.pose.opacity,
                      clip: n.pose.clip,
                    }))
                  : [],
              ),
              changed,
              ink,
              cyan,
              comparison,
            })
          } finally {
            resources.forEach((r) => r.close())
          }
        }
        const frame = evaluateBrowserFrame(snapshot, 30)
        const resources = await components.prepare(frame, controller.signal)
        let staleRefused = false
        try {
          drawMeasuredBrowserComponents(context, snapshot, frame, resources, {
            ...evidence,
            localSnapshotFingerprint: 'b'.repeat(64),
          })
        } catch (error) {
          staleRefused = error instanceof Error && error.message === 'CLIP_SNAPSHOT_SUPERSEDED'
        } finally {
          resources.forEach((r) => r.close())
        }
        if (!staleRefused) throw new Error('CLIP_BACKGROUND_STALE_EVIDENCE_ACCEPTED')
      } finally {
        canvas.width = 0
        canvas.height = 0
      }
    }
    return { evidence, diagnostics, scenes, qualification: false }
  } catch (error) {
    return {
      error: error instanceof Error ? error.message : String(error),
      diagnostics,
      qualification: false,
    }
  } finally {
    components.destroy()
  }
}
window.backgroundFixture = () => {
  const canvas = new OffscreenCanvas(64, 32),
    context = canvas.getContext('2d', { alpha: false })!
  const gpu = new NativeFadeBlackSurface(64, 32)
  const cases = []
  const palette = ['#FF0000', '#00FF00', '#0000FF', '#FFFFFF'] // style-escape: fixed diagnostic RGB quadrants
  try {
    for (const weight of [0, 0.1, 0.2, 0.5, 0.8, 1]) {
      context.globalAlpha = 1
      context.fillStyle = '#000000' // style-escape: physical diagnostic video black
      context.fillRect(0, 0, 64, 32)
      context.globalAlpha = weight
      context.fillStyle = '#FFFFFF' // style-escape: physical diagnostic video white
      context.fillRect(0, 0, 64, 32)
      context.globalAlpha = 1
      gpu.apply(canvas, context, 1 - weight)
      const pixel = Array.from(context.getImageData(0, 0, 1, 1).data)
      const expected = nativeFadeBlackPixel([1, 1, 1], [0, 0, 0], weight, 0).map((v) =>
        Math.round(v * 255),
      )
      cases.push({
        weight,
        pixel,
        expected,
        error: Math.max(...expected.map((v, c) => Math.abs(v - pixel[c]!))),
      })
      context.fillStyle = '#000000' // style-escape: physical diagnostic video black
      context.fillRect(0, 0, 64, 32)
      context.globalAlpha = weight
      for (let i = 0; i < 4; i++) {
        context.fillStyle = palette[i]!
        context.fillRect((i % 2) * 32, Math.floor(i / 2) * 16, 32, 16)
      }
      context.globalAlpha = 1
      gpu.apply(canvas, context, 1 - weight)
      for (let i = 0; i < 4; i++) {
        const rgb = [1, 3, 5].map((at) => parseInt(palette[i]!.slice(at, at + 2), 16) / 255) as [
          number,
          number,
          number,
        ]
        const expected = nativeFadeBlackPixel(rgb, [0, 0, 0], weight, 0).map((v) =>
          Math.round(v * 255),
        )
        const pixel = Array.from(
          context.getImageData((i % 2) * 32 + 8, Math.floor(i / 2) * 16 + 8, 1, 1).data,
        )
        cases.push({
          weight,
          pixel,
          expected,
          error: Math.max(...expected.map((v, c) => Math.abs(v - pixel[c]!))),
        })
      }
    }
    return { cases, passed: cases.every((c) => c.error <= 1), qualification: false }
  } finally {
    gpu.close()
    canvas.width = 0
    canvas.height = 0
  }
}
