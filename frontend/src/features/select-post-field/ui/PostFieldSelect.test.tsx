import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Code } from '@connectrpc/connect'
import { afterEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { connectAppError } from '@/test/app-error'
import { PostFieldSelect } from './PostFieldSelect'

afterEach(() => {
  cleanup()
  initializeI18n('ko')
})

it('offers 없음 and then the nine 분야 in catalogue order, under a visible label', async () => {
  const user = userEvent.setup()
  render(<PostFieldSelect value="" onSelect={() => {}} />)

  expect(screen.getByText('분야', { selector: 'label' })).not.toHaveClass('sr-only')
  const picker = screen.getByRole('combobox', { name: '분야 없음' })
  // What the choice feeds, and nothing it cannot promise (QUAL-19).
  expect(picker).toHaveAccessibleDescription(
    '다음 생성부터 이 분야의 지침과, 네이버 검색 결과의 제목·설명에서 자주 보인 표현을 함께 참고해요.',
  )
  await user.click(picker)
  expect(screen.getAllByRole('option').map((option) => option.textContent)).toEqual([
    '없음',
    '맛집',
    '카페',
    '국내여행',
    '패션·미용',
    '상품리뷰',
    '육아·결혼',
    '반려동물',
    '인테리어·DIY',
    '일상·생각',
  ])
})

it('holds the trigger disabled while the choice is out', async () => {
  const user = userEvent.setup()
  let land = () => {}
  const onSelect = vi.fn(() => new Promise<void>((resolve) => (land = resolve)))
  render(<PostFieldSelect value="" onSelect={onSelect} />)

  const picker = screen.getByRole('combobox', { name: '분야 없음' })
  await user.click(picker)
  await user.click(screen.getByRole('option', { name: '카페' }))

  expect(onSelect).toHaveBeenCalledWith('cafe')
  expect(picker).toBeDisabled()
  land()
  await waitFor(() => expect(picker).toBeEnabled())
  expect(screen.queryByRole('alert')).toBeNull()
})

it('clears a refusal once a later choice lands', async () => {
  const user = userEvent.setup()
  const onSelect = vi
    .fn<() => Promise<void>>()
    .mockRejectedValueOnce(connectAppError('POST_FIELD_NOT_FOUND', Code.NotFound))
    .mockResolvedValueOnce(undefined)
  render(<PostFieldSelect value="" onSelect={onSelect} />)

  const picker = screen.getByRole('combobox', { name: '분야 없음' })
  await user.click(picker)
  await user.click(screen.getByRole('option', { name: '카페' }))
  expect(await screen.findByRole('alert')).toBeInTheDocument()
  await waitFor(() => expect(picker).toBeEnabled())

  await user.click(picker)
  await user.click(screen.getByRole('option', { name: '맛집' }))
  await waitFor(() => expect(screen.queryByRole('alert')).toBeNull())
  expect(picker).not.toHaveAttribute('aria-invalid')
})

it('is disabled for a reason the caller states, adding none of its own', () => {
  render(<PostFieldSelect value="cafe" disabled onSelect={() => {}} />)

  expect(screen.getByRole('combobox', { name: '분야 카페' })).toBeDisabled()
  expect(screen.queryByRole('alert')).toBeNull()
})

it.each([
  ['ko', '분야', '카페', '선택한 분야를 찾을 수 없어요. 다시 선택해 주세요.'],
  ['en', 'Category', 'Cafés', 'That category is not available. Pick one again.'],
] as const)(
  'says why a %s choice was refused under the field and marks it invalid',
  async (locale, label, option, copy) => {
    initializeI18n(locale)
    const user = userEvent.setup()
    render(
      <PostFieldSelect
        value=""
        onSelect={() => Promise.reject(connectAppError('POST_FIELD_NOT_FOUND', Code.NotFound))}
      />,
    )

    const picker = screen.getByRole('combobox', { name: new RegExp(`^${label}`) })
    await user.click(picker)
    await user.click(screen.getByRole('option', { name: option }))

    expect(await screen.findByRole('alert')).toHaveTextContent(copy)
    expect(picker).toHaveAttribute('aria-invalid', 'true')
    expect(picker).toHaveAccessibleDescription(expect.stringContaining(copy))
    await waitFor(() => expect(picker).toBeEnabled())
  },
)
