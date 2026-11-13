import { useTranslation } from 'react-i18next'
import { CLIP_DEFAULT_CAPTION_STYLE } from '@/entities/clip-design/@x/clip-template'
import { Typography } from '@/shared/ui'
import { readStoredClipComposition } from '../lib/composition-parse'
import type { ClipRecipe } from '../model/types'

/** A static, illustrative outline that can also live inside a choice or link. */
export function ClipTemplateSummary({ template }: { template: ClipRecipe }) {
  const { t } = useTranslation('clips')
  let stages: string[] = []
  let readable = false
  try {
    const document = readStoredClipComposition(template.compositionBody)
    stages = document.stages.map((stage) => stage.name).filter(Boolean)
    readable = true
  } catch {
    // A stored body can be absent or unreadable; its name and selection still work.
  }
  const design = (region: 'intro' | 'outro', preset: string) =>
    preset
      ? t(`composition.design.${region}_${preset}`, { defaultValue: preset })
      : t('composition.summary.unspecified')
  const styles = template.allowedCaptionStyles.length
    ? template.allowedCaptionStyles
    : [CLIP_DEFAULT_CAPTION_STYLE]
  const styleNames = styles.map((style) => t(`captionStyles.${style}`, { defaultValue: style }))
  return (
    <span className="mt-3 flex min-w-0 items-start gap-4">
      <span className="bg-media-canvas-bg text-media-scrim-fg flex aspect-9/16 w-24 shrink-0 flex-col justify-between overflow-hidden rounded-lg p-2">
        <Typography as="span" variant="meta" className="text-media-scrim-fg block break-words">
          {t('composition.summary.example')}
        </Typography>
        <Typography as="span" variant="label" className="text-media-scrim-fg block break-words">
          {t('composition.summary.intro')}
          <br />
          {design('intro', template.introPreset)}
        </Typography>
        <Typography
          as="span"
          variant="label"
          className="bg-media-scrim-bg text-media-scrim-fg block rounded-sm p-1 break-words"
        >
          {stages[0] || t('composition.summary.body')}
        </Typography>
        <Typography as="span" variant="label" className="text-media-scrim-fg block break-words">
          {t('composition.summary.outro')}
          <br />
          {design('outro', template.outroPreset)}
        </Typography>
      </span>
      <span className="min-w-0 flex-1 space-y-2">
        <Typography as="span" variant="body" className="block break-words">
          {readable && stages.length ? stages.join(' → ') : t('composition.summary.noOutline')}
        </Typography>
        <Typography as="span" variant="meta" className="block break-words">
          {t('composition.summary.design', {
            intro: design('intro', template.introPreset),
            outro: design('outro', template.outroPreset),
            styles: styleNames.join(', '),
          })}
        </Typography>
      </span>
    </span>
  )
}
