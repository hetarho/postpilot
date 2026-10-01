import { describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Stage } from '@/shared/api'
import { TEMPLATE_MAX_PER_ACCOUNT } from '@/entities/template'
import { renderAppAt, type RenderAppOptions } from '@/test/app'
import { finalizedPostRow } from '@/test/posts'
import type {
  FakeTemplateRequestStart,
  FakeTemplateRow,
  FakeTemplatesOptions,
} from '@/test/templates'
import type { FakeGenerationJobRow } from '@/test/jobs'

const USER = { id: 'alice' }
const JOB = 'template-request-1'
const WRITER = {
  models: [{ providerId: 'openrouter', modelId: 'writer', label: '라이터' }],
  selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer' }],
}
const STORED: FakeTemplateRow = {
  id: 'template-review',
  name: '정보성 식당 리뷰',
  description: '협찬 방문 리뷰',
  body: '<write>인트로</write>',
}
const ANSWER = {
  draft: {
    name: '맛집 리뷰',
    description: '맛집 방문기',
    titleArea: '',
    body: '<write>방문 이유</write>\n<slot kind="photo" count="2"/>',
  },
  wishes: ['친근하게', '가격은 빼고'],
}

function job(status: string, extra: Partial<FakeGenerationJobRow> = {}): FakeGenerationJobRow {
  return { id: JOB, kind: 'template_request', status, stage: 'write', ...extra }
}

function renderEditor(
  at: string,
  {
    templates = {},
    jobs = [job('done')],
    providers = WRITER,
    extra = {},
  }: {
    templates?: FakeTemplatesOptions
    jobs?: FakeGenerationJobRow[]
    providers?: RenderAppOptions['providers']
    extra?: RenderAppOptions
  } = {},
) {
  return renderAppAt(at, {
    user: USER,
    providers,
    jobs: { jobs },
    templates: { templates: [STORED], ...templates },
    ...extra,
  })
}

const box = () => screen.getByRole('region', { name: 'AI에게 템플릿 요청' })
const field = () => within(box()).getByRole('textbox', { name: 'AI에게 템플릿 요청' })
const send = () => within(box()).getByRole('button', { name: '요청 보내기' })

// TMPL-62: the box is the first action on an empty new template and a collapsed button once a
// draft holds anything.
describe('where the request box stands', () => {
  it('is open on an empty new template', async () => {
    renderEditor('/templates/new')
    expect(await screen.findByRole('region', { name: 'AI에게 템플릿 요청' })).toBeInTheDocument()
  })

  it('is a collapsed button on a stored template, and collapsing keeps the text', async () => {
    const user = userEvent.setup()
    renderEditor('/templates/template-review')
    await user.click(await screen.findByRole('button', { name: 'AI에게 요청' }))
    await user.type(field(), '사진 줄을 2장으로')
    await user.click(within(box()).getByRole('button', { name: '접기' }))
    await user.click(screen.getByRole('button', { name: 'AI에게 요청' }))
    expect(field()).toHaveValue('사진 줄을 2장으로')
  })
})

describe('what the box says before it is pressed', () => {
  it('names the 글 작성 모델 and what one request costs', async () => {
    renderEditor('/templates/new', { templates: { requestEstimate: { credits: 3 } } })
    expect(
      await within(await screen.findByRole('region', { name: 'AI에게 템플릿 요청' })).findByText(
        '라이터로 만들어요 · 약 3 크레딧',
      ),
    ).toBeInTheDocument()
  })

  it('says 무료 for a free model', async () => {
    renderEditor('/templates/new', { templates: { requestEstimate: { free: true } } })
    expect(await screen.findByText('라이터로 만들어요 · 무료')).toBeInTheDocument()
  })

  it('names no figure when none can be stated', async () => {
    renderEditor('/templates/new')
    expect(await screen.findByText('라이터로 만들어요')).toBeInTheDocument()
  })

  it('is refused with a route to model selection when no writer is chosen', async () => {
    renderEditor('/templates/new', { providers: { models: WRITER.models } })
    const reason = await screen.findByText(/글 작성 모델을 먼저 선택하세요/)
    expect(within(reason).getByRole('link', { name: '모델 선택하기' })).toHaveAttribute(
      'href',
      '/ai-models',
    )
    expect(send()).toBeDisabled()
  })

  it('is refused on a new template at the account cap, but not on a stored one', async () => {
    const many: FakeTemplateRow[] = Array.from({ length: TEMPLATE_MAX_PER_ACCOUNT }, (_, i) => ({
      id: `template-${i}`,
      name: `템플릿 ${i}`,
    }))
    const user = userEvent.setup()
    const first = renderEditor('/templates/new', { templates: { templates: many } })
    expect(await screen.findByText(/템플릿을 더 만들 수 없어요/)).toBeInTheDocument()
    expect(send()).toBeDisabled()
    first.unmount()

    renderEditor('/templates/template-0', { templates: { templates: many } })
    await user.click(await screen.findByRole('button', { name: 'AI에게 요청' }))
    await user.type(field(), '고쳐 줘')
    expect(send()).toBeEnabled()
  })
})

describe('a request', () => {
  it('sends the explicit writer, the UI language, the text and the whole draft', async () => {
    const user = userEvent.setup()
    const requestStarts: FakeTemplateRequestStart[] = []
    renderEditor('/templates/template-review', { templates: { requestStarts } })
    await user.click(await screen.findByRole('button', { name: 'AI에게 요청' }))
    await user.type(field(), '  사진 줄을 2장으로  ')
    await user.click(send())
    await waitFor(() => expect(requestStarts).toHaveLength(1))
    expect(requestStarts[0]).toEqual({
      writeModel: { providerId: 'openrouter', modelId: 'writer' },
      language: 'ko',
      text: '사진 줄을 2장으로',
      draft: {
        name: STORED.name,
        description: STORED.description,
        titleArea: '',
        body: STORED.body,
      },
      templateId: STORED.id,
      samplePostSlug: undefined,
    })
  })

  it('puts the answer in the draft, lists the wishes, and undoes back to the draft before it', async () => {
    const user = userEvent.setup()
    renderEditor('/templates/template-review', { templates: { requestResult: ANSWER } })
    await user.click(await screen.findByRole('button', { name: 'AI에게 요청' }))
    await user.type(field(), '맛집 리뷰로')
    await user.click(send())

    await waitFor(() => expect(screen.getByLabelText('이름')).toHaveValue('맛집 리뷰'))
    // The draft is dirty: the answer is applied, not saved (TMPL-63).
    expect(screen.getByRole('button', { name: '저장' })).toBeEnabled()
    // The box empties after a finished request.
    expect(field()).toHaveValue('')
    const wishes = within(box()).getByRole('region', { name: '지침에 넣을 내용' })
    expect(within(wishes).getByText('친근하게')).toBeInTheDocument()
    expect(within(wishes).getByRole('link', { name: '지침 만들기' })).toHaveAttribute(
      'href',
      '/guidelines?new=1',
    )

    await user.click(within(box()).getByRole('button', { name: '요청 전으로 되돌리기' }))
    expect(screen.getByLabelText('이름')).toHaveValue(STORED.name)
    expect(
      within(box()).queryByRole('region', { name: '지침에 넣을 내용' }),
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
  })

  it('locks the draft while it runs, and 취소 stops it', async () => {
    const user = userEvent.setup()
    const requestCancels: string[] = []
    renderEditor('/templates/new', { jobs: [job('running')], templates: { requestCancels } })
    await user.type(await screen.findByRole('textbox', { name: 'AI에게 템플릿 요청' }), '맛집 리뷰')
    await user.click(send())

    expect(await within(box()).findByText(/템플릿을 만드는 중이에요/)).toBeInTheDocument()
    expect(screen.getByLabelText('이름')).toBeDisabled()
    expect(screen.getByRole('button', { name: '저장' })).toBeDisabled()
    await user.click(within(box()).getByRole('button', { name: '취소' }))
    await waitFor(() => expect(requestCancels).toEqual([JOB]))
  })

  it('cancels the request when the owner leaves while it runs', async () => {
    const user = userEvent.setup()
    const requestCancels: string[] = []
    renderEditor('/templates/new', { jobs: [job('running')], templates: { requestCancels } })
    await user.type(await screen.findByRole('textbox', { name: 'AI에게 템플릿 요청' }), '맛집 리뷰')
    await user.click(send())
    await within(box()).findByText(/템플릿을 만드는 중이에요/)

    await user.click(screen.getByRole('link', { name: '← 템플릿 목록' }))
    const dialog = await screen.findByRole('dialog', { name: '요청이 취소돼요' })
    await user.click(within(dialog).getByRole('button', { name: '취소하고 나가기' }))
    await waitFor(() => expect(requestCancels).toEqual([JOB]))
  })

  it('keeps the text and the draft when the request fails', async () => {
    const user = userEvent.setup()
    renderEditor('/templates/template-review', {
      jobs: [job('failed', { failureReason: 'TEMPLATE_REQUEST_ANSWER_INVALID' })],
    })
    await user.click(await screen.findByRole('button', { name: 'AI에게 요청' }))
    await user.type(field(), '맛집 리뷰로')
    await user.click(send())

    expect(
      await within(box()).findByText(/템플릿 형식에 맞는 답을 만들지 못했어요/),
    ).toBeInTheDocument()
    expect(field()).toHaveValue('맛집 리뷰로')
    expect(screen.getByLabelText('이름')).toHaveValue(STORED.name)
  })

  it('renders a refused start in place and keeps the text', async () => {
    const user = userEvent.setup()
    renderEditor('/templates/new', {
      templates: { requestRefusal: { reason: 'TEMPLATE_REQUEST_RUNNING' } },
    })
    await user.type(await screen.findByRole('textbox', { name: 'AI에게 템플릿 요청' }), '맛집 리뷰')
    await user.click(send())
    expect(
      await within(box()).findByText(/다른 템플릿 요청이 아직 진행 중이에요/),
    ).toBeInTheDocument()
    expect(field()).toHaveValue('맛집 리뷰')
  })
})

// TMPL-64: a post's ③ opens a new template with that post attached as the sample.
describe('a template started from a post', () => {
  it('attaches the post as a chip, sends it as the sample, and lets the text stay blank', async () => {
    const user = userEvent.setup()
    const requestStarts: FakeTemplateRequestStart[] = []
    renderEditor('/templates/new?from=20260820-final', {
      templates: { requestStarts },
      extra: {
        posts: { posts: [finalizedPostRow({ slug: '20260820-final', title: '성수 카페' })] },
      },
    })
    expect(await screen.findByText('참고 글: 비 온 뒤의 제주')).toBeInTheDocument()
    await user.click(send())
    await waitFor(() => expect(requestStarts).toHaveLength(1))
    expect(requestStarts[0]).toMatchObject({ text: '', samplePostSlug: '20260820-final' })
  })

  it('makes an ordinary request once the chip is removed', async () => {
    const user = userEvent.setup()
    renderEditor('/templates/new?from=20260820-final', {
      extra: {
        posts: { posts: [finalizedPostRow({ slug: '20260820-final', title: '성수 카페' })] },
      },
    })
    await screen.findByText('참고 글: 비 온 뒤의 제주')
    await user.click(screen.getByRole('button', { name: '참고 글 빼기' }))
    expect(screen.queryByText('참고 글: 비 온 뒤의 제주')).not.toBeInTheDocument()
    expect(send()).toBeDisabled()
  })
})
