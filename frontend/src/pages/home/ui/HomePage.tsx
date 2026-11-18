import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { ArrowRight } from 'lucide-react'
import { useSession } from '@/entities/session'
import {
  KIT_HASH,
  KIT_NEXT_HREF,
  KitRing,
  WritingKit,
  useKitText,
  useWritingKit,
} from '@/widgets/writing-kit'
import { Button, Typography, pageStyles, typographyStyles } from '@/shared/ui'

/** Guide: 홈 — "고품질의 글을, 내 언어로." over the owner's writing kit drawn as one picture.
 *  The icon in its middle starts the screen's 글쓰기, or 영상 만들기 once its name has switched it;
 *  each piece's ring opens the screen that makes it ready, and a piece's name changes its default
 *  right here. Under the picture, on its own axis: how much of the kit is there, and the ONE thing
 *  to do next — or, once everything is, that nothing is left but to write.
 *
 *  A new account lands here too, on the empty kit: the picture and its next step are the
 *  onboarding, so no setup gate stands in front of them (setup stays one row away in 설정). */
export function HomePage() {
  const { t } = useTranslation(['creation', 'common'])
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

      {/* On a phone and a tablet it stands on the picture's own axis, as wide as the picture; on
          the desk it is the left column's, under the title it continues. */}
      <section
        aria-label={t('home.nextAria')}
        className="mx-auto flex w-full max-w-md flex-col gap-3 lg:col-start-1 lg:row-start-2 lg:mx-0"
      >
        <div className="flex flex-wrap items-center justify-center gap-x-2 text-center lg:justify-start lg:text-left">
          <Typography variant="body" as="p" role="status" className="text-content-secondary">
            {kit.summary.loading
              ? ' '
              : kit.summary.failed
                ? t('kit.loadFailed')
                : kit.summary.complete
                  ? t('kit.allReady')
                  : t('kit.progress', { count: kit.summary.ready, total: kit.summary.total })}
          </Typography>
          {!kit.summary.loading && kit.summary.failed && (
            <Button variant="ghost" onClick={kit.retry} pending={kit.retrying}>
              {t('action.retry', { ns: 'common' })}
            </Button>
          )}
        </div>
        {/* The next step's place is held while the kit loads, so nothing below it jumps. */}
        {kit.summary.loading && (
          <span
            aria-hidden="true"
            className="loading-skeleton bg-surface-recessed block h-15 w-full rounded-2xl"
          />
        )}
        {!kit.summary.loading && kit.next && (
          <Link
            to={KIT_NEXT_HREF[kit.next.key] ?? kit.next.href}
            // Its way back is 홈, where it was started, even where it opens past the kit screen;
            // the writing AI's place is a tab of its own, which needs no way back.
            search={kit.next.key === 'ai' ? {} : { entry: '/' }}
            hash={KIT_HASH[kit.next.key]}
            className="bg-card-bg hover:bg-card-bg-hover active:bg-card-bg-active flex w-full min-w-0 items-center gap-3 rounded-2xl py-2 pr-4 pl-2 text-left shadow-sm"
          >
            <KitRing kitKey={kit.next.key} state={kit.next.state} fill={kit.next.fill} size="sm" />
            <span className="flex min-w-0 flex-1 flex-col">
              <span className={typographyStyles({ variant: 'meta' })}>{t('kit.nextLabel')}</span>
              <span
                className={typographyStyles({
                  variant: 'label',
                  className: 'text-content-primary text-pretty break-keep',
                })}
              >
                {text.next(kit.next)}
              </span>
            </span>
            <ArrowRight aria-hidden="true" className="text-link-fg size-5 shrink-0" />
          </Link>
        )}
      </section>
    </main>
  )
}
