import { clsx } from 'clsx'
import { useTranslation } from 'react-i18next'
import { CLIP_DEFAULT_CAPTION_STYLE } from '@/entities/clip-design/@x/clip-template'
import { Typography } from '@/shared/ui'
import { readStoredClipComposition } from '../lib/composition-parse'
import type { ClipRecipe } from '../model/types'

// Presentation bounds keep each choice's visible and spoken description concise.
const ITEM_CHARS = 40
const OUTLINE_CHARS = 160
const SEPARATOR = ' → '

function shorten(value: string, limit: number): string {
  const chars = Array.from(value)
  return chars.length > limit
    ? `${chars
        .slice(0, limit - 1)
        .join('')
        .trimEnd()}…`
    : value
}

function summarizeOutline(entries: string[]): string {
  const shown: string[] = []
  for (const entry of entries) {
    const candidate = [...shown, shorten(entry, ITEM_CHARS)].join(SEPARATOR)
    if (Array.from(candidate).length > OUTLINE_CHARS - SEPARATOR.length - 1) {
      return `${shown.join(SEPARATOR)}${SEPARATOR}…`
    }
    shown.push(shorten(entry, ITEM_CHARS))
  }
  return shown.join(SEPARATOR)
}

type Translate = ReturnType<typeof useTranslation<'clips'>>['t']

/** What a template's stored body and design read as, for the summary and its miniature alike. A
 *  body that is absent or unreadable still yields its design, so a name and a choice still work. */
function readClipTemplate(template: ClipRecipe, t: Translate) {
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
  return { outline, example, readable, design }
}

/** The static 9:16 example a video template is recognised by (CLIP-236): its intro and outro
 *  designs and its first entry over a dark frame, labelled as an example. It holds no interactive
 *  child, so it may stand inside a choice button or a directory tile's link; it plays, measures
 *  and generates nothing. `tile` is the directory's larger miniature. */
function ClipTemplateMiniature({
  template,
  size = 'summary',
}: {
  template: ClipRecipe
  size?: 'summary' | 'tile'
}) {
  const { t } = useTranslation('clips')
  const { outline, example, design } = readClipTemplate(template, t)
  return (
    <span
      className={clsx(
        'bg-media-canvas-bg text-media-scrim-fg flex aspect-9/16 shrink-0 flex-col justify-between overflow-hidden rounded-lg p-2',
        size === 'tile' ? 'w-28' : 'w-24',
      )}
    >
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
        {shorten(example || outline[0] || t('composition.summary.body'), ITEM_CHARS)}
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
  )
}

/** A static, illustrative outline that can also live inside a choice or link. `miniature` is the
 *  9:16 example alone, larger, as a directory tile leads with it (CLIP-42). */
export function ClipTemplateSummary({
  template,
  variant = 'summary',
}: {
  template: ClipRecipe
  variant?: 'summary' | 'miniature'
}) {
  const { t } = useTranslation('clips')
  if (variant === 'miniature') return <ClipTemplateMiniature template={template} size="tile" />
  const { outline, readable, design } = readClipTemplate(template, t)
  const styles = template.allowedCaptionStyles.length
    ? template.allowedCaptionStyles
    : [CLIP_DEFAULT_CAPTION_STYLE]
  const styleNames = styles.map((style) => t(`captionStyles.${style}`, { defaultValue: style }))
  return (
    <span className="mt-3 flex min-w-0 items-start gap-4">
      <ClipTemplateMiniature template={template} />
      <span className="min-w-0 flex-1 space-y-2">
        <Typography as="span" variant="body" className="block break-words">
          {readable && outline.length
            ? summarizeOutline(outline)
            : t('composition.summary.noOutline')}
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

/** The one-line description a directory card shows under a video template's name, as a post
 *  template shows its own: the stages it runs through, in order (인트로 · 첫인상 · 대표 메뉴 ·
 *  아웃트로). A template has no written description, so its stages are what tells it apart. */
export function ClipTemplateDescription({
  template,
  className,
}: {
  template: ClipRecipe
  className?: string
}) {
  const { t } = useTranslation('clips')
  let stages: string[] = []
  try {
    stages = readStoredClipComposition(template.compositionBody)
      .stages.map((stage) => stage.name.trim())
      .filter(Boolean)
  } catch {
    // An unreadable body still has its name; the card then says it has no outline.
  }
  return (
    <Typography variant="body" as="span" className={className}>
      {stages.length ? stages.join(' · ') : t('composition.summary.noOutline')}
    </Typography>
  )
}
