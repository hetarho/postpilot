import { cleanup, screen, within } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import { renderAppAt } from '@/test/app'

afterEach(() => {
  cleanup()
  sessionStorage.clear()
})

// 홈: the promise, the writing kit drawn from the owner's own lists, and the one 글쓰기. An empty
// kit offers its first missing element beside 글쓰기; nothing here writes or calls a model.
it('opens an empty kit with each element a way in, the first missing one offered, and 글쓰기', async () => {
  const calls: string[] = []
  renderAppAt('/', { user: { id: 'alice' }, calls, voice: { voices: [] } })
  expect(
    await screen.findByRole('heading', { level: 1, name: /고품질의 글을/ }),
  ).toBeInTheDocument()
  const kit = within(screen.getByRole('figure', { name: '글쓰기 키트' }))
  for (const [name, href] of [
    ['말투', '/voices'],
    ['템플릿', '/templates'],
    ['지침', '/guidelines'],
    ['기억', '/memories'],
  ])
    expect(await kit.findByRole('link', { name: new RegExp('^' + name) })).toHaveAttribute(
      'href',
      href,
    )
  expect(await screen.findByRole('status')).toHaveTextContent('아직 채운 것이 없어요')
  expect(screen.getByRole('link', { name: '말투 만들기' })).toHaveAttribute('href', '/voices?new=1')
  expect(screen.getByRole('link', { name: '글쓰기' })).toHaveAttribute('href', '/posts/new')
  expect(calls.filter((call) => /^(Create|Start|Generate|Save|Analyze)/.test(call))).toEqual([])
})

it('names what the kit holds in the owner’s own words once it is filled in', async () => {
  renderAppAt('/', {
    user: { id: 'alice' },
    voice: { voices: [{ id: 'voice-default', name: '다정한 말투', isDefault: true }] },
    templates: { templates: [{ id: 'review', name: '맛집 리뷰' }] },
    memories: { memories: [{ id: 'm', text: '매운 음식을 못 먹는다' }] },
  })
  const kit = within(await screen.findByRole('figure', { name: '글쓰기 키트' }))
  expect(await kit.findByText('다정한 말투')).toBeInTheDocument()
  expect(await kit.findByText('맛집 리뷰')).toBeInTheDocument()
  expect(await kit.findByText('1개 기억')).toBeInTheDocument()
  // Three of four are ready; 지침 holds only what stands in for the owner's own.
  expect(await screen.findByText('4개 중 3개 준비됐어요')).toBeInTheDocument()
  expect(screen.getByRole('link', { name: '지침 쓰기' })).toHaveAttribute(
    'href',
    '/guidelines?new=1',
  )
})
