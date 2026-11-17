import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { clsx } from 'clsx'
import { ArrowRight, PenLine } from 'lucide-react'
import { useSession } from '@/entities/session'
import { POST_LIST_PATH, rememberPostEntry } from '@/entities/post'
import {
  KIT_ELEMENTS,
  KIT_HREF,
  KitGraphic,
  useWritingKit,
  type KitElement,
} from '@/widgets/writing-kit'
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
  const next = kit.pending
    ? undefined
    : KIT_ELEMENTS.find((key) => kit.entries[key].reading.state !== 'ready')
  // A voice still being made, or a template drafted and not saved, is carried on rather than
  // started again: the step opens that voice, or the templates with the draft to resume.
  const resuming =
    (next === 'voices' || next === 'templates') && kit.entries[next].reading.state === 'partial'
      ? next
      : undefined
  return (
    <>
      <main
        className={pageStyles({
          width: 'wide',
          className:
            'flex flex-1 flex-col pt-3 sm:pt-8 lg:grid lg:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)] lg:grid-rows-[1fr_auto_auto_1fr] lg:content-center lg:gap-x-16 lg:pt-4',
        })}
      >
        <div className="animate-step-in lg:col-start-1 lg:row-start-2">
          <Typography variant="eyebrow" as="p" className="text-link-fg-current">
            {t('kit.label')}
          </Typography>
          <h1
            className={typographyStyles({
              variant: 'largeTitle',
              className: 'mt-1.5 text-balance lg:mt-3',
            })}
          >
            <span className="block">{t('home.promise.first')}</span>
            <span className="from-kit-voice via-kit-template to-kit-guideline block bg-gradient-to-r bg-clip-text pb-1 text-transparent">
              {t('home.promise.second')}
            </span>
          </h1>
          {kit.failed && (
            <p
              role="alert"
              className={typographyStyles({
                variant: 'body',
                className: 'text-content-secondary mt-2 flex flex-wrap items-center gap-x-1',
              })}
            >
              {t('home.failed')}
              <Button variant="ghost" onClick={kit.retry}>
                {t('action.retry', { ns: 'common' })}
              </Button>
            </p>
          )}
        </div>
        <KitGraphic
          kit={kit}
          className="mt-5 sm:mt-8 lg:col-start-2 lg:row-span-4 lg:row-start-1 lg:mt-0"
        />
        {/* The dock is the page's own child, so on a phone it rides above the tab bar the whole
            way down; on the desk it stands under the promise. */}
        <ActionBar
          dock="list"
          ariaLabel={t('home.actions')}
          className="mt-auto flex w-full flex-wrap-reverse justify-end gap-2 pt-4 sm:w-fit lg:static lg:col-start-1 lg:row-start-3 lg:mt-10 lg:ml-0 lg:justify-start lg:self-start lg:pt-0"
        >
          {next && (
            <AppLink
              href={
                next === 'voices' && kit.entries.voices.openId
                  ? `/voices/${encodeURIComponent(kit.entries.voices.openId)}`
                  : resuming
                    ? KIT_HREF[next]
                    : NEXT_HREF[next]
              }
              className={buttonStyles({
                variant: 'secondary',
                size: 'hero',
                className: 'min-w-0 grow-[3] basis-auto whitespace-nowrap sm:grow-0',
              })}
            >
              <span className="truncate">
                {resuming ? t(`home.resume.${resuming}`) : t(`home.next.${next}`)}
              </span>
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
              // Beside the next step it grows least, so the step's name stays whole; alone on its
              // row (the whole kit ready, or text enlarged) it spans it.
              className: clsx(
                'min-w-0 grow basis-auto whitespace-nowrap sm:grow-0',
                next && 'px-7',
              ),
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
