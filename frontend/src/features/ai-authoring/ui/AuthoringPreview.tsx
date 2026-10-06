import { useTranslation } from 'react-i18next'
import { TemplatePreview, parseTemplate, TEMPLATE_PARSE_OPTIONS } from '@/entities/template'
import { parseClipTemplate, type ClipComposition } from '@/entities/clip-template'
import type { AuthoringArtifact, AuthoringKind } from '@/entities/ai-authoring'
import { Typography } from '@/shared/ui'

import { readableAuthoringProse } from '../lib/readable-prose'

/** The compiled payload never becomes source text on the normal authoring surface. */
export function AuthoringPreview({
  kind,
  artifact,
}: {
  kind: AuthoringKind
  artifact: AuthoringArtifact
}) {
  const { t } = useTranslation('authoring')
  if (kind === 'writing-voice' && artifact.body.trim() === '')
    return <Typography variant="body">{t('voiceNeedsExample')}</Typography>
  if (kind === 'post-template') {
    const parsed = parseTemplate(artifact.titleArea, artifact.body, TEMPLATE_PARSE_OPTIONS)
    return parsed.ok ? (
      <TemplatePreview titleArea={artifact.titleArea} body={artifact.body} />
    ) : (
      <Typography variant="body">{t('previewUnavailable')}</Typography>
    )
  }
  if (kind === 'video-template') {
    let document: ClipComposition
    try {
      document = parseClipTemplate(artifact.body)
    } catch {
      return <Typography variant="body">{t('previewUnavailable')}</Typography>
    }
    const fieldLabels = new Map(document.fields.map((field) => [field.id, field.label]))
    const prose = (parts: ClipComposition['elements'][number]['parts']) =>
      parts
        .map((part) =>
          part.field
            ? t('videoValue', { label: fieldLabels.get(part.field) ?? t('videoDetail') })
            : part.literal,
        )
        .join('')
    return (
      <div className="space-y-4">
        <Typography variant="body">{t('videoSummary')}</Typography>
        <ol className="space-y-4">
          {document.outline.map((entry, index) => {
            if (entry.kind === 'stage') {
              const stage = document.stages[entry.index]
              return (
                <li key={index} className="bg-surface-recessed rounded-lg p-4 break-words">
                  <Typography variant="fieldTitle" as="span">
                    {stage.name}
                  </Typography>
                  <Typography variant="body" className="mt-2">
                    {stage.intent}
                  </Typography>
                </li>
              )
            }
            const element = document.elements[entry.index]
            const text = element.rows.length
              ? element.rows.map((row) => prose(row.parts)).join(' / ')
              : prose(element.parts)
            return (
              <li key={index} className="bg-surface-recessed rounded-lg p-4 break-words">
                <Typography variant="label">{t(`videoRoles.${element.role}`)}</Typography>
                <Typography variant="body" className="mt-2">
                  {text || t('videoAI')}
                </Typography>
              </li>
            )
          })}
        </ol>
      </div>
    )
  }
  if (!readableAuthoringProse(artifact.body))
    return <Typography variant="body">{t('previewUnavailable')}</Typography>
  return (
    <div className="space-y-4">
      {kind === 'writing-voice' && <Typography variant="label">{t('fictional')}</Typography>}
      <Typography variant="body" className="max-w-measure break-words whitespace-pre-wrap">
        {artifact.body}
      </Typography>
    </div>
  )
}
