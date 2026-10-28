// TMPL-71: 새 템플릿 is a guided wizard that saves an ordinary template through the standard create.
import { describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TEMPLATE_PARSE_OPTIONS, parseTemplate } from '@/entities/template'
import { renderAppAt } from '@/test/app'
import type { FakeTemplatesOptions } from '@/test/templates'

const next = () => screen.getByRole('button', { name: '다음' })
const title = (name: string | RegExp) => screen.findByRole('heading', { level: 2, name })

describe('the template wizard', () => {
  it('creates a 맛집 template end to end', async () => {
    const user = userEvent.setup()
    const calls: string[] = []
    const creates: NonNullable<FakeTemplatesOptions['creates']> = []
    const { router } = renderAppAt('/templates/new', {
      user: { id: 'alice' },
      calls,
      templates: { templates: [], creates, calls },
    })

    // The target, one question per screen; Back shows the earlier question with its answer kept.
    await title('어떤 체험을 글로 남기나요?')
    await user.click(screen.getByRole('button', { name: /^맛집/ }))
    await title('어떤 식당인가요?')
    await user.click(screen.getByRole('button', { name: /^식사/ }))
    await title('이런 글은 보통 이렇게 구성돼요')
    const service = screen.getByRole('checkbox', { name: /^서비스·응대/ })
    expect(service).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^아쉬운 점/ })).toBeChecked()
    await user.click(service)
    await user.click(screen.getByRole('button', { name: '이전' }))
    await title('어떤 식당인가요?')
    await user.click(screen.getByRole('button', { name: /^식사/ }))
    expect(await screen.findByRole('checkbox', { name: /^서비스·응대/ })).not.toBeChecked()
    await user.click(next())

    // Order, then headings in the owner's own words.
    await title('어떤 순서로 쓸까요?')
    await user.click(next())
    await title('부분마다 소제목을 정해요')
    const heading = screen.getByLabelText('방문 계기 소제목')
    await user.clear(heading)
    await user.type(heading, '찾아간 이유')
    // The headings step continues with the ordinary 다음; own words are the fields (TMPL-71).
    expect(screen.queryByRole('button', { name: '내 말로 쓰기' })).not.toBeInTheDocument()
    await user.click(next())

    // One screen per section, each already answerable; 매장 정보 asks the author and must be answered.
    await title('“찾아간 이유” 부분')
    for (;;) {
      const current = await title(/부분$|이름을 붙이고 저장해요/)
      if (current.textContent === '이름을 붙이고 저장해요') break
      if (current.textContent === '“매장 정보” 부분')
        await user.click(screen.getByRole('checkbox', { name: '꼭 답하게 하기' }))
      await user.click(next())
    }

    // Nothing was written and no AI work started before 저장: the screens only read.
    expect(calls.filter((call) => /Create|Update|Save|Delete|Start/.test(call))).toEqual([])
    expect(screen.getByRole('article', { name: '미리보기' })).toHaveTextContent('찾아간 이유')
    await user.click(screen.getByRole('button', { name: '저장' }))
    expect(await screen.findByText('템플릿 이름을 적어 주세요.')).toBeInTheDocument()
    await user.type(screen.getByLabelText('템플릿 이름'), '내 맛집 리뷰')
    await user.click(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/templates/template-1'))
    expect(creates).toHaveLength(1)
    const saved = creates[0]!
    expect(saved).toMatchObject({ name: '내 맛집 리뷰', description: '', titleArea: '' })
    expect(
      saved.body.startsWith('찾아간 이유\n<write>이곳을 찾게 된 이유와 기대를 씁니다</write>'),
    ).toBe(true)
    expect(saved.body).toContain(
      '주문한 메뉴\n<repeat each="photo">\n<write>주문한 메뉴의 모습과 맛을 씁니다</write>\n<slot kind="photo"/>\n</repeat>',
    )
    expect(saved.body).toContain('아쉬운 점\n<write>')
    expect(saved.body.endsWith('매장 정보\n<ask label="매장 정보" required="true"/>')).toBe(true)
    expect(saved.body).not.toContain('서비스·응대')
    expect(parseTemplate('', saved.body, TEMPLATE_PARSE_OPTIONS).ok).toBe(true)
  })

  it('asks before discarding the answers when the owner leaves', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/templates/new', {
      user: { id: 'alice' },
      templates: { templates: [] },
    })
    await title('어떤 체험을 글로 남기나요?')
    await user.click(screen.getByRole('button', { name: /^카페/ }))
    await user.click(screen.getByRole('link', { name: '← 템플릿 목록' }))

    expect(
      await screen.findByRole('dialog', { name: '템플릿 만들기를 그만둘까요?' }),
    ).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/templates/new')
    await user.click(screen.getByRole('button', { name: '그만두기' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/templates'))
  })
})
