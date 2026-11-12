import { useEffect, useState } from 'react'
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { ClipPreparationScreen } from '@/features/edit-clip-project'
import { Button, TextField } from '@/shared/ui'
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
function Fixture({
  active = true,
  refusal,
  onSubmit = () => {},
  mounted = () => {},
}: {
  active?: boolean
  refusal?: 'sources' | 'style' | 'save'
  onSubmit?: () => void
  mounted?: () => void
}) {
  const [step, select] = useState<ClipPreparationScreen>('sources')
  const [value, change] = useState('')
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
        screens={{
          sources: (
            <>
              <Mounted mounted={mounted} />
              <TextField aria-label="원본 파일" type="file" />
            </>
          ),
          story: (
            <TextField
              aria-label="제목"
              value={value}
              onChange={(event) => change(event.target.value)}
            />
          ),
          style: <TextField aria-label="길이" type="number" defaultValue="30" />,
        }}
        rows={[{ id: 'title', label: '제목', value: value || '비었어요', screen: 'story' }]}
        models={<TextField aria-label="선택 모델" defaultValue="저장된 모델" />}
        refusal={refusal === 'save' ? <p role="alert">저장 실패</p> : undefined}
        correction={
          refusal
            ? { screen: refusal === 'save' ? 'story' : refusal, label: '입력을 확인해 주세요.' }
            : undefined
        }
        actions={<Approval />}
      />
    </form>
  )
}
async function go(name: string) {
  await userEvent.click(
    within(screen.getByRole('navigation', { name: '클립 만들기 단계' })).getByRole('button', {
      name,
    }),
  )
}

describe('retained clip preparation questions', () => {
  it('uses one visible question, truthful positions, native pointer/keyboard navigation and one dock without submitting', async () => {
    const submit = vi.fn(),
      mounted = vi.fn()
    render(<Fixture onSubmit={submit} mounted={mounted} />)
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('어떤 영상으로 만들까요?')
    expect(screen.getByText('1 / 4')).toBeVisible()
    expect(screen.queryByRole('button', { name: '바로 만들기' })).toBeNull()
    const user = userEvent.setup()
    await user.click(screen.getByRole('button', { name: '다음' }))
    expect(screen.getByRole('heading', { level: 2 })).toHaveFocus()
    expect(screen.getByText('2 / 4')).toBeVisible()
    await user.type(screen.getByRole('textbox', { name: '제목' }), '주인이 쓴 제목')
    expect(screen.getByRole('textbox', { name: '제목' })).toHaveFocus()
    await go('스타일')
    await user.click(screen.getByRole('button', { name: '이야기로 이전' }))
    expect(screen.getByRole('textbox', { name: '제목' })).toHaveValue('주인이 쓴 제목')
    const review = within(screen.getByRole('navigation')).getByRole('button', { name: '확인' })
    review.focus()
    await user.keyboard('{Enter}')
    expect(screen.getByText('4 / 4')).toBeVisible()
    expect(review).toHaveAttribute('aria-current', 'step')
    expect(screen.getByText('주인이 쓴 제목')).toBeVisible()
    expect(document.querySelectorAll('[data-action-bar=step]')).toHaveLength(1)
    expect(submit).not.toHaveBeenCalled()
    expect(mounted).toHaveBeenCalledTimes(1)
  })

  it('reads pending IME input, has no duplicate editable summary, and returns to the original control', async () => {
    render(<Fixture />)
    await go('이야기')
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

  it.each(['sources', 'style', 'save'] as const)(
    'keeps a %s refusal actionable and navigation available',
    async (refusal) => {
      const submit = vi.fn()
      render(<Fixture refusal={refusal} onSubmit={submit} />)
      await go('확인')
      expect(screen.getByText('입력을 확인해 주세요.')).toBeVisible()
      if (refusal === 'save') expect(screen.getByRole('alert')).toHaveTextContent('저장 실패')
      await userEvent.click(screen.getByRole('button', { name: '바로잡기' }))
      expect(screen.getByRole('button', { name: '다음' })).toBeEnabled()
      expect(submit).not.toHaveBeenCalled()
    },
  )

  it('retains cursor, approval state, fields and actor mounts across lifecycle visits, resetting at the keyed boundary', async () => {
    const mounted = vi.fn()
    const view = render(<Fixture key="alice/one" mounted={mounted} />)
    await go('확인')
    await userEvent.click(screen.getByRole('button', { name: '바로 만들기' }))
    view.rerender(<Fixture key="alice/one" active={false} mounted={mounted} />)
    expect(screen.queryByRole('heading')).toBeNull()
    view.rerender(<Fixture key="alice/one" active mounted={mounted} />)
    expect(screen.getByText('4 / 4')).toBeVisible()
    expect(screen.getByRole('button', { name: '승인 확인 중' })).toBeVisible()
    await go('소스')
    await go('확인')
    expect(screen.getByRole('button', { name: '승인 확인 중' })).toBeVisible()
    expect(mounted).toHaveBeenCalledTimes(1)
    view.rerender(<Fixture key="alice/two" mounted={mounted} />)
    expect(screen.getByText('1 / 4')).toBeVisible()
    await go('이야기')
    await userEvent.type(screen.getByRole('textbox', { name: '제목' }), 'alice only')
    view.rerender(<Fixture key="bob/two" mounted={mounted} />)
    await go('이야기')
    expect(screen.getByRole('textbox', { name: '제목' })).toHaveValue('')
  })
})

it('describes a held action and routes a region refusal to its original editor without submitting', async () => {
  const fix = vi.fn(),
    submit = vi.fn()
  render(
    <form
      onSubmit={(event) => {
        event.preventDefault()
        submit()
      }}
    >
      <ClipPreparationGuide
        screen="review"
        onSelect={vi.fn()}
        screens={{ sources: null, story: null, style: null }}
        rows={[]}
        models={null}
        reasonId="region-refusal"
        correction={{ retry: fix, label: '스토리라인 문구를 바로잡아 주세요.' }}
        actions={
          <Button disabled aria-describedby="region-refusal">
            바로 만들기
          </Button>
        }
      />
    </form>,
  )
  expect(screen.getByRole('button', { name: '바로 만들기' })).toHaveAccessibleDescription(
    '스토리라인 문구를 바로잡아 주세요.',
  )
  await userEvent.click(screen.getByRole('button', { name: '바로잡기' }))
  expect(fix).toHaveBeenCalledOnce()
  expect(submit).not.toHaveBeenCalled()
})

it('names the wait for running work without offering a fix that does nothing', () => {
  render(
    <ClipPreparationGuide
      screen="review"
      onSelect={vi.fn()}
      screens={{ sources: null, story: null, style: null }}
      rows={[]}
      models={null}
      reasonId="busy-refusal"
      correction={{ label: '진행 중인 작업이 끝나면 시작할 수 있어요.' }}
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
