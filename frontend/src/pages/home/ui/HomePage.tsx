import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowRight, Clapperboard } from 'lucide-react'
import { useSession } from '@/entities/session'
import { CLIP_LIST_PATH, rememberClipEntry } from '@/entities/clip-project'
import { KitRing, WritingKit, useKitText, useWritingKit } from '@/widgets/writing-kit'
import { Typography, buttonStyles, pageStyles, typographyStyles } from '@/shared/ui'

/** Guide: 홈 — "고품질의 글을, 내 언어로." over the owner's writing kit drawn as one picture.
 *  The pen in its middle is the screen's 글쓰기; each piece around it opens the screen that makes
 *  it ready. Beside the picture: how much of the kit is there, and the ONE thing to do next — or,
 *  once everything is, that nothing is left but to write. 영상 만들기 stays a quiet second way in.
 *
 *  A new account lands here too, on the empty kit: the picture and its next step are the
 *  onboarding, so no setup gate stands in front of them (setup stays one row away in 설정). */
export function HomePage() {
  const { t } = useTranslation('creation')
  const { user } = useSession()
  const ownerId = user?.id ?? ''
  const kit = useWritingKit(ownerId)
  const text = useKitText()
  return (
    <main
      className={pageStyles({
        width: 'wide',
        className:
          'grid flex-1 grid-cols-[minmax(0,1fr)] content-start gap-6 pt-4 sm:pt-10 lg:grid-cols-[minmax(0,1fr)_minmax(0,32rem)] lg:grid-rows-[auto_auto] lg:content-center lg:gap-x-16 lg:gap-y-8 lg:pt-8 lg:pb-28',
      })}
    >
      <header className="flex flex-col gap-2 lg:self-end">
        <Typography variant="eyebrow" as="p">
          {t('home.eyebrow')}
        </Typography>
        <h1 className={typographyStyles({ variant: 'largeTitle', className: 'text-balance' })}>
          <span className="block">{t('home.line1')} </span>
          <span className="text-badge-accent-fg block">{t('home.line2')}</span>
        </h1>
      </header>

      <WritingKit
        elements={kit.ordered}
        fill={kit.summary.fill}
        complete={kit.summary.complete}
        className="mx-auto w-full max-w-md lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:max-w-none"
      />

      <section
        aria-label={t('home.nextAria')}
        className="flex flex-col items-start gap-3 lg:col-start-1 lg:row-start-2"
      >
        <Typography variant="body" as="p" role="status" className="text-content-secondary">
          {kit.summary.loading
            ? ' '
            : kit.summary.complete
              ? t('kit.allReady')
              : t('kit.progress', { count: kit.summary.ready, total: kit.summary.total })}
        </Typography>
        {!kit.summary.loading && kit.next && (
          <Link
            to={kit.next.href}
            className="bg-card-bg hover:bg-card-bg-hover active:bg-card-bg-active animate-rise flex w-full max-w-md min-w-0 items-center gap-3 rounded-2xl py-2 pr-4 pl-2 shadow-sm"
          >
            <KitRing kitKey={kit.next.key} state={kit.next.state} fill={kit.next.fill} size="sm" />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className={typographyStyles({ variant: 'meta' })}>{t('kit.nextLabel')}</span>
              <span
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-content-primary truncate',
                })}
              >
                {text.next(kit.next)}
              </span>
            </span>
            <ArrowRight aria-hidden="true" className="text-link-fg size-5 shrink-0" />
          </Link>
        )}
        <Link
          to="/clips/new"
          search={{}}
          onClick={() =>
            rememberClipEntry(ownerId, {
              path: CLIP_LIST_PATH,
              section: 'video',
              filters: {},
              scrollY: 0,
            })
          }
          className={buttonStyles({ variant: 'ghost', className: '-ml-3 gap-2' })}
        >
          <Clapperboard aria-hidden="true" className="size-5" />
          {t('kit.writeVideo')}
        </Link>
      </section>
    </main>
  )
}
