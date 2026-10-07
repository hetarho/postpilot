import { useLayoutEffect, useState } from 'react'
import type { ClipProject } from '@/entities/clip-project'
import { useClipCorrection, ClipCorrectionWorkspace } from '@/features/correct-clip'
import { ClipDraftPreviewPanel } from '@/features/preview-clip-draft'
import { ClipDesignSelection } from '@/features/edit-clip-project/ui/ClipDesignSelection'
const design = (value: ClipProject) => ({
  captionStyles: value.allowedCaptionStyles,
  captionPace: value.captionPace,
  accent: value.accent,
  introPreset: value.introPreset,
  outroPreset: value.outroPreset,
  hideDisclosure: true,
})
function Editor({
  initial,
  onAdopt,
  inspect,
  resolvePlayback,
}: {
  initial: ClipProject
  onAdopt: (value: ClipProject) => void
  inspect: (value: ReturnType<typeof useClipCorrection>) => void
  resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>
}) {
  const correction = useClipCorrection('synthetic-editor-owner', initial)
  useLayoutEffect(() => {
    inspect(correction)
  })
  return (
    <>
      <ClipCorrectionWorkspace
        project={{
          id: initial.id,
          ownerId: 'synthetic-editor-owner',
          ratio: initial.ratio,
          state: initial.editing!,
          captionStyles: initial.allowedCaptionStyles,
          design: design(initial),
        }}
        correction={correction}
        render={{
          ready: false,
          pending: false,
          start: () => {
            throw new Error('Unexpected render from editor fixture')
          },
        }}
        footage={{ localSources: [], resolvePlayback }}
        disabled={false}
        slots={{
          preview: (controls) => (
            <ClipDraftPreviewPanel
              {...controls}
              ownerId="synthetic-editor-owner"
              projectId={initial.id}
              projectRevision={initial.editPlanRevision}
              revision={correction.revision}
              plan={correction.previewPlan}
              ratio={initial.ratio}
              sources={initial.editing!.sources}
              layoutObservations={initial.editing!.layoutObservations}
              design={design(initial)}
              resolvePlayback={resolvePlayback}
            />
          ),
        }}
      />
      <details>
        <summary>Local component catalog</summary>
        <ClipDesignSelection
          projectId={initial.id}
          draft={initial}
          onChange={(change) => onAdopt({ ...initial, ...change })}
        />
      </details>
    </>
  )
}
export function EditorMountedSurface({
  initial,
  inspect,
  resolvePlayback,
}: {
  initial: ClipProject
  inspect: (value: ReturnType<typeof useClipCorrection>) => void
  resolvePlayback: (fingerprint: string, refresh?: boolean) => Promise<string>
}) {
  const [value, setValue] = useState(initial)
  return (
    <Editor
      initial={value}
      onAdopt={setValue}
      inspect={inspect}
      resolvePlayback={resolvePlayback}
    />
  )
}
