import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Eye, Layers, PencilLine, SlidersHorizontal } from 'lucide-react'
import { Typography, typographyStyles } from '@/shared/ui'
import { useAboutMetadata } from '../model/useAboutMetadata'
import { AboutHeader } from './AboutHeader'

/** The public explanation of Postpilot (plan 15). Composition and copy keys only.
 *
 *  It reads nothing and writes nothing: no session probe, no query, no mutation, no provider call
 *  ([I5]) — which is also why it is a direct child of the root route rather than of the
 *  authenticated layout. The plans section is static localized copy, deliberately not a
 *  `GetMyPlan` read: a visitor with no account has no plan to read, and the ladder is a product
 *  fact, not this visitor's state.
 *
 *  Sections are separated by spacing and one surface step, never by bordered card stacks
 *  (design-language §1.3/§1.4). */
export function AboutPage() {
  const { t } = useTranslation('marketing')
  useAboutMetadata()

  return (
    <div className="bg-surface-base text-content-primary flex min-h-full flex-col">
      <AboutHeader />
      <main className="mx-auto w-full max-w-3xl flex-1 px-4 pb-16 sm:px-6">
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
          {/* Scrolls inside its own container: five columns of Korean headers do not fit 320px,
              and the page itself must never scroll sideways (design-language §1.5). */}
          <div className="-mx-4 mt-4 overflow-x-auto overscroll-x-contain px-4 sm:mx-0 sm:px-0">
            {/* The label role carries the table's 14px; every visible cell sets its own colour. */}
            <table
              className={typographyStyles({
                variant: 'label',
                className: 'w-full min-w-md text-left',
              })}
            >
              <caption className="sr-only">{t('plans.title')}</caption>
              <thead className={typographyStyles({ variant: 'label' })}>
                <tr>
                  <th scope="col" className="py-2 pr-4">
                    {t('plans.columns.plan')}
                  </th>
                  <th scope="col" className="py-2 pr-4">
                    {t('plans.columns.monthlyCredits')}
                  </th>
                  <th scope="col" className="py-2 pr-4">
                    {t('plans.columns.price')}
                  </th>
                  <th scope="col" className="py-2">
                    {t('plans.columns.models')}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-divider divide-y">
                {(['free', 'basic', 'pro', 'max'] as const).map((tier) => (
                  <tr key={tier}>
                    <th
                      scope="row"
                      className={typographyStyles({
                        variant: 'label',
                        mono: true,
                        className: 'text-content-primary py-3 pr-4',
                      })}
                    >
                      {t(`plans.${tier}.name`)}
                    </th>
                    <td className="text-content-secondary py-3 pr-4 whitespace-nowrap">
                      {t(`plans.${tier}.monthlyCredits`)}
                    </td>
                    <td className="text-content-secondary py-3 pr-4 whitespace-nowrap">
                      {t(`plans.${tier}.price`)}
                    </td>
                    <td className="text-content-secondary py-3">{t(`plans.${tier}.models`)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Typography variant="body" className="text-content-secondary max-w-measure mt-4">
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
