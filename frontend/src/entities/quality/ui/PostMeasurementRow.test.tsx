import { create } from '@bufbuild/protobuf'
import { createRouterTransport } from '@connectrpc/connect'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import {
  GetPostMeasurementResponseSchema,
  QualityService,
  type ContentLanguage,
} from '@/shared/api'
import {
  createFakeQualityTransport,
  toFakeProtoReading,
  type FakeQualityOptions,
  type FakeQualityReading,
} from '@/test/quality'
import { createTestQueryClient, withProviders } from '@/test/session'
import { invalidateQuality } from '../api/quality-cache'
import { PostMeasurementRow } from './PostMeasurementRow'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

const HEADING = '이 글의 측정값'
const M2 = '글 간 고정 문구'
const M3 = '글 안 반복과 제목 관련성'
const M4 = '분량·구성'

// Band edges are fixture data: the row prints what the server sent and mirrors none of them.
const M2_VALUES = { share: 0.123, shareWarnAbove: 0.1 }
const M3_VALUES = {
  repetitionShare: 0.09,
  titleRelevance: 0.4,
  repetitionShareWarnAbove: 0.08,
  titleRelevanceWarnBelow: 0.5,
}
const M4_VALUES = {
  charCount: 1234,
  photoCount: 3,
  distinctBlockTypes: 2,
  averageSentenceLength: 41.2,
  distinctBlockTypesWarnAtOrBelow: 2,
}

function readings(verdict: FakeQualityReading['verdict']): FakeQualityReading[] {
  return [
    { metric: 'cross_post_phrases', verdict, minimum: 3, publishedCount: 5, values: M2_VALUES },
    { metric: 'in_post_repetition', verdict, values: M3_VALUES },
    { metric: 'composition', verdict, values: M4_VALUES },
  ]
}

function renderRow(
  measured: FakeQualityReading[] | undefined,
  options: Omit<FakeQualityOptions, 'measurements'> & {
    contentLanguage?: ContentLanguage
    ownerId?: string
  } = {},
) {
  const calls: string[] = []
  const { contentLanguage, ownerId = 'alice', ...quality } = options
  const transport = createFakeQualityTransport({
    calls,
    measurements: measured ? { post: measured } : {},
    ...quality,
  })
  const queryClient = createTestQueryClient()
  render(
    <PostMeasurementRow
      ownerId={ownerId}
      slug="post"
      revision={1n}
      contentLanguage={contentLanguage}
    />,
    { wrapper: withProviders(transport, queryClient) },
  )
  return { calls, transport, queryClient }
}

/** One metric's group: its name, its badge and its value lines. */
async function group(name: string) {
  const term = await screen.findByText(name)
  return term.closest('div') as HTMLElement
}

const lines = (element: HTMLElement) =>
  within(element)
    .getAllByRole('definition')
    .map((line) => line.textContent)

describe('the post measurement row', () => {
  it('shows every number over its band with the edge beside it and 주의', async () => {
    renderRow(readings('over_band'))

    const m2 = await group(M2)
    expect(lines(m2)).toEqual(['저장된 발행 글과 겹치는 부분 12.3% · 기준 10% 이하'])
    const m3 = await group(M3)
    expect(lines(m3)).toEqual([
      '본문 명사 중 가장 잦은 명사 9% · 기준 8% 이하',
      '본문에도 나오는 제목 명사 40% · 기준 50% 이상',
    ])
    const m4 = await group(M4)
    // Only the distinct block types carry a band; the other three are context (QUAL-10).
    expect(lines(m4)).toEqual([
      '글자 수 1,234자',
      '사진 3장',
      '블록 종류 2가지 · 기준 2가지 초과',
      '평균 문장 길이 41.2자',
    ])
    for (const metric of [m2, m3, m4]) {
      const badge = within(metric).getByText('주의')
      expect(badge).toHaveClass('bg-notice-warning-bg')
      expect(within(metric).queryByText('양호')).toBeNull()
    }
  })

  // QUAL-37: a post's own numbers are shown whether or not a band is crossed.
  it('shows the same numbers within band, under 양호', async () => {
    renderRow(readings('within_band'))

    const m2 = await group(M2)
    expect(lines(m2)).toEqual(['저장된 발행 글과 겹치는 부분 12.3% · 기준 10% 이하'])
    for (const metric of [m2, await group(M3), await group(M4)]) {
      expect(within(metric).getByText('양호')).toHaveClass('bg-notice-success-bg')
      expect(within(metric).queryByText('주의')).toBeNull()
    }
    expect(lines(await group(M4))).toHaveLength(4)
  })

  // QUAL-12, QUAL-36: an account under M2's minimum reads the minimum, the brief's own line.
  it('names M2’s minimum in place of its value while the account is under it', async () => {
    renderRow([
      {
        metric: 'cross_post_phrases',
        verdict: 'below_minimum',
        minimum: 3,
        publishedCount: 1,
        values: { shareWarnAbove: 0.1 },
      },
      { metric: 'in_post_repetition', verdict: 'within_band', values: M3_VALUES },
      { metric: 'composition', verdict: 'within_band', values: M4_VALUES },
    ])

    const m2 = await group(M2)
    expect(lines(m2)).toEqual(['발행한 글이 3편 이상이면 비교해요. 지금은 1편이에요.'])
    expect(within(m2).queryByText('주의')).toBeNull()
    expect(within(m2).queryByText('양호')).toBeNull()
  })

  // QUAL-40: a value that cannot be computed is neither zero nor a verdict.
  it('reads an absent metric as unmeasurable, with no badge', async () => {
    renderRow([
      { metric: 'cross_post_phrases', verdict: 'absent', values: { shareWarnAbove: 0.1 } },
      {
        metric: 'in_post_repetition',
        verdict: 'absent',
        values: { repetitionShareWarnAbove: 0.08, titleRelevanceWarnBelow: 0.5 },
      },
      { metric: 'composition', verdict: 'absent' },
    ])

    expect(lines(await group(M2))).toEqual([
      '저장된 발행 글과 겹치는 부분 측정할 수 없어요 · 기준 10% 이하',
    ])
    expect(lines(await group(M3))).toEqual([
      '본문 명사 중 가장 잦은 명사 측정할 수 없어요 · 기준 8% 이하',
      '본문에도 나오는 제목 명사 측정할 수 없어요 · 기준 50% 이상',
    ])
    // With no values at all there is no edge to print either.
    expect(lines(await group(M4))).toEqual([
      '글자 수 측정할 수 없어요',
      '사진 측정할 수 없어요',
      '블록 종류 측정할 수 없어요',
      '평균 문장 길이 측정할 수 없어요',
    ])
    for (const name of [M2, M3, M4]) {
      const metric = await group(name)
      expect(within(metric).queryByText('주의')).toBeNull()
      expect(within(metric).queryByText('양호')).toBeNull()
    }
  })

  it('renders a stored 0 as 0, never as absent', async () => {
    renderRow([
      {
        metric: 'cross_post_phrases',
        verdict: 'within_band',
        values: { share: 0, shareWarnAbove: 0.1 },
      },
      {
        metric: 'in_post_repetition',
        verdict: 'over_band',
        values: { ...M3_VALUES, titleRelevance: 0 },
      },
      {
        metric: 'composition',
        verdict: 'within_band',
        values: { ...M4_VALUES, photoCount: 0, averageSentenceLength: undefined },
      },
    ])

    expect(lines(await group(M2))).toEqual(['저장된 발행 글과 겹치는 부분 0% · 기준 10% 이하'])
    expect(lines(await group(M3))[1]).toBe('본문에도 나오는 제목 명사 0% · 기준 50% 이상')
    expect(lines(await group(M4))).toEqual([
      '글자 수 1,234자',
      '사진 0장',
      '블록 종류 2가지 · 기준 2가지 초과',
      '평균 문장 길이 측정할 수 없어요',
    ])
  })

  // QUAL-26: characters for Korean, words for English, and a post with none reads as Korean.
  it.each([
    ['ko', '평균 문장 길이 41.2자'],
    ['en', '평균 문장 길이 41.2단어'],
    [undefined, '평균 문장 길이 41.2자'],
  ] as const)('measures a %s post’s sentences in its own unit', async (language, line) => {
    renderRow(readings('within_band'), { contentLanguage: language })

    expect(lines(await group(M4))[3]).toBe(line)
  })

  // QUAL-6, QUAL-36: three groups, never M1 and never a composite score, and one line saying whose
  // thresholds these are.
  it('holds exactly M2, M3 and M4 under one heading, and says whose bands they are', async () => {
    renderRow([
      {
        metric: 'title_saturation',
        verdict: 'over_band',
        values: { share: 0.5, shareWarnAbove: 0.3 },
      },
      ...readings('within_band'),
    ])

    const region = await screen.findByRole('region', { name: HEADING })
    await group(M2)
    expect(
      within(region)
        .getAllByRole('term')
        .map((term) => term.firstChild?.textContent),
    ).toEqual([M2, M3, M4])
    expect(within(region).queryByText('제목 도배율')).toBeNull()
    expect(region.querySelector('dl')).toHaveClass('grid-cols-1', 'sm:grid-cols-3')
    expect(within(region).getByText('배지 기준은 PostPilot이 정한 값이에요.')).toBeInTheDocument()
  })

  // QUAL-3: a new revision is a new measurement — a generation moves it without any save hook — and
  // the last reading stays on screen while the next one loads.
  it('reads again when the revision moves, keeping the last reading meanwhile', async () => {
    const answers: Array<() => void> = []
    const measured: Record<string, FakeQualityReading[]> = {
      '1': readings('over_band'),
      '2': readings('within_band'),
    }
    let reads = 0
    const transport = createRouterTransport(({ rpc }) => {
      rpc(QualityService.method.getPostMeasurement, async () => {
        reads += 1
        const revision = String(reads)
        if (reads > 1) await new Promise<void>((resolve) => answers.push(resolve))
        return create(GetPostMeasurementResponseSchema, {
          contentRevision: BigInt(revision),
          readings: measured[revision].map(toFakeProtoReading),
        })
      })
    })
    const view = render(<PostMeasurementRow ownerId="alice" slug="post" revision={1n} />, {
      wrapper: withProviders(transport, createTestQueryClient()),
    })
    expect(within(await group(M2)).getByText('주의')).toBeInTheDocument()

    view.rerender(<PostMeasurementRow ownerId="alice" slug="post" revision={2n} />)
    await waitFor(() => expect(reads).toBe(2))
    expect(screen.queryByText('측정하는 중이에요.')).toBeNull()
    expect(within(await group(M2)).getByText('주의')).toBeInTheDocument()

    answers.shift()?.()
    await waitFor(() =>
      expect(within(screen.getByText(M2).closest('div')!).getByText('양호')).toBeInTheDocument(),
    )
  })

  // The post hooks mark every quality entry stale by prefix; this reading must sit under it.
  it('reads again when the quality readings are marked stale', async () => {
    const { calls, transport, queryClient } = renderRow(readings('within_band'))
    await group(M2)
    expect(calls).toEqual(['GetPostMeasurement'])

    await invalidateQuality(queryClient, transport)
    expect(calls).toEqual(['GetPostMeasurement', 'GetPostMeasurement'])
  })

  it('reads nothing without an account', async () => {
    const { calls } = renderRow(readings('within_band'), { ownerId: '' })

    expect(screen.getByText('측정하는 중이에요.')).toBeInTheDocument()
    await new Promise((resolve) => setTimeout(resolve, 50))
    expect(calls).toEqual([])
  })

  it('says it is measuring until the reading lands', async () => {
    renderRow(readings('within_band'))

    expect(screen.getByText('측정하는 중이에요.')).toBeInTheDocument()
    await group(M2)
    expect(screen.queryByText('측정하는 중이에요.')).toBeNull()
  })

  it('says a failed read failed and retries it on request', async () => {
    const user = userEvent.setup()
    const { calls } = renderRow(undefined, { measurementFails: true })

    const region = await screen.findByRole('region', { name: HEADING })
    expect(await within(region).findByText(/측정값을 불러오지 못했어요\./)).toBeInTheDocument()
    expect(calls).toEqual(['GetPostMeasurement'])
    await user.click(within(region).getByRole('button', { name: '다시 시도' }))
    expect(calls).toEqual(['GetPostMeasurement', 'GetPostMeasurement'])
  })

  it('renders in English with the same shape', async () => {
    initializeI18n('en')
    renderRow(readings('over_band'), { contentLanguage: 'en' })

    await screen.findByRole('region', { name: 'This post’s measurements' })
    const m4 = await group('Length and structure')
    expect(lines(m4)).toEqual([
      'Characters 1,234',
      'Photos 3',
      'Block types 2 · target above 2',
      'Average sentence length 41.2 words',
    ])
    expect(within(m4).getByText('Caution')).toBeInTheDocument()
    expect(screen.getByText('The badge thresholds are PostPilot’s own.')).toBeInTheDocument()
  })
})
