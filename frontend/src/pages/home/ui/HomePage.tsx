import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ChevronRight, PenLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { POST_LIST_PATH, rememberPostEntry } from '@/entities/post'
import {
  ActionBar,
  PromoText,
  Typography,
  buttonStyles,
  chipStyles,
  pageStyles,
  typographyStyles,
} from '@/shared/ui'
import { useWritingKit } from '../model/useWritingKit'
import { KitSheet } from './KitSheet'

/** Guide: 홈 — the product's promise, 고품질의 글을 내 언어로, and how ready the owner's writing kit
 *  is to keep it. It leads with the promise and a four-step readiness strip, draws the kit as the
 *  page it makes (`KitSheet`), and holds the one action a ready owner came for: 글쓰기. On a phone
 *  that action floats in the thumb zone over the tab bar; on the desk it sits under the promise,
 *  beside the sheet. Nothing here calls a model or charges: every figure is a list the kit's own
 *  screens already read.
 *
 *  Home is the readiness surface, so it no longer sends a new account through the first-run setup
 *  before showing itself: an empty kit is drawn as its empty layers, and the setup that fills
 *  several at once is offered from here. */
export function HomePage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const data = useWritingKit()
  const { summary } = data
  const message = !summary.settled
    ? t('home.message.loading')
    : summary.ready === summary.total
      ? t('home.message.ready')
      : summary.ready === 0 && data.kit.every((item) => item.level !== 'partial')
        ? t('home.message.empty')
        : t('home.message.partial', {
            next: summary.next ? t(`home.elements.${summary.next}.name`) : '',
          })
  return (
    <main
      className={pageStyles({
        width: 'board',
        className:
          'flex flex-1 flex-col pt-4 sm:pt-8 lg:grid lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] lg:grid-rows-[auto_auto_1fr] lg:content-start lg:gap-x-14 lg:pt-14',
      })}
    >
      <header className="min-w-0 lg:col-start-1 lg:row-start-1 lg:pt-6">
        <Readiness kit={data.kit} ready={summary.ready} total={summary.total} />
        <h1 className="mt-4 text-balance">
          <Typography variant="launch" as="span" className="block">
            {t('home.catchFirst')}
          </Typography>
          <PromoText variant="launch" as="span" className="block pb-1">
            {t('home.catchSecond')}
          </PromoText>
        </h1>
        <Typography
          variant="body"
          role="status"
          className="text-content-secondary max-w-measure mt-3 sm:mt-4"
        >
          {message}
        </Typography>
        {summary.settled && summary.ready === 0 && (
          <Link
            to="/setup"
            search={{ restart: false }}
            className={typographyStyles({
              variant: 'label',
              className:
                'text-link-fg hover:text-link-fg-hover mt-2 flex min-h-11 w-fit items-center gap-1',
            })}
          >
            {t('home.setup')}
            <ChevronRight aria-hidden="true" className="size-4" />
          </Link>
        )}
        <Link
          to="/ai-models"
          className={typographyStyles({
            variant: 'label',
            className:
              'hover:bg-row-bg-hover active:bg-row-bg-active text-content-secondary mt-2 -ml-3 inline-flex min-h-11 max-w-full items-center gap-2 rounded-full px-3',
          })}
        >
          <PenLine aria-hidden="true" className="size-4 shrink-0" />
          <span className="truncate">
            {data.writer.label
              ? `${t('home.writer.label')} · ${data.writer.label}`
              : data.writer.pending
                ? t('home.writer.label')
                : t('home.writer.none')}
          </span>
          <ChevronRight aria-hidden="true" className="size-4 shrink-0" />
        </Link>
      </header>

      <div className="mt-8 min-w-0 lg:col-start-2 lg:row-span-3 lg:row-start-1 lg:mt-0">
        <KitSheet data={data} />
      </div>

      <nav
        aria-labelledby="home-video-kit"
        className="mt-10 min-w-0 lg:col-start-1 lg:row-start-3 lg:mt-12 lg:self-start"
      >
        <Typography variant="groupLabel" as="h2" id="home-video-kit">
          {t('home.video.title')}
        </Typography>
        <ul className="mt-2 flex flex-wrap gap-2">
          <li>
            <Link to="/spoken-voices" className={chipStyles({})}>
              {data.spokenCount !== undefined
                ? t('home.video.spokenCount', { count: data.spokenCount })
                : t('home.video.spoken')}
            </Link>
          </li>
          <li>
            <Link to="/video-templates" className={chipStyles({})}>
              {t('home.video.templates')}
            </Link>
          </li>
          <li>
            <Link to="/video-guidelines" className={chipStyles({})}>
              {t('home.video.guidelines')}
            </Link>
          </li>
        </ul>
      </nav>

      {/* The one action. Floating over the tab bar on a phone, like every tab's own creation;
          resting under the promise on the desk. */}
      <ActionBar
        dock="list"
        className="mt-auto w-full pt-6 sm:w-fit lg:static lg:col-start-1 lg:row-start-2 lg:mt-8 lg:ml-0 lg:pt-0"
      >
        <Link
          to="/posts/new"
          search={{}}
          onClick={() =>
            rememberPostEntry(user?.id ?? '', {
              path: POST_LIST_PATH,
              section: 'post',
              filters: {},
              scrollY: 0,
            })
          }
          className={buttonStyles({ variant: 'cta', size: 'hero', className: 'w-full sm:w-auto' })}
        >
          <PenLine aria-hidden="true" className="size-5 shrink-0" />
          {t('home.write')}
        </Link>
      </ActionBar>
    </main>
  )
}

/** The kit at a glance: one segment per element, filled when ready, half when almost. */
function Readiness({
  kit,
  ready,
  total,
}: {
  kit: ReturnType<typeof useWritingKit>['kit']
  ready: number
  total: number
}) {
  const { t } = useTranslation('creation')
  return (
    <p className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <span aria-hidden="true" className="flex gap-1">
        {kit.map((item) => (
          <span
            key={item.element}
            className={clsx(
              'block h-1.5 w-7 overflow-hidden rounded-full',
              item.level === 'ready' ? 'bg-badge-done-bg' : 'bg-divider',
            )}
          >
            {item.level === 'partial' && (
              <span className="bg-notice-warning-fg block h-full w-1/2" />
            )}
          </span>
        ))}
      </span>
      <Typography variant="eyebrow" as="span">
        {t('home.progress', { ready, total })}
      </Typography>
    </p>
  )
}
