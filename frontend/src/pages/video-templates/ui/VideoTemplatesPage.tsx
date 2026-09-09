import { Link, useLocation } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipTemplates } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import {
  ActionBar,
  Button,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

export function VideoTemplatesPage() {
  const { t, i18n } = useTranslation(['clips', 'common'])
  const { user } = useSession()
  const query = useClipTemplates(user?.id ?? '')
  const detachedCount = useLocation({ select: (location) => location.state.clipDetachedCount })
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col gap-8' })}>
      <header>
        <Typography variant="display">{t('directory.title', { ns: 'clips' })}</Typography>
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('directory.description', { ns: 'clips' })}
        </Typography>
      </header>
      {detachedCount !== undefined && (
        <Typography variant="body" role="status">
          {t('directory.deleted', { ns: 'clips', count: detachedCount })}
        </Typography>
      )}
      <section aria-label={t('directory.saved', { ns: 'clips' })}>
        {query.isPending ? (
          <Typography variant="body" role="status" className="text-content-tertiary">
            {t('state.loading', { ns: 'common' })}
          </Typography>
        ) : query.isError ? (
          <div role="alert">
            <Typography variant="body">{t('editor.loadFailed', { ns: 'clips' })}</Typography>
            <Button variant="ghost" onClick={() => void query.refetch()}>
              {t('action.retry', { ns: 'common' })}
            </Button>
          </div>
        ) : query.templates.length === 0 ? (
          <Typography variant="body" className="text-content-tertiary">
            {t('directory.empty', { ns: 'clips' })}
          </Typography>
        ) : (
          <ul className="-mx-4 sm:-mx-6 lg:-mx-8">
            {query.templates.map((template) => (
              <li key={template.id}>
                <Link
                  to="/video-templates/$templateId"
                  params={{ templateId: template.id }}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-w-0 flex-col gap-2 px-4 py-3 sm:px-6 lg:px-8"
                >
                  <span
                    className={typographyStyles({ variant: 'label', className: 'break-words' })}
                  >
                    {template.name}
                  </span>
                  <Typography variant="meta">
                    {t('directory.fields', {
                      ns: 'clips',
                      count: template.informationFields.length,
                    })}
                  </Typography>
                  <Typography variant="meta">
                    {template.copyStyles
                      .map((style) => t(`style.${style}`, { ns: 'clips' }))
                      .join(' · ')}
                  </Typography>
                  <Typography variant="meta">
                    {t('directory.updated', {
                      ns: 'clips',
                      date: new Date(template.updatedAt).toLocaleDateString(i18n.language),
                    })}
                  </Typography>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </section>
      <ActionBar className="mt-auto sm:ml-auto">
        <Link
          to="/video-templates/new"
          className={buttonStyles({ variant: 'cta', className: 'w-full sm:w-auto' })}
        >
          {t('directory.create', { ns: 'clips' })}
        </Link>
      </ActionBar>
    </main>
  )
}
