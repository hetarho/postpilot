import { useState, type ReactNode } from 'react'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import type { QualityMetricId } from '@/entities/quality'
import { Popover } from '@/shared/ui'
import type { FakeQualityReading } from '@/test/quality'
import { createFakeAuthTransport, createTestQueryClient, withProviders } from '@/test/session'
import { QualityRuleChoices } from './QualityRuleChoices'

afterEach(cleanup)

const RULE_M1 = '제목마다 “성수 카페”를 반복하지 말고, 이번 글의 장소나 메뉴로 제목을 여세요.'
const RULE_M2 = '“오늘도 즐거운 하루”처럼 다른 글에 있던 문장을 그대로 쓰지 마세요.'

// Band edges and counts are fixture data: the rows print what the server sent.
const READINGS: FakeQualityReading[] = [
  {
    metric: 'title_saturation',
    verdict: 'over_band',
    minimum: 10,
    publishedCount: 12,
    ruleText: RULE_M1,
    values: { share: 0.42, shareWarnAbove: 0.3 },
  },
  {
    metric: 'cross_post_phrases',
    verdict: 'over_band',
    minimum: 3,
    publishedCount: 12,
    ruleText: RULE_M2,
    values: { share: 0.15, shareWarnAbove: 0.1 },
  },
  {
    metric: 'in_post_repetition',
    verdict: 'within_band',
    minimum: 1,
    publishedCount: 12,
    values: {
      repetitionShare: 0.05,
      titleRelevance: 0.7,
      repetitionShareWarnAbove: 0.08,
      titleRelevanceWarnBelow: 0.5,
    },
  },
  {
    metric: 'composition',
    verdict: 'within_band',
    minimum: 3,
    publishedCount: 12,
    values: { distinctBlockTypes: 3, distinctBlockTypesWarnAtOrBelow: 2 },
  },
]

/** The form the rows report into: it holds the set and records every report, the way the brief's
 *  run-options form does (POST-89). */
function Harness({
  initial,
  targetLanguage = 'ko',
  disabled,
  reported,
}: {
  initial: QualityMetricId[]
  targetLanguage?: 'ko' | 'en'
  disabled: boolean
  reported: QualityMetricId[][]
}) {
  const [value, setValue] = useState(initial)
  return (
    <QualityRuleChoices
      ownerId="alice"
      slug="post"
      targetLanguage={targetLanguage}
      value={value}
      onChange={(next) => {
        reported.push(next)
        setValue(next)
      }}
      disabled={disabled}
    />
  )
}

function renderChoices(
  props: { ticked?: QualityMetricId[]; disabled?: boolean } = {},
  backend: {
    readings?: FakeQualityReading[]
    accountFails?: boolean
    wrap?: (ui: ReactNode) => ReactNode
  } = {},
) {
  const calls: string[] = []
  const reported: QualityMetricId[][] = []
  const transport = createFakeAuthTransport({
    user: { id: 'alice' },
    calls,
    posts: { posts: [{ slug: 'post', targetLength: 1500 }] },
    quality: {
      accounts: { post: backend.readings ?? READINGS },
      accountFails: backend.accountFails,
    },
  })
  const harness = (targetLanguage: 'ko' | 'en' = 'ko') => (
    <Harness
      initial={props.ticked ?? []}
      targetLanguage={targetLanguage}
      disabled={props.disabled ?? false}
      reported={reported}
    />
  )
  const ui = harness()
  const view = render(backend.wrap ? backend.wrap(ui) : ui, {
    wrapper: withProviders(transport, createTestQueryClient()),
  })
  return { calls, reported, view, harness }
}

const m1Box = () => screen.findByRole('checkbox', { name: /^제목 도배율 42%/ })
const saves = (calls: string[]) => calls.filter((call) => call === 'SavePostGenerationOptions')

describe('the brief quality rows', () => {
  it('reports a tick as the whole next set, in catalogue order, and saves nothing', async () => {
    const user = userEvent.setup()
    const { calls, reported } = renderChoices({ ticked: ['cross_post_phrases'] })

    const box = await m1Box()
    expect(box).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^글 간 고정 문구 15%/ })).toBeChecked()
    await user.click(box)

    expect(box).toBeChecked()
    // The whole set in catalogue order, whatever was pressed first; the brief's 저장 sends it.
    expect(reported).toEqual([['title_saturation', 'cross_post_phrases']])
    expect(saves(calls)).toEqual([])
  })

  it('reports an untick', async () => {
    const user = userEvent.setup()
    const { reported } = renderChoices({ ticked: ['title_saturation'] })

    const box = await m1Box()
    expect(box).toBeChecked()
    await user.click(box)

    expect(box).not.toBeChecked()
    expect(reported).toEqual([[]])
  })

  // R23: a stored tick whose metric is no longer over band has no box, and it stays in the set;
  // the server ignores it until the metric crosses again.
  it('keeps a stored within-band tick in the set it reports', async () => {
    const user = userEvent.setup()
    const { reported } = renderChoices({ ticked: ['in_post_repetition'] })

    expect(await screen.findByText(/반복 5%/)).toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: /글 안 반복/ })).toBeNull()
    await user.click(await m1Box())

    expect(reported).toEqual([['title_saturation', 'in_post_repetition']])
  })

  // QUAL-13, QUAL-14: the tip says what is counted and why the row appeared, then quotes the
  // exact sentence ticking adds.
  it('quotes the rule text verbatim in the tip', async () => {
    const user = userEvent.setup()
    renderChoices()

    const tip = await screen.findByRole('button', { name: '제목 도배율 설명' })
    await user.click(tip)

    const spoken = tip.parentElement?.querySelector('[role="status"]')
    expect(spoken?.textContent).toBe(
      '최근 발행 글의 제목 가운데, 가장 많은 제목에 들어간 명사가 들어 있는 제목의 비율이에요. ' +
        '발행한 글이 12편 있고, 지금 값은 42%예요. 기준은 30% 이하예요. ' +
        `체크하면 다음 생성에 이 규칙이 더해져요: “${RULE_M1}”`,
    )
  })

  // Each over-band metric explains itself with its own values and edges, in the direction that is
  // fine, and the published count as the account's.
  it('explains every over-band metric with its own values and edges', async () => {
    const user = userEvent.setup()
    renderChoices(
      {},
      {
        readings: [
          READINGS[0],
          READINGS[1],
          {
            ...READINGS[2],
            verdict: 'over_band',
            ruleText: '한 명사를 본문 내내 되풀이하지 마세요.',
          },
          { ...READINGS[3], verdict: 'over_band', ruleText: '블록을 세 종류 이상으로 짜세요.' },
        ],
      },
    )

    const spoken = async (name: string) => {
      const tip = await screen.findByRole('button', { name: `${name} 설명` })
      await user.click(tip)
      const text = tip.parentElement?.querySelector('[role="status"]')?.textContent ?? ''
      await user.click(tip)
      return text
    }
    expect(await spoken('글 간 고정 문구')).toContain(
      '발행한 글이 12편 있고, 지금 값은 15%예요. 기준은 10% 이하예요.',
    )
    expect(await spoken('글 안 반복과 제목 관련성')).toContain(
      '발행한 글이 12편 있고, 지금 반복은 5%, 제목 관련성은 70%예요. 기준은 반복 8% 이하, 제목 관련성 50% 이상이에요.',
    )
    expect(await spoken('분량·구성')).toContain(
      '발행한 글이 12편 있고, 지금 블록 종류는 3가지예요. 기준은 2가지 초과예요. 체크하면 다음 생성에 이 규칙이 더해져요: “블록을 세 종류 이상으로 짜세요.”',
    )
  })

  // The rule texts are rendered in the post's target language, so a switch is a new read.
  it('reads the aggregate again when the target language changes', async () => {
    const { calls, view, harness } = renderChoices()
    await m1Box()
    expect(calls.filter((call) => call === 'GetAccountQuality')).toHaveLength(1)

    view.rerender(harness('en'))
    await waitFor(() =>
      expect(calls.filter((call) => call === 'GetAccountQuality')).toHaveLength(2),
    )
  })

  it('opens the tip without toggling the box', async () => {
    const user = userEvent.setup()
    const { calls } = renderChoices()

    const box = await m1Box()
    const tip = screen.getByRole('button', { name: '제목 도배율 설명' })
    await user.click(tip)

    expect(tip).toHaveAttribute('aria-expanded', 'true')
    expect(box).not.toBeChecked()
    expect(saves(calls)).toEqual([])
  })

  // THEME-32: one Escape dismisses one thing.
  it('closes only the tip on Escape inside the brief', async () => {
    const user = userEvent.setup()
    renderChoices({}, { wrap: (ui) => <Popover label="글쓰기 옵션">{() => ui}</Popover> })

    await user.click(screen.getByRole('button', { name: '글쓰기 옵션' }))
    const tip = await screen.findByRole('button', { name: '제목 도배율 설명' })
    await user.click(tip)
    expect(tip).toHaveAttribute('aria-expanded', 'true')

    await user.keyboard('{Escape}')
    expect(tip).toHaveAttribute('aria-expanded', 'false')
    expect(screen.getByRole('dialog', { name: '글쓰기 옵션' })).toBeInTheDocument()

    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog', { name: '글쓰기 옵션' })).toBeNull())
  })

  it('keeps the tip operable while the box is disabled', async () => {
    const user = userEvent.setup()
    renderChoices({ disabled: true })

    expect(await m1Box()).toBeDisabled()
    const tip = screen.getByRole('button', { name: '제목 도배율 설명' })
    expect(tip).toBeEnabled()
    await user.click(tip)
    expect(tip).toHaveAttribute('aria-expanded', 'true')
    expect(tip.parentElement?.querySelector('[role="status"]')?.textContent).toContain(RULE_M1)
  })

  // POST-81: a metric the answer left out is absent, never a missing row.
  it('renders every metric, a missing one as absent', async () => {
    renderChoices({}, { readings: [READINGS[0]] })

    await m1Box()
    for (const name of ['글 간 고정 문구', '글 안 반복과 제목 관련성', '분량·구성']) {
      expect(screen.getByText(name).parentElement).toHaveTextContent(`${name}측정할 수 없어요`)
    }
    expect(screen.getAllByRole('checkbox')).toHaveLength(1)
  })

  it('names the minimum while the account is under it, and 양호 within band', async () => {
    renderChoices(
      {},
      {
        readings: [
          {
            metric: 'title_saturation',
            verdict: 'below_minimum',
            minimum: 10,
            publishedCount: 4,
            values: { shareWarnAbove: 0.3 },
          },
          READINGS[2],
        ],
      },
    )

    expect(
      await screen.findByText('발행한 글이 10편 이상이면 비교해요. 지금은 4편이에요.'),
    ).toBeInTheDocument()
    const within = screen.getByText('글 안 반복과 제목 관련성').parentElement
    expect(within).toHaveTextContent('반복 5% · 제목 관련성 70%')
    expect(screen.getByText('양호')).toHaveClass('bg-notice-success-bg')
    expect(screen.queryAllByRole('checkbox')).toHaveLength(0)
    // QUAL-6: whose thresholds, and counted from what.
    expect(
      screen.getByText(
        '배지 기준은 PostPilot이 정한 값이에요. PostPilot에 저장된 발행 글에서 센 값이에요.',
      ),
    ).toBeInTheDocument()
  })

  it('says it is counting until the aggregate lands, and retries a failed read', async () => {
    const user = userEvent.setup()
    const { calls } = renderChoices({}, { accountFails: true })

    expect(screen.getByText('발행 글을 세는 중이에요.')).toBeInTheDocument()
    expect(await screen.findByText(/발행 글 점검을 불러오지 못했어요/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '다시 시도' }))
    await waitFor(() =>
      expect(calls.filter((call) => call === 'GetAccountQuality')).toHaveLength(2),
    )
  })
})
