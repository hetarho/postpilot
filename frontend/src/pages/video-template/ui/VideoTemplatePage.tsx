import { Link, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipTemplates } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { ClipTemplateEditor } from '@/features/edit-clip-template'
import { Button, Notice, Typography, pageStyles, typographyStyles } from '@/shared/ui'

/** One video template, created or edited — the same screen for both, in the shape
 *  `/templates/$templateId` already has (CLIP-42): the way back, the record's own name as the
 *  heading, its fields, and one docked save. */
export function VideoTemplatePage() {
  // `strict: false` because ONE component serves both routes: `/video-templates/new` has no param,
  // and asking for it strictly there would throw rather than mean "a template that does not exist
  // yet".
  const { templateId } = useParams({ strict: false }) as { templateId?: string }
  const { t } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const { templates, isPending, isError, isFetching, refetch } = useClipTemplates(ownerId)
  const stored = templateId ? templates.find((template) => template.id === templateId) : undefined

  if (isError) {
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Notice tone="danger" role="alert" className="mt-4">
          <span>{t('directory.loadFailed', { ns: 'clips' })}</span>
          <Button
            variant="ghost"
            onClick={() => void refetch()}
            pending={isFetching}
            className="text-notice-danger-fg underline"
          >
            {t('action.retry', { ns: 'common' })}
          </Button>
        </Notice>
      </main>
    )
  }
  if (templateId && (isPending || (!stored && isFetching))) {
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Typography variant="body" role="status" className="text-content-tertiary mt-8">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      </main>
    )
  }
  // `!isFetching` matters right after a create: the screen replaces to the new id while the
  // directory invalidation is still in flight, so the row is legitimately not there yet.
  if (templateId && !stored && !isFetching) {
    return (
      <main className={pageStyles({ width: 'wide' })}>
        <BackLink />
        <Notice tone="danger" role="alert" className="mt-4">
          {t('editor.notFound', { ns: 'clips' })}
        </Notice>
      </main>
    )
  }

  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <BackLink />
      {/* The record's own name, not a fixed 영상 템플릿 편집: a directory-item form's heading IS
          its identity, and a fixed label makes two templates' screens indistinguishable. */}
      <Typography variant="display" className="mt-2 block">
        {stored ? stored.name : t('editor.create', { ns: 'clips' })}
      </Typography>
      <ClipTemplateEditor key={stored?.id ?? 'new'} ownerId={ownerId} stored={stored} />
    </main>
  )
}

function BackLink() {
  const { t } = useTranslation('clips')
  return (
    <Link
      to="/video-templates"
      className={typographyStyles({
        variant: 'label',
        className:
          'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center underline',
      })}
    >
      {t('editor.back')}
    </Link>
  )
}
