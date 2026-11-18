import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, it } from 'vitest'
import { readClipReturnContext } from '@/entities/clip-project'
import { forgetLastTemplates } from '@/entities/template'
import { forgetCreationModes } from '@/widgets/writing-kit'
import { Stage } from '@/shared/api'
import { renderAppAt } from '@/test/app'
import { templateField } from '@/test/editor'

beforeEach(() => {
  sessionStorage.clear()
  forgetLastTemplates()
  forgetCreationModes()
})

/** A piece's ring: the way to its screen, described by the piece's state and its one fact. */
const ring = (name: string) => screen.getByRole('link', { name: `${name} 화면으로 가기` })
const fact = async (name: string, value: RegExp) =>
  waitFor(() => expect(ring(name)).toHaveAccessibleDescription(value))

/** Every element of the picture works one way: its icon goes, its name chooses. A piece's ring
 *  opens its screen with 홈 as the way back, by the screen's ← and by history; the name of a piece
 *  with a default is ONE button with its chevron, which opens the dropdown and goes nowhere. 기억
 *  has no default (MEM-29), so its ring and its name are the one way to its screen. */
it('opens each screen from its ring only, and each default from its name', async () => {
  const user = userEvent.setup()
  const view = renderAppAt('/', { user: { id: 'alice' } })
  const kit = within(await screen.findByRole('list', { name: '글쓰기 준비' }))
  for (const [name, href] of [
    ['말투', '/voices'],
    ['템플릿', '/templates'],
    ['지침', '/guidelines'],
    ['글 쓰는 AI', '/ai-models#ai-in-use'],
    ['기억', '/memories'],
  ])
    expect(kit.getByRole('link', { name: `${name} 화면으로 가기` })).toHaveAttribute('href', href)
  expect(kit.getAllByRole('link')).toHaveLength(5)
  // Each name and its chevron are one trigger; 기억 has none.
  const triggers = kit.getAllByRole('button')
  expect(triggers.map((button) => button.getAttribute('aria-label'))).toEqual([
    '말투 기본값 바꾸기',
    '템플릿 기본값 바꾸기',
    '기본 지침 바꾸기',
    '글 쓰는 AI 바꾸기',
  ])
  expect(triggers.map((button) => button.textContent)).toEqual([
    '말투',
    '템플릿',
    '지침',
    '글 쓰는 AI',
  ])
  for (const trigger of triggers) expect(trigger.querySelector('svg')).not.toBeNull()

  // The name opens its dropdown in place, and the chevron turns with `aria-expanded`.
  const voiceName = kit.getByRole('button', { name: '말투 기본값 바꾸기' })
  expect(voiceName).toHaveAttribute('aria-expanded', 'false')
  await user.click(within(voiceName).getByText('말투'))
  expect(await screen.findByRole('dialog', { name: '말투 기본값 바꾸기' })).toBeInTheDocument()
  expect(voiceName).toHaveAttribute('aria-expanded', 'true')
  expect(view.router.state.location.pathname).toBe('/')
  await user.keyboard('{Escape}')
  expect(voiceName).toHaveAttribute('aria-expanded', 'false')

  for (const [name, path] of [
    ['말투', '/voices'],
    ['템플릿', '/templates'],
    ['지침', '/guidelines'],
    ['기억', '/memories'],
  ]) {
    await user.click(ring(name))
    await screen.findByRole('heading', { level: 1, name })
    expect(view.router.state.location.pathname).toBe(path)
    expect(screen.queryByRole('dialog')).toBeNull()
    await user.click(within(screen.getByRole('main')).getByRole('link', { name: '홈' }))
    await waitFor(() => expect(view.router.state.location.pathname).toBe('/'))

    await user.click(await screen.findByRole('link', { name: `${name} 화면으로 가기` }))
    await screen.findByRole('heading', { level: 1, name })
    act(() => view.router.history.back())
    await waitFor(() => expect(view.router.state.location.pathname).toBe('/'))
    await screen.findByRole('list', { name: '글쓰기 준비' })
  }
})

/** The centre works as the pieces do: its icon starts what it names, and its name switches that
 *  between 글쓰기 and 영상 만들기. 영상 starts the clip workspace with the video list as its way
 *  back, and the switch is remembered for the account, so 홈 opens on it next time. */
it('switches the centre to 영상 만들기 from its name, and opens on it next time', async () => {
  const user = userEvent.setup()
  const first = renderAppAt('/', { user: { id: 'alice' } })
  const main = within(await screen.findByRole('main'))
  expect(await main.findByRole('link', { name: '글쓰기' })).toHaveAttribute('href', '/posts/new')
  expect(main.queryByRole('link', { name: '영상 만들기' })).toBeNull()

  await user.click(main.getByRole('button', { name: '글쓰기, 만들 것 바꾸기' }))
  const panel = within(await screen.findByRole('dialog', { name: '무엇을 만들까요' }))
  expect(panel.getByRole('radio', { name: /^글쓰기/ })).toHaveAttribute('aria-checked', 'true')
  await user.click(panel.getByRole('radio', { name: /^영상 만들기/ }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(main.getByRole('button', { name: '영상 만들기, 만들 것 바꾸기' })).toHaveFocus()
  expect(main.getByRole('link', { name: '영상 만들기' })).toHaveAttribute('href', '/clips/new')
  expect(main.queryByRole('link', { name: '글쓰기' })).toBeNull()

  first.unmount()
  const second = renderAppAt('/', { transport: first.transport })
  const video = await screen.findByRole('link', { name: '영상 만들기' })
  await user.click(video)
  await waitFor(() => expect(second.router.state.location.pathname).toBe('/clips/new'))
  expect(readClipReturnContext('alice')).toMatchObject({ path: '/clips' })
})

it('changes the writing AI from its name with the tournament picker, and closes once saved', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/', {
    user: { id: 'alice' },
    providers: {
      calls,
      models: [
        { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
        { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer-a' }],
    },
  })
  await fact('글 쓰는 AI', /Writer A/)
  const trigger = screen.getByRole('button', { name: '글 쓰는 AI 바꾸기' })
  expect(trigger).toHaveAttribute('aria-haspopup', 'dialog')
  await user.click(trigger)
  const panel = within(await screen.findByRole('dialog', { name: '글 쓰는 AI 바꾸기' }))
  expect(panel.getByRole('radio', { name: /Writer A/ })).toHaveAttribute('aria-checked', 'true')

  await user.click(panel.getByRole('radio', { name: /Writer B/ }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(calls.filter((call) => call === 'SaveSelection')).toHaveLength(1)
  expect(trigger).toHaveFocus()
  await fact('글 쓰는 AI', /^준비됨 Writer B$/)
})

it('keeps the dropdown open on a refused save, saying so, with the previous choice still checked', async () => {
  const user = userEvent.setup()
  renderAppAt('/', {
    user: { id: 'alice' },
    providers: {
      saveFails: true,
      models: [
        { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
        { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer-a' }],
    },
  })
  await user.click(await screen.findByRole('button', { name: '글 쓰는 AI 바꾸기' }))
  const panel = within(await screen.findByRole('dialog', { name: '글 쓰는 AI 바꾸기' }))
  await user.click(panel.getByRole('radio', { name: /Writer B/ }))
  expect(await panel.findByRole('alert')).toBeInTheDocument()
  expect(panel.getByRole('radio', { name: /Writer A/ })).toHaveAttribute('aria-checked', 'true')
  await fact('글 쓰는 AI', /^준비됨 Writer A$/)
})

it('makes a finished voice the default from its name, and lists one still being made unchosen', async () => {
  const user = userEvent.setup()
  const calls: string[] = []
  renderAppAt('/', {
    user: { id: 'alice' },
    voice: {
      calls,
      voices: [
        { id: 'blog', name: '블로그 말투', isDefault: true },
        { id: 'diary', name: '일기 말투' },
        { id: 'draft', name: '만드는 말투', made: false },
      ],
    },
  })
  await screen.findByRole('list', { name: '글쓰기 준비' })
  await fact('말투', /^준비됨 블로그 말투$/)
  await user.click(screen.getByRole('button', { name: '말투 기본값 바꾸기' }))
  const panel = within(await screen.findByRole('dialog', { name: '말투 기본값 바꾸기' }))
  expect(panel.getByRole('radio', { name: '블로그 말투' })).toHaveAttribute('aria-checked', 'true')
  const making = panel.getByRole('radio', { name: /만드는 말투 학습 \d+%/ })
  expect(making).toBeDisabled()

  await user.click(panel.getByRole('radio', { name: '일기 말투' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(calls.filter((call) => call === 'SetDefaultVoice')).toHaveLength(1)
  await fact('말투', /^준비됨 일기 말투$/)
})

it('switches several 기본 지침 in one visit to its dropdown, saving each press', async () => {
  const user = userEvent.setup()
  renderAppAt('/', {
    user: { id: 'alice' },
    guidelines: {
      defaults: [
        { key: 'facts', name: '재료에 있는 사실만', text: '재료에 없는 사실은 쓰지 않아요.' },
        { key: 'names', name: '메모의 이름으로', text: '메모의 이름을 써요.', enabled: false },
        { key: 'rule', name: '원칙', text: '늘 지켜요.', principle: true },
      ],
    },
  })
  await user.click(await screen.findByRole('button', { name: '기본 지침 바꾸기' }))
  const panel = within(await screen.findByRole('dialog', { name: '기본 지침 바꾸기' }))
  const facts = await panel.findByRole('checkbox', { name: '재료에 있는 사실만' })
  const names = panel.getByRole('checkbox', { name: '메모의 이름으로' })
  // The 원칙 is always applied and never switched (GUIDE-51).
  expect(panel.queryByRole('checkbox', { name: '원칙' })).toBeNull()
  expect(facts).toBeChecked()
  expect(names).not.toBeChecked()

  await user.click(names)
  await user.click(facts)
  await waitFor(() => expect(names).toBeChecked())
  await waitFor(() => expect(facts).not.toBeChecked())
  expect(screen.getByRole('dialog', { name: '기본 지침 바꾸기' })).toBeInTheDocument()
})

/** What is chosen in a dropdown is saved where the product keeps it — the server for 말투, 지침 and
 *  글 쓰는 AI, this browser for 템플릿 — so a reload, a fresh app with an empty cache over the same
 *  account, shows it in the picture and in the dropdown, and the next new post starts on the
 *  template. */
it('keeps what was chosen in each dropdown through a reload, and starts the next post on the template', async () => {
  const user = userEvent.setup()
  const first = renderAppAt('/', {
    user: { id: 'alice' },
    voice: {
      voices: [
        { id: 'blog', name: '블로그 말투', isDefault: true },
        { id: 'diary', name: '일기 말투' },
      ],
    },
    templates: {
      templates: [
        { id: 'template-review', name: '식당 리뷰' },
        { id: 'template-diary', name: '하루 일기' },
      ],
    },
    guidelines: {
      defaults: [
        { key: 'facts', name: '재료에 있는 사실만', text: '재료에 없는 사실은 쓰지 않아요.' },
      ],
    },
    providers: {
      models: [
        { providerId: 'openrouter', modelId: 'writer-a', label: 'Writer A' },
        { providerId: 'openrouter', modelId: 'writer-b', label: 'Writer B' },
      ],
      selections: [{ stage: Stage.WRITE, providerId: 'openrouter', modelId: 'writer-a' }],
    },
  })
  const choose = async (control: string, role: 'radio' | 'checkbox', name: string | RegExp) => {
    await user.click(await screen.findByRole('button', { name: control }))
    const panel = within(await screen.findByRole('dialog', { name: control }))
    await user.click(await panel.findByRole(role, { name }))
  }
  await choose('말투 기본값 바꾸기', 'radio', '일기 말투')
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  // 없음 is where a template starts until one is specified.
  await user.click(screen.getByRole('button', { name: '템플릿 기본값 바꾸기' }))
  expect(
    await within(screen.getByRole('dialog')).findByRole('radio', { name: '없음' }),
  ).toHaveAttribute('aria-checked', 'true')
  await user.keyboard('{Escape}')
  await choose('템플릿 기본값 바꾸기', 'radio', '하루 일기')
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  await choose('글 쓰는 AI 바꾸기', 'radio', /Writer B/)
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  await choose('기본 지침 바꾸기', 'checkbox', '재료에 있는 사실만')
  await waitFor(() =>
    expect(screen.getByRole('checkbox', { name: '재료에 있는 사실만' })).not.toBeChecked(),
  )
  await user.keyboard('{Escape}')

  first.unmount()
  const second = renderAppAt('/', { transport: first.transport })
  await screen.findByRole('list', { name: '글쓰기 준비' })
  await fact('말투', /^준비됨 일기 말투$/)
  await fact('템플릿', /^준비됨 하루 일기 외 1개$/)
  await fact('글 쓰는 AI', /^준비됨 Writer B$/)
  const checked = async (control: string, role: 'radio' | 'checkbox', name: string | RegExp) => {
    await user.click(screen.getByRole('button', { name: control }))
    const option = await within(await screen.findByRole('dialog', { name: control })).findByRole(
      role,
      { name },
    )
    const state = role === 'radio' ? option.getAttribute('aria-checked') === 'true' : option
    await user.keyboard('{Escape}')
    return state
  }
  expect(await checked('말투 기본값 바꾸기', 'radio', '일기 말투')).toBe(true)
  expect(await checked('템플릿 기본값 바꾸기', 'radio', '하루 일기')).toBe(true)
  expect(await checked('글 쓰는 AI 바꾸기', 'radio', /Writer B/)).toBe(true)
  await user.click(screen.getByRole('button', { name: '기본 지침 바꾸기' }))
  expect(
    await within(await screen.findByRole('dialog')).findByRole('checkbox', {
      name: '재료에 있는 사실만',
    }),
  ).not.toBeChecked()

  second.unmount()
  renderAppAt('/posts/new', { transport: first.transport })
  const picker = within(await templateField(user))
  expect(await picker.findByRole('button', { name: '하루 일기' })).toHaveAttribute(
    'aria-pressed',
    'true',
  )
})
