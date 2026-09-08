import type { ReactNode } from 'react'
import { Link, useSearch } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { Eye, Layers, PencilLine, SlidersHorizontal } from 'lucide-react'
import { isInAppPath } from '@/shared/lib'
import { PromoStage, Typography, typographyStyles } from '@/shared/ui'
import { PlanLadder } from '@/widgets/plan-ladder'
import { PUBLIC_LADDER } from '../model/ladder'
import { useAboutMetadata } from '../model/useAboutMetadata'
import { AboutHeader } from './AboutHeader'

/** The public explanation of Postpilot (plan 15). Composition and copy keys only.
 *
 *  It reads nothing and writes nothing: no session probe, no query, no mutation, no provider call
 *  ([I5]) — which is also why it is a direct child of the root route rather than of the
 *  authenticated layout. The plans section is the same promotional ladder `/plans` shows, fed
 *  from a static code-owned table rather than a `GetMyPlan` read (MARKETING-5): a visitor with
 *  no account has no plan to read, and the ladder is a product fact, not this visitor's state.
 *
 *  Sections are separated by spacing and one surface step, never by bordered card stacks
 *  (design-language §1.3/§1.4) — the plan cards being the one promotional exception (THEME-37). */
export function AboutPage() {
  const { t } = useTranslation('marketing')
  useAboutMetadata()
  // Handed straight back to /login so a detour through this page does not cost the visitor the
  // destination their session expired on. Filtered here as well as there: an off-site value must
  // never survive a round trip through a public page (MARKETING-2).
  const { redirect } = useSearch({ from: '/about' })
  const carried = isInAppPath(redirect) ? redirect : undefined

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <AboutHeader />
      <main className="mx-auto w-full max-w-3xl flex-1 px-4 pb-16 sm:px-6 lg:max-w-5xl lg:px-8">
        <section aria-labelledby="about-hero" className="pt-10 sm:pt-16">
          <Typography variant="display" id="about-hero">
            {t('hero.title')}
          </Typography>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-4">
            {t('hero.body')}
          </Typography>
          <Typography variant="body" className="text-content-tertiary max-w-measure mt-3">
            {t('hero.access')}
          </Typography>
          {/* The quiet way in for someone who already has an account, where the account path is
              being explained rather than as a second control in the header (MARKETING-6). A bare
              text link keeps its 44px box at every pointer — nothing visible is oversized. */}
          <Typography
            variant="body"
            className="text-content-secondary mt-1 flex flex-wrap items-center gap-x-2"
          >
            <span>{t('hero.haveAccount')}</span>
            <Link
              to="/login"
              search={carried ? { redirect: carried } : {}}
              className={typographyStyles({
                variant: 'label',
                className:
                  'text-link-fg hover:text-link-fg-hover inline-flex min-h-11 items-center underline',
              })}
            >
              {t('header.login')}
            </Link>
          </Typography>
        </section>

        <Section id="about-flow" title={t('flow.title')}>
          {/* A real ordered list: the four steps happen in this order, and that is semantic
              information a screen reader has to get from the markup, not from the numerals. */}
          <ol className="mt-4 space-y-5">
            {(['step1', 'step2', 'step3', 'step4'] as const).map((step, index) => (
              <li key={step} className="flex gap-3">
                {/* Decorative: the ordinal is already in the list semantics, so it is hidden
                    rather than read out twice. */}
                <Typography
                  variant="label"
                  aria-hidden="true"
                  className="bg-surface-raised mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-full"
                >
                  {index + 1}
                </Typography>
                <div className="min-w-0">
                  <Typography variant="label" as="h3">
                    {t(`flow.${step}.title`)}
                  </Typography>
                  <Typography variant="body" className="text-content-secondary max-w-measure mt-1">
                    {t(`flow.${step}.body`)}
                  </Typography>
                </div>
              </li>
            ))}
          </ol>
        </Section>

        <Section id="about-different" title={t('different.title')}>
          <ul className="mt-4 grid gap-5 sm:grid-cols-2">
            {(
              [
                ['voices', Layers],
                ['observation', Eye],
                ['blocks', PencilLine],
                ['control', SlidersHorizontal],
              ] as const
            ).map(([key, Icon]) => (
              <li key={key} className="min-w-0">
                <Typography variant="label" as="h3" className="flex items-center gap-2">
                  <Icon aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
                  <span className="min-w-0">{t(`different.${key}.title`)}</span>
                </Typography>
                <Typography variant="body" className="text-content-secondary mt-1">
                  {t(`different.${key}.body`)}
                </Typography>
              </li>
            ))}
          </ul>
        </Section>

        <Section id="about-outputs" title={t('outputs.title')}>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-3">
            {t('outputs.body')}
          </Typography>
          <ul className="text-content-primary mt-4 flex flex-wrap gap-2">
            {(['naver', 'tistory', 'html', 'markdown'] as const).map((format) => (
              <Typography
                variant="label"
                as="li"
                key={format}
                className="bg-surface-raised text-content-primary rounded-md px-3 py-1.5 whitespace-nowrap"
              >
                {t(`outputs.${format}`)}
              </Typography>
            ))}
          </ul>
          {/* The publishing boundary, stated rather than marketed: an operator-tier surface whose
              live verification is still open (plan 12). Never softened into a shipped feature. */}
          <Typography variant="body" className="text-content-tertiary max-w-measure mt-4">
            {t('outputs.publishing')}
          </Typography>
        </Section>

        <Section id="about-plans" title={t('plans.title')}>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-3">
            {t('plans.body')}
          </Typography>
          {/* The same promotional cards `/plans` shows, on their stage (THEME-37, MARKETING-5):
              static figures, the code-owned recommended mark, no action on any card — plans are
              presented here, never sold (MARKETING-6). The section title is this page's `h2`, so
              the tier names take `h3`. */}
          <PromoStage className="mt-5">
            <PlanLadder offers={PUBLIC_LADDER} headingLevel="h3" />
          </PromoStage>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-5">
            {t('plans.assignment')}
          </Typography>
          <Typography variant="body" className="text-content-tertiary max-w-measure mt-2">
            {t('plans.master')}
          </Typography>
        </Section>

        <Section id="about-facts" title={t('facts.title')}>
          {/* No icons here, unlike the section above: these are four unrelated facts and any icon
              set would be decoration standing in for meaning. The separator is the divider the
              rest of the app uses for a plain fact list. */}
          <ul className="divide-divider mt-3 divide-y">
            {(['images', 'isolation', 'noBackground', 'credentials'] as const).map((fact) => (
              <Typography
                variant="body"
                as="li"
                key={fact}
                className="text-content-secondary max-w-measure py-3"
              >
                {t(`facts.${fact}`)}
              </Typography>
            ))}
          </ul>
        </Section>
      </main>
      {/* Identity only. No second CTA, no contact collection, no legal claim (plan 15). */}
      {/* `pb-8 mb-safe-b`, not `pb-8 pb-safe-b`: two padding utilities on the same side collide and
          the later one in the emitted CSS wins, which would resolve the footer's bottom padding to
          the bare inset — 0 on every desktop browser (app/styles/index.css). Margin adds instead. */}
      <footer
        className={typographyStyles({
          variant: 'body',
          className: 'text-content-tertiary mb-safe-b px-4 pb-8 sm:px-6',
        })}
      >
        <p>Postpilot · {t('footer.tagline')}</p>
      </footer>
    </div>
  )
}

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="mt-14 sm:mt-20">
      <Typography variant="title" id={id}>
        {title}
      </Typography>
      {children}
    </section>
  )
}
