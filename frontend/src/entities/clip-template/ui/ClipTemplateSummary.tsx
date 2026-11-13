import { useTranslation } from 'react-i18next'
import { CLIP_DEFAULT_CAPTION_STYLE } from '@/entities/clip-design/@x/clip-template'
import { Typography } from '@/shared/ui'
import { readStoredClipComposition } from '../lib/composition-parse'
import type { ClipRecipe } from '../model/types'

/** A static, illustrative outline that can also live inside a choice or link. */
export function ClipTemplateSummary({ template }: { template: ClipRecipe }) {
  const { t } = useTranslation('clips')
  let outline: string[] = []
  let example = ''
  let readable = false
  try {
    const document = readStoredClipComposition(template.compositionBody)
    const fieldLabel = (reference: string) => {
      const field = document.fields.find(
        (item) => (item.group ? `${item.group}.${item.id}` : item.id) === reference,
      )
      return field?.label || reference
    }
    outline = document.outline
      .map((entry) => {
        if (entry.kind === 'stage') return document.stages[entry.index].name
        const element = document.elements[entry.index]
        const pieces = element.rows.length
          ? element.rows.flatMap((row) => row.parts)
          : element.parts
        const content = pieces
          .map((part) => (part.field ? fieldLabel(part.field) : part.literal))
          .join('')
          .trim()
        const role = t(`composition.role.${element.role}`)
        const hasAI = element.rows.length
          ? element.rows.some((row) => row.kind === 'ai')
          : element.kind === 'ai'
        const detail = [hasAI ? t('composition.kind.ai') : '', content].filter(Boolean).join(' · ')
        const label = detail ? `${role}: ${detail}` : role
        if (!example) example = label
        return label
      })
      .filter(Boolean)
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
        <Typography
          as="span"
          variant="meta"
          className="text-media-scrim-fg line-clamp-1 shrink-0 break-words"
        >
          {t('composition.summary.example')}
        </Typography>
        <Typography
          as="span"
          variant="label"
          className="text-media-scrim-fg line-clamp-2 shrink-0 break-words"
        >
          {t('composition.summary.intro')}
          <br />
          {design('intro', template.introPreset)}
        </Typography>
        <Typography
          as="span"
          variant="label"
          className="bg-media-scrim-bg text-media-scrim-fg line-clamp-2 shrink-0 rounded-sm p-1 break-words"
        >
          {example || outline[0] || t('composition.summary.body')}
        </Typography>
        <Typography
          as="span"
          variant="label"
          className="text-media-scrim-fg line-clamp-2 shrink-0 break-words"
        >
          {t('composition.summary.outro')}
          <br />
          {design('outro', template.outroPreset)}
        </Typography>
      </span>
      <span className="min-w-0 flex-1 space-y-2">
        <Typography as="span" variant="body" className="block break-words">
          {readable && outline.length ? outline.join(' → ') : t('composition.summary.noOutline')}
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
