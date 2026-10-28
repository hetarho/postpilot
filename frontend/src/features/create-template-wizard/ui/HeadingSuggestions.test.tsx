import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import type { FakeTemplatesOptions } from '@/test/templates'

const providers = {
  models: [{ providerId: 'p', modelId: 'w', label: '글 모델', stages: [Stage.WRITE] }],
  selections: [{ stage: Stage.WRITE, providerId: 'p', modelId: 'w' }],
}
async function headings(options: FakeTemplatesOptions = {}, withModel = true) {
  const user = userEvent.setup()
  const calls: string[] = []
  const starts: NonNullable<FakeTemplatesOptions['headingStarts']> = []
  const view = renderAppAt('/templates/new', {
    user: { id: 'alice' },
    calls,
    providers: withModel ? providers : {},
    templates: { headingStarts: starts, headingEstimate: { credits: 2 }, ...options },
    jobs: {
      jobs: [
        { id: 'heading-job', kind: 'template_heading_suggestion', status: 'done', stage: 'write' },
      ],
    },
  })
  await user.click(await screen.findByRole('button', { name: /^맛집/ }))
  await user.click(await screen.findByRole('button', { name: /^식사/ }))
  await user.click(screen.getByRole('button', { name: '다음' }))
  await user.click(screen.getByRole('button', { name: '다음' }))
  await screen.findByRole('heading', { name: '부분마다 소제목을 정해요' })
  return { ...view, user, calls, starts }
}

/** Opens the style sheet and sends the given request (TMPL-73). */
async function suggest(user: ReturnType<typeof userEvent.setup>, request = '') {
  await user.click(screen.getByRole('button', { name: '제목 제안 받기' }))
  const sheet = await screen.findByRole('dialog', { name: '제목 제안 받기' })
  if (request)
    await user.type(within(sheet).getByLabelText('원하는 스타일을 말씀해주세요'), request)
  await user.click(within(sheet).getByRole('button', { name: '제안 받기' }))
  return sheet
}

describe('explicit heading suggestions', () => {
  it('shows the estimate in the sheet but opening, closing and 다음 issue no paid call', async () => {
    const { user, calls, starts } = await headings()
    expect(screen.queryByRole('button', { name: '내 말로 쓰기' })).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: '제목 제안 받기' }))
    const sheet = await screen.findByRole('dialog', { name: '제목 제안 받기' })
    expect(await within(sheet).findByText('약 2 크레딧')).toBeVisible()
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: '다음' }))
    expect(await screen.findByLabelText('소제목')).toHaveValue('방문 계기')
    expect(starts).toEqual([])
    expect(calls.filter((name) => name.startsWith('Start'))).toEqual([])
  })

  it('keeps navigation and manual input while one call runs, then offers editable choices per section', async () => {
    let finish!: () => void
    const headingStartGate = new Promise<void>((resolve) => {
      finish = resolve
    })
    const { user, starts } = await headings({ headingStartGate })
    await user.click(screen.getByRole('button', { name: '제목 제안 받기' }))
    const sheet = await screen.findByRole('dialog', { name: '제목 제안 받기' })
    await user.click(
      within(sheet).getByRole('button', {
        name: '이모지를 포함해서 20대 여성을 타겟팅할 수 있는 소제목으로 만들어줘',
      }),
    )
    expect(within(sheet).getByLabelText('원하는 스타일을 말씀해주세요')).toHaveValue(
      '이모지를 포함해서 20대 여성을 타겟팅할 수 있는 소제목으로 만들어줘',
    )
    await user.dblClick(within(sheet).getByRole('button', { name: '제안 받기' }))
    await waitFor(() => expect(starts).toHaveLength(1))
    expect(starts[0]).toMatchObject({
      writeModel: { providerId: 'p', modelId: 'w' },
      targetKind: 'post',
      targetPath: ['맛집', '식사'],
      styleRequest: '이모지를 포함해서 20대 여성을 타겟팅할 수 있는 소제목으로 만들어줘',
    })
    // A slow admission does not hold the owner: the sheet closes and the request goes on.
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    await user.click(screen.getByRole('button', { name: '다음' }))
    const field = await screen.findByLabelText('소제목')
    await user.clear(field)
    await user.type(field, '내가 쓴 제목')
    expect(screen.getByRole('status')).toHaveTextContent('소제목을 제안하는 중')
    await act(async () => finish())
    await screen.findByRole('button', { name: '방문 계기 후보 하나' })
    expect(field).toHaveValue('내가 쓴 제목')
    await user.click(screen.getByRole('button', { name: '방문 계기 후보 둘' }))
    expect(field).toHaveValue('방문 계기 후보 둘')
    await user.type(field, ' 내 표현')
    expect(field).toHaveValue('방문 계기 후보 둘 내 표현')
    await user.click(screen.getByRole('button', { name: '이전' }))
    expect(screen.getByLabelText('방문 계기 소제목')).toHaveValue('방문 계기 후보 둘 내 표현')
    expect(starts).toHaveLength(1)
  })

  it('keeps own words after admission failure', async () => {
    const { user, starts } = await headings({ headingRefusal: 'TEMPLATE_REQUEST_RUNNING' })
    await user.type(screen.getByLabelText('방문 계기 소제목'), ' 내 문장')
    const sheet = await suggest(user, '짧게')
    // The refusal is said in the sheet, beside the request it refused, and the sheet stays up.
    await within(sheet).findByText(/다른 템플릿 요청이 아직 진행 중/)
    expect(within(sheet).getByLabelText('원하는 스타일을 말씀해주세요')).toHaveValue('짧게')
    await user.keyboard('{Escape}')
    await screen.findByText(/다른 템플릿 요청이 아직 진행 중/)
    expect(screen.getByLabelText('방문 계기 소제목')).toHaveValue('방문 계기 내 문장')
    await user.click(screen.getByRole('button', { name: '다음' }))
    expect(await screen.findByLabelText('소제목')).toHaveValue('방문 계기 내 문장')
    expect(starts).toHaveLength(1)
  })

  it('does not offer a late result for a different target or its sections', async () => {
    let finish!: () => void
    const headingStartGate = new Promise<void>((resolve) => {
      finish = resolve
    })
    const { user, starts } = await headings({ headingStartGate })
    await suggest(user)
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    for (let i = 0; i < 4; i++) await user.click(screen.getByRole('button', { name: '이전' }))
    await user.click(screen.getByRole('button', { name: /^카페/ }))
    await user.click(screen.getByRole('button', { name: /^브런치/ }))
    await user.click(screen.getByRole('button', { name: '다음' }))
    await user.click(screen.getByRole('button', { name: '다음' }))
    await act(async () => finish())
    await waitFor(() =>
      expect(screen.getByRole('button', { name: '제목 제안 받기' })).toBeEnabled(),
    )
    expect(screen.queryByRole('button', { name: '방문 계기 후보 하나' })).not.toBeInTheDocument()
    expect(screen.getByLabelText('방문 계기 소제목')).toHaveValue('방문 계기')
    expect(starts).toHaveLength(1)
  })

  it('retries a failed result read without another provider admission', async () => {
    const { user, starts } = await headings({ headingReadFailures: 1 })
    await suggest(user)
    await user.click(await screen.findByRole('button', { name: '결과 다시 불러오기' }))
    await screen.findByRole('button', { name: '방문 계기 후보 하나' })
    expect(starts).toHaveLength(1)
  })

  it('sends no request when the sheet is sent empty', async () => {
    const { user, starts } = await headings()
    await suggest(user)
    await waitFor(() => expect(starts).toHaveLength(1))
    expect(starts[0]!.styleRequest ?? '').toBe('')
  })

  it('requires an explicit eligible writer while own words remain available', async () => {
    const { user, starts } = await headings({}, false)
    expect(screen.getByRole('button', { name: '제목 제안 받기' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: '다음' }))
    expect(await screen.findByLabelText('소제목')).toHaveValue('방문 계기')
    expect(starts).toEqual([])
  })
})
