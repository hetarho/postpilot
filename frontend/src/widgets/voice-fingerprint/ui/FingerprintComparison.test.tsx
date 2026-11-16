import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import type { FingerprintComparisonItem } from '@/entities/voice'
import { FingerprintComparison } from './FingerprintComparison'

/** Three items as the server orders them: the farthest first, the unknown last (VOICE-62). */
const ITEMS: FingerprintComparisonItem[] = [
  {
    item: 'endings',
    unknown: false,
    distance: 0.9,
    headline: '해요',
    verdict: 'different',
    facets: [
      { key: '다', unit: 'share', voice: 0.05, text: 0.95 },
      { key: '해요', unit: 'share', voice: 0.92, text: 0 },
      { key: 'suffixes', unit: 'text', voice: ['더라구요'], text: [] },
    ],
  },
  {
    item: 'adverbs',
    unknown: false,
    distance: 0.5,
    headline: '진짜',
    verdict: 'similar',
    facets: [{ key: '진짜', unit: 'per_hundred', voice: 12.4, text: 0 }],
  },
  // Too rare to expect in a text this long: shown, with no verdict (VOICE-62).
  {
    item: 'person',
    unknown: false,
    distance: 0.2,
    headline: 'jeo',
    facets: [{ key: 'jeo', unit: 'share', voice: 0.3, text: 0.3 }],
  },
  { item: 'emoji', unknown: true, distance: 0, headline: '', facets: [] },
]

afterEach(() => initializeI18n('ko'))

describe('FingerprintComparison', () => {
  // VOICE-62: the server's order, each item's headline facet beside 내 말투, every facet on
  // demand, and 알 수 없음 for an item the text is too short to show.
  it('reads each item in order with its headline, all facets on demand, and unknown items', async () => {
    const user = userEvent.setup()
    render(<FingerprintComparison items={ITEMS} textLabel="이 글" />)

    const rows = screen.getAllByRole('listitem')
    // Each row starts with its item's name: a known item's opens its facets, an unknown one's
    // is plain text.
    const names = rows.map(
      (row) => within(row).queryByRole('button')?.textContent ?? row.firstChild?.textContent,
    )
    expect(names).toEqual(['문장 끝', '자주 쓰는 말', '1인칭', '이모지와 자모'])
    expect(rows[0]).toHaveTextContent("'~해요' 내 말투 92% · 이 글 0%")
    expect(rows[1]).toHaveTextContent("'진짜' 내 말투 100문장에 12번 · 이 글 100문장에 0번")
    expect(rows[3]).toHaveTextContent('이모지와 자모알 수 없음')
    expect(within(rows[3]).queryByRole('button')).toBeNull()

    // Closed, only the headline is read; opened, every facet in its own unit.
    expect(within(rows[0]).queryByText('자주 쓰는 끝맺음')).toBeNull()
    await user.click(within(rows[0]).getByRole('button', { name: '문장 끝' }))
    const facets = within(within(rows[0]).getByRole('region', { name: '문장 끝' }))
    expect(facets.getByText("'~다'").nextElementSibling).toHaveTextContent('내 말투 5% · 이 글 95%')
    expect(facets.getByText('자주 쓰는 끝맺음').nextElementSibling).toHaveTextContent(
      '내 말투 ~더라구요 · 이 글 없음',
    )
  })

  // VOICE-62: the summary counts judged items only; each judged item carries its badge and an
  // item with no verdict or an unknown one carries none.
  it('opens with the n-of-m summary and badges each judged item', () => {
    render(<FingerprintComparison items={ITEMS} textLabel="이 글" />)

    expect(screen.getByText('2개 중 1개 비슷해요')).toBeInTheDocument()
    const rows = screen.getAllByRole('listitem')
    expect(within(rows[0]).getByText('다름')).toBeInTheDocument()
    expect(within(rows[1]).getByText('비슷함')).toBeInTheDocument()
    expect(within(rows[2]).queryByText('비슷함')).toBeNull()
    expect(within(rows[2]).queryByText('다름')).toBeNull()
    expect(within(rows[3]).queryByText('비슷함')).toBeNull()
  })

  it('shows no summary when nothing was judged', () => {
    render(<FingerprintComparison items={[ITEMS[2], ITEMS[3]]} textLabel="이 글" />)

    expect(screen.queryByText(/비슷해요/)).toBeNull()
    expect(screen.queryByText('비슷함')).toBeNull()
  })

  it('words the comparison in English', () => {
    initializeI18n('en')
    render(<FingerprintComparison items={ITEMS} textLabel="This post" />)

    const rows = screen.getAllByRole('listitem')
    expect(rows[0]).toHaveTextContent("'~해요' Your voice 92% · This post 0%")
    expect(rows[3]).toHaveTextContent('Unknown')
    expect(screen.getByText('1 of 2 match your voice')).toBeInTheDocument()
    expect(within(rows[0]).getByText('Different')).toBeInTheDocument()
    expect(within(rows[1]).getByText('Similar')).toBeInTheDocument()
  })
})
