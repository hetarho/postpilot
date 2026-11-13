import { useEffect, useState } from 'react'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { ClipPreparationScreen } from '@/features/edit-clip-project'
import { Button, TextField } from '@/shared/ui'
import { preparationOpening } from '../model/steps'
import { ClipObserveBar } from './ClipObserveBar'
import { ClipPreparationGuide } from './ClipPreparationGuide'

function Mounted({ mounted }: { mounted: () => void }) {
  useEffect(mounted, [mounted])
  return null
}
function Approval() {
  const [approved, approve] = useState(false)
  return (
    <>
      <Button variant="secondary" onClick={() => approve(true)}>
        스토리라인 먼저
      </Button>
      <Button variant="cta" onClick={() => approve(true)}>
        {approved ? '승인 확인 중' : '바로 만들기'}
      </Button>
    </>
  )
}
const EMPTY = {
  kind: null,
  requirements: null,
  sources: null,
  template: null,
  content: null,
  style: null,
  note: null,
}
function Fixture({
  active = true,
  campaign = false,
  onSubmit = () => {},
  onNext = () => {},
  mounted = () => {},
}: {
  active?: boolean
  campaign?: boolean
  onSubmit?: () => void
  onNext?: () => void
  mounted?: () => void
}) {
  const [step, select] = useState<ClipPreparationScreen>('kind')
  const [value, change] = useState('')
  const [note, write] = useState('')
  return (
    <form
      hidden={!active}
      onSubmit={(event) => {
        event.preventDefault()
        onSubmit()
      }}
    >
      <ClipPreparationGuide
        screen={step}
        onSelect={select}
        campaign={campaign}
        requirements={
          campaign
            ? {
                coverage: {
                  keywords: [
                    { keyword: '성수카페', held: !!value },
                    { keyword: '라떼', held: false },
                  ],
                },
                hashtags: ['성수'],
              }
            : undefined
        }
        noteEmpty={!note.trim()}
        screens={{
          ...EMPTY,
          kind: <p>종류 선택</p>,
          requirements: <TextField aria-label="광고주 가이드" />,
          sources: (
            <>
              <Mounted mounted={mounted} />
              <TextField aria-label="원본 파일" type="file" />
            </>
          ),
          content: (
            <TextField
              aria-label="제목"
              value={value}
              onChange={(event) => change(event.target.value)}
            />
          ),
          style: <TextField aria-label="길이" type="number" defaultValue="30" />,
          note: (
            <TextField
              aria-label="덧붙임"
              value={note}
              onChange={(event) => write(event.target.value)}
            />
          ),
        }}
        rows={[{ id: 'title', label: '제목', value: value || '비었어요', screen: 'content' }]}
        models={<TextField aria-label="선택 모델" defaultValue="저장된 모델" />}
        onNext={onNext}
      />
    </form>
  )
}
const progress = () => screen.getByRole('navigation', { name: '클립 만들기 단계' })
async function go(name: string) {
  await userEvent.click(within(progress()).getByRole('button', { name }))
}

describe('retained clip preparation questions', () => {
  it('asks the post guide order one screen at a time, with positions, and never submits', async () => {
    const submit = vi.fn(),
      mounted = vi.fn()
    render(<Fixture onSubmit={submit} mounted={mounted} />)
    expect(
      within(progress())
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['영상 종류', '영상', '템플릿', '내용', '스타일', '덧붙임', '확인'])
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('어떤 영상을 만드시나요?')
    expect(screen.getByText('1 / 7')).toBeVisible()
    expect(screen.queryByRole('button', { name: '바로 만들기' })).toBeNull()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: '다음' }))
    expect(screen.getByRole('heading', { level: 2 })).toHaveFocus()
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('어떤 영상으로 만들까요?')
    expect(screen.getByText('2 / 7')).toBeVisible()
    await go('내용')
    expect(screen.getByText('4 / 7')).toBeVisible()
    await user.type(screen.getByRole('textbox', { name: '제목' }), '주인이 쓴 제목')
    expect(screen.getByRole('textbox', { name: '제목' })).toHaveFocus()
    await go('스타일')
    await user.click(screen.getByRole('button', { name: '내용로 이전' }))
    expect(screen.getByRole('textbox', { name: '제목' })).toHaveValue('주인이 쓴 제목')
    const review = within(progress()).getByRole('button', { name: '확인' })
    review.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByText('7 / 7')).toBeVisible()
    expect(review).toHaveAttribute('aria-current', 'step')
    expect(screen.getByText('주인이 쓴 제목')).toBeVisible()
    expect(screen.getByRole('button', { name: '덧붙임로 이전' })).toBeVisible()
    expect(document.querySelectorAll('[data-action-bar=step]')).toHaveLength(1)
    expect(screen.queryByRole('complementary', { name: '광고주 요청 챙기기' })).toBeNull()
    expect(submit).not.toHaveBeenCalled()
    expect(mounted).toHaveBeenCalledTimes(1)
  })

  it('adds 요청 and the tracker on every screen after it while sponsored', async () => {
    render(<Fixture campaign />)
    expect(
      within(progress())
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['영상 종류', '요청', '영상', '템플릿', '내용', '스타일', '덧붙임', '확인'])
    expect(screen.getByText('1 / 8')).toBeVisible()
    const tracker = () => screen.queryByRole('complementary', { name: '광고주 요청 챙기기' })
    expect(tracker()).toBeNull()
    await go('요청')
    expect(screen.getByText('2 / 8')).toBeVisible()
    expect(tracker()).toBeNull()
    await go('영상')
    expect(tracker()).toHaveTextContent('입력한 키워드 0 / 2')
    expect(within(tracker()!).queryByRole('list')).toBeNull()
    await go('내용')
    await userEvent.type(screen.getByRole('textbox', { name: '제목' }), '성수 카페')
    expect(tracker()).toHaveTextContent('입력한 키워드 1 / 2')
    expect(within(tracker()!).getByRole('list', { name: '입력한 내용의 키워드' })).toBeVisible()
    await go('확인')
    expect(screen.getByText('아직 입력하지 않은 키워드: 라떼')).toBeVisible()
    expect(screen.getByText(/업로드 문구에서 필수 해시태그를 확인해요: #성수/)).toBeVisible()
    await userEvent.click(within(tracker()!).getByRole('button', { name: '광고주 요청 바꾸기' }))
    expect(screen.getByText('2 / 8')).toBeVisible()
  })

  it('reads 건너뛰고 확인 on an empty note and 확인하기 once it holds text', async () => {
    render(<Fixture />)
    await go('덧붙임')
    expect(screen.getByRole('button', { name: '건너뛰고 확인' })).toBeVisible()
    await userEvent.type(screen.getByRole('textbox', { name: '덧붙임' }), '라떼 위주로')
    await userEvent.click(screen.getByRole('button', { name: '확인하기' }))
    expect(screen.getByText('7 / 7')).toBeVisible()
  })

  it('reads pending IME input, has no duplicate editable summary, and returns to the original control', async () => {
    render(<Fixture />)
    await go('내용')
    const input = screen.getByRole('textbox', { name: '제목' })
    fireEvent.compositionStart(input)
    fireEvent.input(input, { target: { value: '아직 저장 안 된 한글' }, isComposing: true })
    await go('확인')
    expect(screen.getByText('아직 저장 안 된 한글')).toBeVisible()
    expect(screen.queryByRole('textbox', { name: '제목' })).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: '제목 바꾸기' }))
    expect(screen.getByRole('textbox', { name: '제목' })).toBe(input)
    expect(input).toHaveValue('아직 저장 안 된 한글')
  })

  it('ends 확인 on 이전 and 다음, which moves on without submitting', async () => {
    const submit = vi.fn(),
      next = vi.fn()
    render(<Fixture onSubmit={submit} onNext={next} />)
    await go('확인')
    expect(screen.queryByRole('button', { name: '바로 만들기' })).toBeNull()
    expect(screen.getByRole('button', { name: '덧붙임로 이전' })).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: '다음' }))
    expect(next).toHaveBeenCalledOnce()
    expect(submit).not.toHaveBeenCalled()
  })

  it('retains cursor, fields and actor mounts across lifecycle visits, resetting at the keyed boundary', async () => {
    const mounted = vi.fn()
    const view = render(<Fixture key="alice/one" mounted={mounted} />)
    await go('확인')
    view.rerender(<Fixture key="alice/one" active={false} mounted={mounted} />)
    expect(screen.queryByRole('heading')).toBeNull()
    view.rerender(<Fixture key="alice/one" active mounted={mounted} />)
    expect(screen.getByText('7 / 7')).toBeVisible()
    await go('영상')
    await go('확인')
    expect(screen.getByText('7 / 7')).toBeVisible()
    expect(mounted).toHaveBeenCalledTimes(1)
    view.rerender(<Fixture key="alice/two" mounted={mounted} />)
    expect(screen.getByText('1 / 7')).toBeVisible()
    await go('내용')
    await userEvent.type(screen.getByRole('textbox', { name: '제목' }), 'alice only')
    view.rerender(<Fixture key="bob/two" mounted={mounted} />)
    await go('내용')
    expect(screen.getByRole('textbox', { name: '제목' })).toHaveValue('')
  })
})

it('opens on 확인 for an instruction, a source or a running job, otherwise on 영상 종류', () => {
  expect(preparationOpening({ sources: 0, running: false })).toBe('kind')
  expect(preparationOpening({ instruction: '  ', sources: 0, running: false })).toBe('kind')
  expect(preparationOpening({ instruction: '라떼', sources: 0, running: false })).toBe('review')
  expect(preparationOpening({ sources: 1, running: false })).toBe('review')
  expect(preparationOpening({ sources: 0, running: true })).toBe('review')
})

it('holds an unminted 확인 on its reason with the way to fix it and no 다음', async () => {
  const select = vi.fn()
  render(
    <ClipPreparationGuide
      screen="review"
      onSelect={select}
      campaign={false}
      noteEmpty
      screens={EMPTY}
      rows={[]}
      models={null}
      correction={{ screen: 'sources', label: '먼저 영상을 넣어 주세요.' }}
    />,
  )
  expect(screen.getByText('먼저 영상을 넣어 주세요.')).toBeVisible()
  expect(screen.queryByRole('button', { name: '다음' })).toBeNull()
  await userEvent.click(screen.getByRole('button', { name: '바로잡기' }))
  expect(select).toHaveBeenCalledWith('sources')
})

describe('② 관찰 step bar', () => {
  it('holds 이전 and the quoted actions in one row, with the refusal above it', async () => {
    const back = vi.fn()
    render(
      <ClipObserveBar
        reasonId="observe-reason"
        refusal={<p role="alert">저장 실패</p>}
        onBack={back}
        onRoute={vi.fn()}
        actions={<Approval />}
      />,
    )
    expect(screen.getByRole('alert')).toHaveTextContent('저장 실패')
    await userEvent.click(screen.getByRole('button', { name: '바로 만들기' }))
    expect(screen.getByRole('button', { name: '승인 확인 중' })).toBeVisible()
    await userEvent.click(screen.getByRole('button', { name: '이전: 생성' }))
    expect(back).toHaveBeenCalledOnce()
  })

  it('routes an input refusal back to its ① screen', async () => {
    const route = vi.fn()
    render(
      <ClipObserveBar
        reasonId="input-reason"
        correction={{ screen: 'content', label: '입력을 확인해 주세요.' }}
        onBack={vi.fn()}
        onRoute={route}
        actions={
          <Button disabled aria-describedby="input-reason">
            바로 만들기
          </Button>
        }
      />,
    )
    expect(screen.getByRole('button', { name: '바로 만들기' })).toHaveAccessibleDescription(
      '입력을 확인해 주세요.',
    )
    await userEvent.click(screen.getByRole('button', { name: '바로잡기' }))
    expect(route).toHaveBeenCalledWith('content')
  })

  it('retries a region refusal from its own editor', async () => {
    const fix = vi.fn()
    render(
      <ClipObserveBar
        reasonId="region-refusal"
        correction={{ retry: fix, label: '스토리라인 문구를 바로잡아 주세요.' }}
        onBack={vi.fn()}
        onRoute={vi.fn()}
        actions={
          <Button disabled aria-describedby="region-refusal">
            바로 만들기
          </Button>
        }
      />,
    )
    await userEvent.click(screen.getByRole('button', { name: '바로잡기' }))
    expect(fix).toHaveBeenCalledOnce()
  })

  it('names the wait for running work without offering a fix that does nothing', () => {
    render(
      <ClipObserveBar
        reasonId="busy-refusal"
        correction={{ label: '진행 중인 작업이 끝나면 시작할 수 있어요.' }}
        onBack={vi.fn()}
        onRoute={vi.fn()}
        actions={
          <Button disabled aria-describedby="busy-refusal">
            바로 만들기
          </Button>
        }
      />,
    )
    expect(screen.getByRole('button', { name: '바로 만들기' })).toHaveAccessibleDescription(
      '진행 중인 작업이 끝나면 시작할 수 있어요.',
    )
    expect(screen.queryByRole('button', { name: '바로잡기' })).not.toBeInTheDocument()
  })
})
