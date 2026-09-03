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
    login: '로그인',
    access: /가입 절차는 없습니다/,
    steps: [
      '말투·용도·출력 언어를 고릅니다',
      '사진과 메모를 올립니다',
      '사진을 먼저 관찰하고, 그다음 씁니다',
      '고치고 확정한 뒤 내보냅니다',
    ],
    formats: ['네이버 블로그용', '티스토리용', '개인 사이트용 HTML', '마크다운'],
    publishing: /실제 환경 검증이 진행 중/,
    assignment: /요금제는 운영자가 계정에 지정합니다/,
    master: /사용자가 받을 수 있는 등급이 아닙니다/,
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
    login: 'Log in',
    access: /There is no signup/,
    steps: [
      'Pick a voice, a purpose, and the output language',
      'Add photos and rough notes',
      'Observe the photos first, then write',
      'Revise, finalize, and export',
    ],
    formats: ['Naver Blog', 'Tistory', 'HTML for your own site', 'Markdown'],
    publishing: /live verification is still in progress/,
    assignment: /Plans are assigned to an account by the operator/,
    master: /not a tier a user can be given/,
    facts: /Opening a screen never starts AI work/,
  },
} as const

/** The shipped ladder, copied from `backend/internal/plan`'s limits table by way of
 *  spec/policy/plans.md. A divergence between this table and the page is a copy bug — the whole
 *  point of A17 — so the test states the numbers rather than reading them from the catalog. */
const PLANS = [
  { name: 'free', credits: '50', price: /무료|Free/ },
  { name: 'basic', credits: '200', price: '$2' },
  { name: 'pro', credits: '500', price: '$5' },
  { name: 'max', credits: '1,000', price: '$10' },
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

  // A17: the tier values equal plan 17's shipped limits table, master is operator-only, plans are
  // operator-assigned, and there is no commercial affordance anywhere.
  it('presents exactly the shipped plan ladder with no purchase affordance', async () => {
    render()
    const plans = within(await screen.findByRole('region', { name: copy.sections[3] }))

    for (const tier of PLANS) {
      const row = plans.getByRole('row', { name: new RegExp(`^${tier.name}\\b`) })
      const cells = within(row).getAllByRole('cell')
      expect(cells[0]).toHaveTextContent(tier.credits)
      expect(cells[1]).toHaveTextContent(tier.price)
    }
    // master appears only as prose about the operator tier — never as a fourth obtainable row.
    expect(plans.getAllByRole('row')).toHaveLength(PLANS.length + 1)
    expect(plans.getByText(copy.master)).toBeInTheDocument()
    expect(plans.getByText(copy.assignment)).toBeInTheDocument()
    expect(plans.queryByRole('button')).not.toBeInTheDocument()
    expect(plans.queryByRole('link')).not.toBeInTheDocument()
  })

  // A5/A11: Login is the page's only product CTA, and nothing here collects anything.
  it('offers Login as the only product action and no form or third-party asset', async () => {
    const { container } = render()
    await screen.findByRole('heading', { level: 1 })

    const productLinks = screen
      .getAllByRole('link')
      .filter((link) => link.getAttribute('href')?.startsWith('http') === false)
    expect(productLinks).toHaveLength(1)
    expect(productLinks[0]).toHaveAccessibleName(copy.login)
    expect(productLinks[0]).toHaveAttribute('href', '/login')

    // No signup/contact/waitlist/purchase surface of any kind.
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
 *  the one element too wide for 320px scrolls inside itself, the page has no second vertical
 *  scroller, every header control keeps its 44px floor, and edge-anchored chrome is inset-padded. */
describe('the About page layout invariants', () => {
  beforeEach(() => {
    initializeI18n('ko')
  })

  it('keeps the wide plans table inside its own horizontal scroller', async () => {
    const { container } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    const table = container.querySelector('table')
    expect(table).not.toBeNull()
    const scroller = table?.parentElement
    // The table is the only thing on the page wider than 320px. It scrolls itself, so the page
    // body never does (design-language §1.5).
    expect(scroller?.className).toContain('overflow-x-auto')
    expect(scroller?.className).toContain('overscroll-x-contain')
    for (const element of container.querySelectorAll('*')) {
      expect(element.className.toString()).not.toContain('overflow-y-auto')
    }
  })

  it('keeps the header controls at the 44px floor and pads the safe areas', async () => {
    const { container } = renderAppAt('/about')
    await screen.findByRole('heading', { level: 1 })

    const header = container.querySelector('header')
    expect(header?.className).toContain('pt-safe-t')
    expect(header?.className).toContain('sticky')
    const login = screen.getByRole('link', { name: '로그인' })
    expect(login.className).toContain('min-h-11')
    for (const name of ['테마', '언어']) {
      expect(screen.getByRole('button', { name }).className).toMatch(/min-h-11|size-11/)
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

    // The wordmark is not a link on its own page. Login comes first, then the two menus stay at
    // the viewport edge so their right-aligned panels cannot cross the 320px left edge.
    await user.tab()
    expect(screen.getByRole('link', { name: '로그인' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '테마' })).toHaveFocus()
    await user.tab()
    expect(screen.getByRole('button', { name: '언어' })).toHaveFocus()
  })
})
