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
import { KitLegend } from './KitLegend'
import { KitSheet } from './KitSheet'
import { KIT_TONES } from '../model/kit-tones'

/** Guide: 홈 — the product's promise, 고품질의 글을 내 언어로, and how ready the owner's writing kit
 *  is to keep it. The promise leads; beside it (under it on a phone) the kit is drawn as the post
 *  it writes (`KitSheet`), and its key (`KitLegend`) names each element, says where it stands and
 *  opens its screen. The one action a ready owner came for — 글쓰기 — sits under the promise on the
 *  desk and floats in the thumb zone over the tab bar on a phone. Nothing here calls a model or
 *  charges: every figure is a list the kit's own screens already read.
 *
 *  Home is the readiness surface, so it no longer sends a new account through the first-run setup
 *  before showing itself: an empty kit is drawn as its empty parts, and the setup that fills
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
        : t(summary.total - summary.ready > 1 ? 'home.message.several' : 'home.message.partial', {
            next: summary.next ? t(`home.elements.${summary.next}.noun`) : '',
          })
  return (
    <main
      className={pageStyles({
        width: 'wide',
        className:
          'flex flex-1 flex-col pt-4 sm:pt-8 lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,1.08fr)] lg:grid-rows-[auto_auto_auto_1fr] lg:content-start lg:gap-x-14 lg:pt-12',
      })}
    >
      <header className="min-w-0 lg:col-start-1 lg:row-start-1 lg:pt-4">
        <Readiness data={data} />
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
                'text-link-fg hover:text-link-fg-hover mt-1 flex min-h-11 w-fit items-center gap-1',
            })}
          >
            {t('home.setup')}
            <ChevronRight aria-hidden="true" className="size-4" />
          </Link>
        )}
      </header>

      {/* The one action, right after the promise in reading (and keyboard) order. Floating over
          the tab bar on a phone, like every tab's own creation — `order-last` keeps it at the
          column's foot there, where its sticky dock belongs; resting under the promise on the
          desk, where the grid places it. */}
      <ActionBar
        dock="list"
        className="order-last mt-auto w-full pt-8 sm:w-fit lg:static lg:order-none lg:col-start-1 lg:row-start-2 lg:mt-8 lg:ml-0 lg:pt-0"
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

      <div className="mt-10 min-w-0 px-1 sm:px-6 lg:col-start-2 lg:row-span-4 lg:row-start-1 lg:mt-2 lg:px-0">
        <KitSheet data={data} />
      </div>

      <KitLegend data={data} className="mt-12 lg:col-start-1 lg:row-start-3 lg:mt-10" />

      <div className="mt-6 flex min-w-0 flex-col gap-4 lg:col-start-1 lg:row-start-4 lg:mt-6">
        <Link
          to="/ai-models"
          className={typographyStyles({
            variant: 'label',
            className:
              'hover:bg-row-bg-hover active:bg-row-bg-active text-content-secondary -ml-3 inline-flex min-h-11 w-fit max-w-full items-center gap-2 rounded-full px-3',
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
        <nav aria-labelledby="home-video-kit" className="min-w-0">
          <Typography variant="groupLabel" as="h2" id="home-video-kit">
            {t('home.video.title')}
          </Typography>
          <ul className="mt-2 flex flex-wrap gap-2">
            <li>
              <Link to="/spoken-voices" className={chipStyles({})}>
                {/* A count only once there is one: "목소리 0" reads as a broken number. */}
                {data.spokenCount
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
      </div>
    </main>
  )
}

/** The kit at a glance, above the promise: one dot per element in its own ink — filled when
 *  ready, a ring while almost, a dashed hole while empty. */
function Readiness({ data }: { data: ReturnType<typeof useWritingKit> }) {
  const { t } = useTranslation('creation')
  return (
    <p className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <span aria-hidden="true" className="flex gap-1.5">
        {data.kit.map((item) => (
          <span
            key={item.element}
            className={clsx(
              'block size-2.5 rounded-full',
              item.level === 'ready' && KIT_TONES[item.element].line,
              item.level === 'partial' && ['ring-2 ring-inset', KIT_TONES[item.element].ring],
              (item.level === 'empty' || item.level === 'failed') &&
                'border-content-tertiary border border-dashed',
              item.level === 'loading' && 'bg-divider',
            )}
          />
        ))}
      </span>
      <Typography variant="eyebrow" as="span">
        {t('home.progress', { ready: data.summary.ready, total: data.summary.total })}
      </Typography>
    </p>
  )
}
