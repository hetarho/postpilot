import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowRight, PenLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { POST_LIST_PATH, rememberPostEntry } from '@/entities/post'
import { KIT_ELEMENTS, KitGraphic, useWritingKit, type KitElement } from '@/widgets/writing-kit'
import {
  ActionBar,
  Button,
  Typography,
  buttonStyles,
  pageStyles,
  typographyStyles,
  useNavigationContext,
  type NavigationLinkProps,
} from '@/shared/ui'

function NativeLink(props: NavigationLinkProps) {
  return <a {...props} />
}

/** Where each element's "next" step opens: straight into making one. */
const NEXT_HREF: Readonly<Record<KitElement, string>> = {
  voices: '/voices?new=1',
  templates: '/templates/new',
  guidelines: '/guidelines?new=1',
  memories: '/memories?new=1',
}

/** Guide: 홈 — the product's promise as the screen's message, the owner's writing kit drawn as
 *  one graphic (KitGraphic: how ready 말투 · 템플릿 · 지침 · 기억 are, each a way into the screen
 *  that makes it ready), and the one 글쓰기. While the kit is not complete, the first element
 *  still missing is offered beside 글쓰기 — writing never waits for the kit. The home is the
 *  first-use guide itself, so it sends no one to /setup (which stays under 설정 · 시작 설정 다시). */
export function HomePage() {
  const { t } = useTranslation(['creation', 'common'])
  const AppLink = useNavigationContext()?.Link ?? NativeLink
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const kit = useWritingKit(ownerId)
  const total = KIT_ELEMENTS.length
  const next = kit.pending
    ? undefined
    : KIT_ELEMENTS.find((key) => kit.entries[key].reading.state !== 'ready')
  const status = kit.failed
    ? t('home.status.failed')
    : kit.pending
      ? t('home.status.loading')
      : kit.ready === total
        ? t('home.status.complete')
        : kit.ready === 0
          ? t('home.status.empty')
          : t('home.status.partial', { count: kit.ready, total })
  return (
    <>
      <main
        className={pageStyles({
          width: 'wide',
          className:
            'flex flex-1 flex-col gap-4 pt-4 sm:pt-10 lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)] lg:grid-rows-[auto_1fr] lg:content-center lg:gap-x-12',
        })}
      >
        <div className="animate-step-in lg:col-start-1 lg:row-start-1 lg:self-end">
          <Typography variant="eyebrow" as="p" className="text-link-fg-current">
            {t('kit.label')}
          </Typography>
          <h1
            className={typographyStyles({
              variant: 'largeTitle',
              className: 'mt-2 text-balance',
            })}
          >
            <span className="block">{t('home.promise.first')}</span>
            <span className="from-kit-voice to-kit-template block bg-gradient-to-r bg-clip-text text-transparent">
              {t('home.promise.second')}
            </span>
          </h1>
          <p
            role="status"
            className={typographyStyles({
              variant: 'body',
              className: 'text-content-secondary mt-3 lg:mt-5',
            })}
          >
            {status}
            {kit.failed && (
              <Button variant="ghost" onClick={kit.retry} className="ml-1">
                {t('action.retry', { ns: 'common' })}
              </Button>
            )}
          </p>
        </div>
        <KitGraphic
          kit={kit}
          className="mt-2 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:mt-0"
        />
        {/* The dock is the page's own child, so on a phone it rides above the tab bar the whole
            way down; on the desk it stands under the promise. */}
        <ActionBar
          dock="list"
          ariaLabel={t('home.actions')}
          className="mt-auto flex w-full flex-wrap-reverse justify-end gap-2 sm:w-fit lg:col-start-1 lg:row-start-2 lg:mt-6 lg:ml-0 lg:justify-start lg:self-start"
        >
          {next && (
            <AppLink
              href={
                next === 'voices' && kit.entries.voices.openId
                  ? `/voices/${encodeURIComponent(kit.entries.voices.openId)}`
                  : NEXT_HREF[next]
              }
              className={buttonStyles({
                variant: 'secondary',
                size: 'hero',
                className: 'min-w-0 flex-1 basis-40 whitespace-nowrap sm:flex-none sm:basis-auto',
              })}
            >
              <span className="truncate">{t(`home.next.${next}`)}</span>
              <ArrowRight aria-hidden="true" className="size-5 shrink-0" />
            </AppLink>
          )}
          <Link
            to="/posts/new"
            search={{}}
            onClick={() =>
              rememberPostEntry(ownerId, {
                path: POST_LIST_PATH,
                section: 'post',
                filters: {},
                scrollY: 0,
              })
            }
            className={buttonStyles({
              variant: 'cta',
              size: 'hero',
              className: 'min-w-0 flex-1 basis-40 whitespace-nowrap sm:flex-none sm:basis-auto',
            })}
          >
            <PenLine aria-hidden="true" className="size-6 shrink-0" />
            {t('home.write')}
          </Link>
        </ActionBar>
      </main>
    </>
  )
}
