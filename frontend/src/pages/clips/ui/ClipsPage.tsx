import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { useClipProjects } from '@/entities/clip-project'
import { useClipTemplates } from '@/entities/clip-template'
import { useSession } from '@/entities/session'
import { appFailureFromConnect } from '@/shared/api'
import {
  ActionBar,
  AppFailureMessage,
  Button,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'

export function ClipsPage() {
  const { t, i18n } = useTranslation('clips')
  const { user } = useSession()
  const projects = useClipProjects(user?.id ?? '')
  const templates = useClipTemplates(user?.id ?? '')
  return (
    <main className={pageStyles({ width: 'wide', className: 'flex flex-1 flex-col gap-8' })}>
      <header>
        <Typography variant="display">{t('title')}</Typography>
        <Typography variant="body" className="text-content-secondary mt-3">
          {t('project.description')}
        </Typography>
      </header>
      {templates.isError && !projects.isError && (
        <div role="alert">
          <AppFailureMessage failure={appFailureFromConnect(templates.error)} />
          <Button variant="ghost" onClick={() => void templates.refetch()}>
            {t('project.retry')}
          </Button>
        </div>
      )}
      <section aria-label={t('project.directory')}>
        {projects.isPending ? (
          <Typography variant="body" role="status">
            {t('project.loading')}
          </Typography>
        ) : projects.isError ? (
          <div role="alert">
            <AppFailureMessage failure={appFailureFromConnect(projects.error)} />
            <Button variant="ghost" onClick={() => void projects.refetch()}>
              {t('project.retry')}
            </Button>
          </div>
        ) : !projects.data.length ? (
          <Typography variant="body" className="text-content-tertiary">
            {t('project.empty')}
          </Typography>
        ) : (
          <ul className="-mx-4 sm:-mx-6 lg:-mx-8">
            {projects.data.map((project) => (
              <li key={project.id}>
                <Link
                  to="/clips/$clipId"
                  params={{ clipId: project.id }}
                  className="hover:bg-row-bg-hover active:bg-row-bg-active flex min-w-0 flex-col gap-2 px-4 py-3 sm:px-6 lg:px-8"
                >
                  <span
                    className={typographyStyles({ variant: 'label', className: 'break-words' })}
                  >
                    {project.title}
                  </span>
                  <Typography variant="meta">{t(`ratio.${project.ratio}`)}</Typography>
                  <Typography variant="meta">
                    {t(project.result ? 'generation.hasResult' : 'generation.noResult')}
                  </Typography>
                  <Typography variant="meta">
                    {t('project.summaryDuration', { seconds: project.targetDurationMs / 1000 })}
                  </Typography>
                  <Typography variant="meta" className="break-words">
                    {templates.templates.find((v) => v.id === project.videoTemplateId)?.name ??
                      t(
                        project.videoTemplateId
                          ? templates.isPending
                            ? 'project.templateLoading'
                            : templates.isError
                              ? 'project.templateUnavailable'
                              : 'project.detachedTemplate'
                          : 'project.detachedTemplate',
                      )}
                  </Typography>
                  <Typography variant="meta">
                    {t('directory.updated', {
                      date: new Date(project.updatedAt).toLocaleDateString(i18n.language),
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
          to="/clips/new"
          className={buttonStyles({ variant: 'cta', className: 'w-full sm:w-auto' })}
        >
          {t('project.new')}
        </Link>
      </ActionBar>
    </main>
  )
}
