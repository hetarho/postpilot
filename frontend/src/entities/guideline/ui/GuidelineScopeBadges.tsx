import { useTranslation } from 'react-i18next'
import { BLOG_FIELD_IDS, blogFieldLabelKey } from '@/entities/blog-field/@x/guideline'
import { Badge, Typography } from '@/shared/ui'
import { isOrphanedScope, type Guideline } from '../model/types'

type ScopeView = Pick<Guideline, 'scope' | 'templates' | 'fields'> &
  Partial<Pick<Guideline, 'kind'>>

/** A guideline's scope in full, as an open row states it (GUIDE-47): `전역`, the template-name
 *  chips, the 분야 chips, or `적용 대상 없음` for a scope whose every template was deleted. Colour
 *  is never the only signal: the orphaned state says so in words, and its help line says what to do
 *  about it (THEME-33). */
export function GuidelineScopeBadges({ guideline }: { guideline: ScopeView }) {
  const { t } = useTranslation(['guidelines', 'posts'])
  if (isOrphanedScope(guideline)) {
    return (
      <div>
        <Badge tone="warning">{t('scope.orphaned', { ns: 'guidelines' })}</Badge>
        <Typography variant="body" as="p" className="text-content-secondary mt-1">
          {t(guideline.kind === 'clip' ? 'scope.clipOrphanedHelp' : 'scope.orphanedHelp', {
            ns: 'guidelines',
          })}
        </Typography>
      </div>
    )
  }
  if (guideline.scope === 'global') {
    return <Badge tone="neutral">{t('scope.global', { ns: 'guidelines' })}</Badge>
  }
  if (guideline.scope === 'fields') {
    // In catalogue order, whatever order the set arrived in; the names are the posts namespace's
    // own, never retyped here.
    return (
      <div className="flex flex-wrap gap-1">
        {BLOG_FIELD_IDS.filter((field) => guideline.fields.includes(field)).map((field) => (
          <Badge key={field} tone="neutral">
            {t(blogFieldLabelKey(field), { ns: 'posts' })}
          </Badge>
        ))}
      </div>
    )
  }
  return (
    <div className="flex flex-wrap gap-1">
      {guideline.templates.map((template) => (
        <Badge key={template.id} tone="neutral">
          {template.name}
        </Badge>
      ))}
    </div>
  )
}

/** The one badge a closed row carries (GUIDE-20): the scope's kind, not its members — the names
 *  are for the open row. `적용 대상 없음` keeps its warning tone, because a rule that reaches
 *  nothing is the one thing worth seeing without opening it. */
export function GuidelineScopeBadge({ guideline }: { guideline: ScopeView }) {
  const { t } = useTranslation('guidelines')
  if (isOrphanedScope(guideline)) return <Badge tone="warning">{t('scope.orphaned')}</Badge>
  const label =
    guideline.scope === 'global'
      ? t('scope.global')
      : guideline.scope === 'fields'
        ? t('scope.fieldsShort')
        : t(guideline.kind === 'clip' ? 'scope.videoTemplatesShort' : 'scope.templatesShort')
  return <Badge tone="neutral">{label}</Badge>
}
