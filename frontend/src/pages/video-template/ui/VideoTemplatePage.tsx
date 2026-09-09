import { Link, useParams } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipTemplates } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { ClipTemplateEditor } from '@/features/edit-clip-template'
import { Button, Typography, pageStyles, typographyStyles } from '@/shared/ui'

export function VideoTemplatePage() {
  const { t } = useTranslation(['clips', 'common'])
  const { templateId } = useParams({ strict: false })
  const { user } = useSession()
  const query = useClipTemplates(user?.id ?? '')
  const stored = query.templates.find((value) => value.id === templateId)
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col' })}>
      <Link
        to="/video-templates"
        className={typographyStyles({
          variant: 'label',
          className:
            'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center px-3 underline',
        })}
      >
        {t('editor.back', { ns: 'clips' })}
      </Link>
      <Typography variant="display" className="mt-4">
        {t(templateId ? 'editor.edit' : 'editor.create', { ns: 'clips' })}
      </Typography>
      {query.isError ? (
        <div role="alert" className="mt-6">
          <Typography variant="body">{t('editor.loadFailed', { ns: 'clips' })}</Typography>
          <Button variant="ghost" onClick={() => void query.refetch()}>
            {t('action.retry', { ns: 'common' })}
          </Button>
        </div>
      ) : templateId && (query.isPending || (!stored && query.isFetching)) ? (
        <Typography variant="body" role="status" className="text-content-tertiary mt-6">
          {t('state.loading', { ns: 'common' })}
        </Typography>
      ) : templateId && !stored ? (
        <Typography variant="body" role="alert" className="mt-6">
          {t('editor.notFound', { ns: 'clips' })}
        </Typography>
      ) : (
        <ClipTemplateEditor key={templateId ?? 'new'} ownerId={user?.id ?? ''} stored={stored} />
      )}
    </main>
  )
}
