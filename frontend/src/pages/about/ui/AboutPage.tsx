import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Eye, Layers, PencilLine, SlidersHorizontal } from 'lucide-react'
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
          <h1 id="about-hero" className="text-3xl font-semibold tracking-tight sm:text-4xl">
            {t('hero.title')}
          </h1>
          <p className="text-content-secondary max-w-measure mt-4 text-base leading-relaxed">
            {t('hero.body')}
          </p>
          <p className="text-content-tertiary max-w-measure mt-3 text-sm leading-relaxed">
            {t('hero.access')}
          </p>
        </section>

        <Section id="about-flow" title={t('flow.title')}>
          {/* A real ordered list: the four steps happen in this order, and that is semantic
              information a screen reader has to get from the markup, not from the numerals. */}
          <ol className="mt-4 space-y-5">
            {(['step1', 'step2', 'step3', 'step4'] as const).map((step, index) => (
              <li key={step} className="flex gap-3">
                {/* Decorative: the ordinal is already in the list semantics, so it is hidden
                    rather than read out twice. */}
                <span
                  aria-hidden="true"
                  className="bg-surface-raised text-content-secondary mt-0.5 inline-flex size-7 shrink-0 items-center justify-center rounded-full text-sm font-medium"
                >
                  {index + 1}
                </span>
                <div className="min-w-0">
                  <h3 className="text-base font-medium">{t(`flow.${step}.title`)}</h3>
                  <p className="text-content-secondary max-w-measure mt-1 text-sm leading-relaxed">
                    {t(`flow.${step}.body`)}
                  </p>
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
                <h3 className="flex items-center gap-2 text-base font-medium">
                  <Icon aria-hidden="true" className="text-content-tertiary size-4 shrink-0" />
                  <span className="min-w-0">{t(`different.${key}.title`)}</span>
                </h3>
                <p className="text-content-secondary mt-1 text-sm leading-relaxed">
                  {t(`different.${key}.body`)}
                </p>
              </li>
            ))}
          </ul>
        </Section>

        <Section id="about-outputs" title={t('outputs.title')}>
          <p className="text-content-secondary max-w-measure mt-3 text-sm leading-relaxed">
            {t('outputs.body')}
          </p>
          <ul className="text-content-primary mt-4 flex flex-wrap gap-2">
            {(['naver', 'tistory', 'html', 'markdown'] as const).map((format) => (
              <li
                key={format}
                className="bg-surface-raised rounded-md px-3 py-1.5 text-sm whitespace-nowrap"
              >
                {t(`outputs.${format}`)}
              </li>
            ))}
          </ul>
          {/* The publishing boundary, stated rather than marketed: an operator-tier surface whose
              live verification is still open (plan 12). Never softened into a shipped feature. */}
          <p className="text-content-tertiary max-w-measure mt-4 text-sm leading-relaxed">
            {t('outputs.publishing')}
          </p>
        </Section>

        <Section id="about-plans" title={t('plans.title')}>
          <p className="text-content-secondary max-w-measure mt-3 text-sm leading-relaxed">
            {t('plans.body')}
          </p>
          {/* Scrolls inside its own container: five columns of Korean headers do not fit 320px,
              and the page itself must never scroll sideways (design-language §1.5). */}
          <div className="-mx-4 mt-4 overflow-x-auto overscroll-x-contain px-4 sm:mx-0 sm:px-0">
            <table className="w-full min-w-md text-left text-sm">
              <caption className="sr-only">{t('plans.title')}</caption>
              <thead className="text-content-tertiary text-xs">
                <tr>
                  <th scope="col" className="py-2 pr-4 font-medium">
                    {t('plans.columns.plan')}
                  </th>
                  <th scope="col" className="py-2 pr-4 font-medium">
                    {t('plans.columns.dailyStarts')}
                  </th>
                  <th scope="col" className="py-2 pr-4 font-medium">
                    {t('plans.columns.dailyBudget')}
                  </th>
                  <th scope="col" className="py-2 pr-4 font-medium">
                    {t('plans.columns.monthlyBudget')}
                  </th>
                  <th scope="col" className="py-2 font-medium">
                    {t('plans.columns.models')}
                  </th>
                </tr>
              </thead>
              <tbody className="divide-divider divide-y">
                {(['free', 'basic', 'max'] as const).map((tier) => (
                  <tr key={tier}>
                    <th scope="row" className="py-3 pr-4 font-mono text-sm font-medium">
                      {t(`plans.${tier}.name`)}
                    </th>
                    <td className="text-content-secondary py-3 pr-4 whitespace-nowrap">
                      {t(`plans.${tier}.dailyStarts`)}
                    </td>
                    <td className="text-content-secondary py-3 pr-4 whitespace-nowrap">
                      {t(`plans.${tier}.dailyBudget`)}
                    </td>
                    <td className="text-content-secondary py-3 pr-4 whitespace-nowrap">
                      {t(`plans.${tier}.monthlyBudget`)}
                    </td>
                    <td className="text-content-secondary py-3">{t(`plans.${tier}.models`)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <p className="text-content-secondary max-w-measure mt-4 text-sm leading-relaxed">
            {t('plans.assignment')}
          </p>
          <p className="text-content-tertiary max-w-measure mt-2 text-sm leading-relaxed">
            {t('plans.master')}
          </p>
        </Section>

        <Section id="about-facts" title={t('facts.title')}>
          {/* No icons here, unlike the section above: these are four unrelated facts and any icon
              set would be decoration standing in for meaning. The separator is the divider the
              rest of the app uses for a plain fact list. */}
          <ul className="divide-divider mt-3 divide-y">
            {(['images', 'isolation', 'noBackground', 'credentials'] as const).map((fact) => (
              <li
                key={fact}
                className="text-content-secondary max-w-measure py-3 text-sm leading-relaxed"
              >
                {t(`facts.${fact}`)}
              </li>
            ))}
          </ul>
        </Section>
      </main>
      {/* Identity only. No second CTA, no contact collection, no legal claim (plan 15). */}
      {/* `pb-8 mb-safe-b`, not `pb-8 pb-safe-b`: two padding utilities on the same side collide and
          the later one in the emitted CSS wins, which would resolve the footer's bottom padding to
          the bare inset — 0 on every desktop browser (app/styles/index.css). Margin adds instead. */}
      <footer className="text-content-tertiary mb-safe-b px-4 pb-8 text-sm sm:px-6">
        <p>Postpilot — {t('footer.tagline')}</p>
      </footer>
    </div>
  )
}

function Section({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={id} className="mt-14 sm:mt-20">
      <h2 id={id} className="text-xl font-semibold tracking-tight">
        {title}
      </h2>
      {children}
    </section>
  )
}
