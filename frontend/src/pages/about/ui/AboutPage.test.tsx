import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { initializeI18n } from '@/app/providers/i18n'
import { renderAppAt } from '@/test/app'

/** Every locale-parametrized assertion below runs against BOTH catalogs, so a section that exists
 *  in Korean and not in English is a failure rather than a silent gap. */
const COPY = {
  ko: {
    h1: '사진과 메모를 내 말투의 블로그 초안으로',
    sections: ['어떻게 쓰나요', '다른 점', '결과물은 어디로 가나요', '요금제', '통제와 데이터'],
    getStarted: '시작하기',
    login: '로그인',
    access:
      /이메일 주소와 비밀번호로 계정을 만들고, 첫 로그인 전에 메일로 주소를 인증합니다. Google 로그인도 사용할 수 있습니다/,
    steps: [
      '말투·용도·출력 언어를 고릅니다',
      '사진과 메모를 올립니다',
      '사진을 먼저 관찰하고, 그다음 씁니다',
      '고치고 확정한 뒤 내보냅니다',
    ],
    formats: ['네이버 블로그용', '티스토리용', '개인 사이트용 HTML', '마크다운'],
    publishing: /실제 환경 검증이 진행 중/,
    assignment: /플랜 화면에서 원하는 등급을 고르고 결제해 구독을 시작할 수 있습니다/,
    master: /사용자가 받을 수 있는 등급이 아닙니다/,
    recommended: '가장 합리적',
    haveAccount: '이미 계정이 있나요?',
    // Claims the product does not own (QUOTA-2, QUOTA-19): a plan decides the monthly grant
    // and nothing else — no daily job count, no spend allowance, no model range.
    unownedPlanClaims: ['하루', '일일', '사용 금액', '범위'],
    facts: /화면을 여는 것만으로는 AI 작업이 시작되지 않습니다/,
  },
  en: {
    h1: 'Photos and rough notes into a blog draft in your own voice',
    sections: [
      'How it works',
      'What is different',
      'Where the result goes',
      'Plans',
      'Control and data',
    ],
    getStarted: 'Get started',
    login: 'Log in',
    access:
      /An email address and password open an account, and you verify the address by mail before your first login. Google sign-in is also available/,
    steps: [
      'Pick a voice, a purpose, and the output language',
      'Add photos and rough notes',
      'Observe the photos first, then write',
      'Revise, finalize, and export',
    ],
    formats: ['Naver Blog', 'Tistory', 'HTML for your own site', 'Markdown'],
    publishing: /live verification is still in progress/,
    assignment: /choose a tier on the Plans screen and pay there to start a subscription/,
    master: /not a tier a user can be given/,
    recommended: 'Best value',
    haveAccount: 'Already have an account?',
    unownedPlanClaims: ['per day', 'daily', 'spend', 'range of'],
    facts: /Opening a screen never starts AI work/,
  },
} as const

/** The shipped ladder, copied from `backend/internal/plan`'s grant table. A divergence
 *  between this table and the page is a copy bug — the whole point of A17 (MARKETING-5) — so
 *  the test states the numbers rather than reading them from anywhere. */
const PLANS = [
  { name: 'Free', credits: /\b50\b/, price: /무료|Free/ },
  { name: 'Basic', credits: /\b220\b/, price: '$2' },
  { name: 'Pro', credits: /\b575\b/, price: '$5' },
  { name: 'Max', credits: /\b1200\b/, price: '$10' },
] as const

afterEach(() => {
  initializeI18n('ko')
})

describe.each(['ko', 'en'] as const)('the public About page in %s', (locale) => {
  const copy = COPY[locale]

  const render = () => {
    initializeI18n(locale)
    return renderAppAt('/about')
  }

  // A8/A12: one H1, every defined section present, and the ordered flow is a real ordered list.
  it('covers every defined section under one H1 with semantic ordered steps', async () => {
    render()

    const headings = await screen.findAllByRole('heading', { level: 1 })
    expect(headings).toHaveLength(1)
    expect(headings[0]).toHaveTextContent(copy.h1)
    for (const section of copy.sections) {
      expect(screen.getByRole('heading', { level: 2, name: section })).toBeInTheDocument()
    }
    expect(screen.getByText(copy.access)).toBeInTheDocument()
    expect(screen.getByText(copy.facts)).toBeInTheDocument()

    const flow = within(screen.getByRole('region', { name: copy.sections[0] }))
    const steps = flow.getAllByRole('listitem')
    expect(steps).toHaveLength(4)
    // An ordered list, not four divs with numerals: the order is the meaning.
    expect(steps[0].closest('ol')).not.toBeNull()
    copy.steps.forEach((step, index) => {
      expect(
        within(steps[index]).getByRole('heading', { level: 3, name: step }),
      ).toBeInTheDocument()
    })
  })

  // A8: the four export formats all come from the one canonical post.
  it('lists the four output formats', async () => {
    render()
    const outputs = within(await screen.findByRole('region', { name: copy.sections[2] }))
    for (const format of copy.formats) {
      expect(outputs.getByText(format)).toBeInTheDocument()
    }
  })

  // A9: publishing is stated as an operator-tier surface still in verification, never as shipped.
  it('states the publishing claim boundary rather than marketing it', async () => {
    render()
    const outputs = within(await screen.findByRole('region', { name: copy.sections[2] }))
    expect(outputs.getByText(copy.publishing)).toBeInTheDocument()
  })

  // A17 / MARKETING-5: the tier values equal the shipped grant table and are presented as the
  // same promotional cards `/plans` shows; master is operator-only prose, and the purchase path
  // is named without adding a second commercial control to this public explainer.
  it('presents exactly the shipped plan ladder as cards with no purchase affordance', async () => {
    render()
    const plans = within(await screen.findByRole('region', { name: copy.sections[3] }))

    const cards = plans.getAllByRole('listitem')
    expect(cards).toHaveLength(PLANS.length)
    PLANS.forEach((tier, index) => {
      const card = within(cards[index])
      // The section title is the page's h2, so the tier names step down to h3.
      expect(card.getByRole('heading', { level: 3, name: tier.name })).toBeInTheDocument()
      expect(card.getByText(tier.credits)).toBeInTheDocument()
      // The price is the card's hero figure — matched by its role, since the free tier's price
      // word is also its name.
      expect(card.getAllByText(tier.price).some((el) => el.classList.contains('text-3xl'))).toBe(
        true,
      )
    })
    // The same code-owned recommended rung `/plans` marks, and only that one (MARKETING-15).
    expect(plans.getAllByText(copy.recommended)).toHaveLength(1)
    expect(within(cards[2]).getByText(copy.recommended)).toBeInTheDocument()
    // No estimate: it needs the operator's priced combos, which a visitor never reads.
    expect(plans.queryByText(/매달 약|About \d+ posts/)).not.toBeInTheDocument()
    // master appears only as prose about the operator tier — never as a fifth card.
    expect(plans.getByText(copy.master)).toBeInTheDocument()
    expect(plans.getByText(copy.assignment)).toBeInTheDocument()
    expect(plans.queryByRole('button')).not.toBeInTheDocument()
    expect(plans.queryByRole('link')).not.toBeInTheDocument()
    // The cards are the promotional surface, on their stage (THEME-37).
    expect(
      plans.getByRole('list').closest('[class~="isolate"]')?.querySelector('[data-promo-aurora]'),
    ).not.toBeNull()
    expect(document.querySelectorAll('[data-promo-stroke]')).toHaveLength(PLANS.length)
  })

  // A17 again, as a claim rather than a layout: the section may only say what a plan
  // actually decides (MARKETING-4, MARKETING-5). This case fails on the sentence, not on
  // where it sits, so re-introducing "so many jobs a day" is caught wherever it is written.
  it('claims nothing about a plan beyond its monthly grant', async () => {
    render()
    const plans = await screen.findByRole('region', { name: copy.sections[3] })

    for (const claim of copy.unownedPlanClaims) {
      expect(plans.textContent).not.toContain(claim)
    }
  })

  // MARKETING-6/16: Get started is the one filled CTA and the header's only action; Login is a
  // quiet link in the hero and alone carries the blocked destination. The explanation itself
  // still collects nothing.
  it('offers one signup CTA, a quiet login link, and no form or third-party asset', async () => {
    initializeI18n(locale)
    const { container } = renderAppAt('/about?redirect=%2Fposts%2Fwelcome')
    await screen.findByRole('heading', { level: 1 })

    const getStarted = screen.getByRole('link', { name: copy.getStarted })
    const login = screen.getByRole('link', { name: copy.login })
    expect(getStarted).toHaveAttribute('href', '/signup')
    expect(getStarted.className).toContain('bg-button-cta-bg')
    expect(getStarted.closest('header')).not.toBeNull()
    // The one link in the header is the CTA — Login is not a second button beside it.
    expect(
      within(container.querySelector('header') as HTMLElement).getAllByRole('link'),
    ).toHaveLength(1)
    expect(login.closest('header')).toBeNull()
    expect(login.closest('section')).toBe(
      screen.getByRole('heading', { level: 1 }).closest('section'),
    )
    expect(screen.getByText(copy.haveAccount)).toBeInTheDocument()
    const loginURL = new URL(login.getAttribute('href') ?? '', 'https://postpilot.test')
    expect(loginURL.pathname).toBe('/login')
    expect(loginURL.searchParams.get('redirect')).toBe('/posts/welcome')
    expect(login.className).not.toContain('bg-button-cta-bg')
    expect(login.className).toContain('min-h-11')
    expect(container.querySelectorAll('.bg-button-cta-bg')).toHaveLength(1)

    // No signup/contact/waitlist/purchase form on this page; the CTA links to its owner.
    expect(container.querySelector('form')).toBeNull()
    expect(container.querySelector('input')).toBeNull()
    expect(container.querySelector('textarea')).toBeNull()
    // No marketing imagery, embedded media, or any remote asset.
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('picture, video, audio, iframe, embed, object')).toBeNull()
    for (const element of container.querySelectorAll('[src], [href]')) {
      const url = element.getAttribute('src') ?? element.getAttribute('href') ?? ''
      expect(url.startsWith('http')).toBe(false)
    }
    // No analytics or tag-manager script injected by the page.
    expect(container.querySelector('script')).toBeNull()
  })
})

/** The structural half of the 320px/keyboard pass (A12, A13). A physical device sweep cannot be
 *  asserted here, so the invariants that a future edit could silently break are pinned instead:
 *  the plan cards stack unprefixed so nothing is wider than 320px, the page has no second
 *  vertical scroller, every header control keeps the pointer floor, and edge-anchored chrome is
 *  inset-padded. */
describe('the About page layout invariants', () => {
  beforeEach(() => {
    initializeI18n('ko')
  })

  it('stacks the plan cards on a phone and keeps one vertical scroller', async () => {
    const { container } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    // No table any more — the ladder is cards, and a card stack is never wider than the column.
    expect(container.querySelector('table')).toBeNull()
    const ladder = within(screen.getByRole('region', { name: '요금제' })).getByRole('list')
    // Unprefixed, the grid is one column: the columns only arrive with `md:` and `lg:`.
    expect(ladder.className.split(/\s+/).filter((name) => name.startsWith('grid-cols-'))).toEqual(
      [],
    )
    expect(ladder).toHaveClass('md:grid-cols-2', 'lg:grid-cols-4')
    for (const element of container.querySelectorAll('*')) {
      expect(element.className.toString()).not.toContain('overflow-y-auto')
      expect(element.className.toString()).not.toContain('overflow-x-auto')
    }
  })

  it('keeps the header in one padded row with its controls at the pointer floor', async () => {
    const { container } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    const header = container.querySelector('header') as HTMLElement
    expect(header.className).toContain('pt-safe-t')
    expect(header.className).toContain('sticky')
    // One row, centred in the bar's height, with a gap between the wordmark and the controls —
    // never the wordmark stacked flush against the top edge over a centred pair of buttons.
    expect(header).toHaveClass('flex', 'items-center', 'justify-between', 'min-h-14', 'px-4')
    expect(header.className).not.toContain('flex-col')
    expect(header.className).not.toContain('justify-center')
    // The bare login link keeps its 44px box at every pointer; the CTA and the two icon
    // triggers rest at 40px under a mouse and 44px under a thumb (THEME-23).
    const login = screen.getByRole('link', { name: '로그인' })
    expect(login.className).toContain('min-h-11')
    const getStarted = screen.getByRole('link', { name: '시작하기' })
    expect(getStarted).toHaveClass('min-h-10', 'pointer-coarse:min-h-11')
    for (const name of ['테마', '언어']) {
      expect(screen.getByRole('button', { name })).toHaveClass('size-10', 'pointer-coarse:size-11')
    }
    // The inset is a MARGIN here: `pb-8 pb-safe-b` would collide and leave 0 on desktop.
    const footer = container.querySelector('footer')
    expect(footer?.className).toContain('mb-safe-b')
    expect(footer?.className).toContain('pb-8')
    expect(footer?.className).not.toContain('pb-safe-b')
  })

  it('puts the header controls in a reachable tab order', async () => {
    const user = userEvent.setup()
    renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    // The wordmark is not a link on its own page. The CTA comes first, then the two menus,
    // which stay viewport-side so their right-aligned panels cannot cross the 320px left edge;
    // the quiet Login link follows in the hero.
    await user.tab()
    expect(screen.getByRole('link', { name: '시작하기' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '테마' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '언어' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('link', { name: '로그인' })).toHaveFocus()
  })
})
