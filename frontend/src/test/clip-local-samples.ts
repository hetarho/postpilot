import { CLIP_CAPTION_STYLES, CLIP_DESIGN, type ClipRatioId } from '@/entities/clip-design'
import type { BrowserProjectCompositionInput } from '@/entities/clip-preview'
let presetFailure = false
export function failLocalPresetSamplesForControls(failed: boolean) {
  presetFailure = failed
}
/** Control-test metadata only. Actual fonts/WASM/geometry are measured by browser-editor-check. */
export const localSamplesForControls = {
  useClipLocalStyleSamples: (ratio: ClipRatioId, enabled: boolean) => ({
    isError: false,
    data: enabled
      ? {
          ratio,
          canvas: { x: 0, y: 0, ...CLIP_DESIGN.ratios[ratio].canvas },
          safeArea: { ...CLIP_DESIGN.ratios[ratio].safe },
          captions: CLIP_CAPTION_STYLES.map((style) => ({
            instanceId: style,
            style,
            svg: `<g data-style="${style}"><text>오늘의 한 장면</text></g>`,
            box: { x: 0, y: 0, width: 600, height: 120 },
            fontSize: 72,
            representativeFrame: !['bold', 'keynote', 'film'].includes(style),
          })),
        }
      : undefined,
  }),
  useClipLocalPresetSamples: (ratio: ClipRatioId, slotLabel: string) => ({
    isError: presetFailure,
    data: presetFailure
      ? undefined
      : {
          ratio,
          canvas: { x: 0, y: 0, ...CLIP_DESIGN.ratios[ratio].canvas },
          intro: Object.keys(CLIP_DESIGN.regions.intro).map((preset) => ({
            preset,
            svg: `<g data-preset="intro-${preset}"><text>${slotLabel.replace('{n}', '1')}</text></g>`,
            box: { x: 100, y: 800, width: 880, height: 300 },
          })),
          outro: Object.keys(CLIP_DESIGN.regions.outro).map((preset) => ({
            preset,
            svg: `<g data-preset="outro-${preset}"><text>${slotLabel.replace('{n}', '1')}</text></g>`,
            box: { x: 100, y: 800, width: 880, height: 300 },
          })),
        },
  }),
  useClipLocalCaptionPreview: (
    input: BrowserProjectCompositionInput | undefined,
    enabled: boolean,
  ) => ({
    isError: false,
    isPending: false,
    isLoading: false,
    refetch: () => {},
    data:
      enabled && input
        ? {
            ratio: input.ratio,
            canvas: { x: 0, y: 0, width: 1080, height: 1920 },
            safeArea: { x: 60, y: 120, width: 960, height: 1680 },
            captions: (input.plan.elements ?? [])
              .filter((text) => text.role === 'caption')
              .map((text) => ({
                instanceId: text.instanceId,
                style: text.ownerStyle || text.style,
                svg: `<g data-caption="${text.instanceId}"><text>${text.text}</text></g>`,
                box: { x: 240, y: 1500, width: 600, height: 120 },
                fontSize: 72,
                representativeFrame: false,
              })),
          }
        : undefined,
  }),
}
