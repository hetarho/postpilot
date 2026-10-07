import {
  frameParity,
  mountFinalizedEditor,
  blankPresetSlots,
  mountEditor,
  editorState,
  seekEditor,
  editEditor,
  delays,
  componentCatalog,
  unmountEditor,
} from './editor-mounted'
import {
  BrowserLocalComponents,
  freezeBrowserPreviewComposition,
  evaluateBrowserFrame,
  type BrowserCompositionInput,
} from '@/entities/clip-preview'

interface NativeLayoutFixture extends Omit<
  BrowserCompositionInput,
  'ownerId' | 'projectId' | 'projectRevision' | 'planRevision'
> {
  id: string
  expected: {
    instanceId: string
    text: string
    position: string
    style: string
    startMs: number
    endMs: number
    box: { x: number; y: number; width: number; height: number }
    fontSize: number
  }[]
}
declare global {
  interface Window {
    browserEditorFixture: {
      layout: (fixture: NativeLayoutFixture) => Promise<unknown>
      finalized: typeof mountFinalizedEditor
      blank: typeof blankPresetSlots
      parity: typeof frameParity
      mount: typeof mountEditor
      state: typeof editorState
      seek: typeof seekEditor
      edit: typeof editEditor
      delays: typeof delays
      catalog: typeof componentCatalog
      unmount: typeof unmountEditor
    }
  }
}
window.browserEditorFixture = {
  finalized: mountFinalizedEditor,
  blank: blankPresetSlots,
  parity: frameParity,
  mount: mountEditor,
  state: editorState,
  seek: seekEditor,
  edit: editEditor,
  delays,
  catalog: componentCatalog,
  unmount: unmountEditor,
  async layout(fixture) {
    const snapshot = await freezeBrowserPreviewComposition({
      ownerId: 'synthetic-native-layout',
      projectId: fixture.id,
      projectRevision: 1,
      planRevision: 1,
      plan: fixture.plan,
      ratio: fixture.ratio,
      sources: fixture.sources,
      layoutObservations: fixture.layoutObservations,
      design: fixture.design,
    })
    const local = new BrowserLocalComponents(snapshot)
    const results: unknown[] = []
    try {
      await local.resolveLayout()
      for (const expected of fixture.expected) {
        const component = snapshot.components.find(
          (component) =>
            component.instanceId === expected.instanceId &&
            component.startMs === expected.startMs &&
            component.endMs === expected.endMs,
        )
        if (!component)
          throw new Error(
            `NATIVE_WINDOW_MISSING:${fixture.id}:${expected.text}:${expected.startMs}/${expected.endMs}`,
          )
        const state = evaluateBrowserFrame(snapshot, component.visibleFirstFrame).components.find(
          (state) => state.component === component,
        )!
        const geometry = await local.backgroundGeometry(state)
        const caption = geometry?.caption
        if (!caption) throw new Error(`NATIVE_CAPTION_MISSING:${fixture.id}`)
        const actualText =
          component.phraseIndex === undefined
            ? component.element.text
            : component.element.phrases![component.phraseIndex]!.text
        if (
          actualText !== expected.text ||
          caption.style.id !== expected.style ||
          caption.role.size !== expected.fontSize ||
          geometry!.anchor !== expected.position
        )
          throw new Error(
            `NATIVE_CAPTION_CONTRACT:${fixture.id}:${JSON.stringify({ actualText, style: caption.style.id, size: caption.role.size, anchor: geometry!.anchor, expected })}`,
          )
        for (const key of ['x', 'y', 'width', 'height'] as const)
          if (Math.abs(caption.region[key] - expected.box[key]) > 0.001)
            throw new Error(
              `NATIVE_CAPTION_BOX:${fixture.id}:${key}:${caption.region[key]}/${expected.box[key]}`,
            )
        results.push({
          text: actualText,
          position: geometry!.anchor,
          style: caption.style.id,
          fontSize: caption.role.size,
          box: caption.region,
        })
      }
      const first = fixture.expected[0],
        selected = first ? await local.captionGeometry(first.instanceId) : undefined
      if (
        first &&
        selected &&
        Object.entries(first.box).some(
          ([key, value]) =>
            Math.abs(value - selected.box[key as keyof typeof selected.box]) > 0.001,
        )
      )
        throw new Error(`NATIVE_FIRST_CUE_BOX:${fixture.id}`)
      return {
        id: fixture.id,
        passed: true,
        snapshotFingerprint: snapshot.snapshotFingerprint,
        purpose: snapshot.purpose,
        firstCue: selected,
        results,
      }
    } finally {
      local.destroy()
    }
  },
}
