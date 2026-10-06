import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ChevronRight } from 'lucide-react'
import { useSession } from '@/entities/session'
import { Typography, typographyStyles, pageStyles } from '@/shared/ui'
const GROUPS = [
  {
    key: 'writing',
    links: [
      ['/voices', 'voices'],
      ['/templates', 'templates'],
      ['/guidelines', 'guidelines'],
      ['/memories', 'memories'],
    ],
  },
  {
    key: 'video',
    links: [
      ['/video-templates', 'videoTemplates'],
      ['/video-guidelines', 'videoGuidelines'],
      ['/spoken-voices', 'spokenVoices'],
    ],
  },
  {
    key: 'ai',
    links: [
      ['/ai-models', 'models'],
      ['/ai-models/compare', 'compare'],
      ['/ai-models/experiments', 'history'],
      ['/ai-models/leaderboard', 'leaderboard'],
      ['/account', 'account'],
      ['/plans', 'plans'],
      ['/billing', 'billing'],
    ],
  },
] as const
export function SettingsPage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  return (
    <main className={pageStyles({ width: 'wide' })}>
      <Typography variant="display">{t('settings.title')}</Typography>
      <Typography variant="body" className="text-content-secondary mt-3">
        {t('settings.description')}
      </Typography>
      <div className="mt-10 grid gap-10 md:grid-cols-3">
        {GROUPS.map((group) => (
          <section key={group.key} aria-labelledby={`settings-${group.key}`} className="min-w-0">
            <Typography variant="title" id={`settings-${group.key}`} className="mb-4">
              {t(`settings.${group.key}`)}
            </Typography>
            <nav aria-label={t(`settings.${group.key}`)} className="flex flex-col gap-1">
              {group.links.map(([to, key]) => (
                <Link
                  to={to}
                  key={to}
                  className={typographyStyles({
                    variant: 'body',
                    className:
                      'hover:bg-row-bg-hover active:bg-row-bg-active flex min-h-11 items-center justify-between gap-3 rounded-md px-4 py-3',
                  })}
                >
                  {t(`settings.${key}`)}
                  <ChevronRight aria-hidden="true" className="size-4 shrink-0" />
                </Link>
              ))}
            </nav>
          </section>
        ))}
      </div>
      <Link
        to="/setup"
        search={{ restart: true }}
        className={typographyStyles({
          variant: 'body',
          className:
            'text-link-fg hover:text-link-fg-hover mt-10 inline-flex min-h-11 items-center px-4',
        })}
      >
        {t('setup.restart')}
      </Link>
      {user?.plan === 'master' && (
        <Link
          to="/admin"
          className={typographyStyles({
            variant: 'body',
            className:
              'text-link-fg hover:text-link-fg-hover mt-10 inline-flex min-h-11 items-center px-4',
          })}
        >
          {t('settings.admin')}
        </Link>
      )}
    </main>
  )
}
