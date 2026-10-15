import { useState } from 'react'
import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { EMPTY_GUIDE, type AdvertiserGuide, type AdvertiserGuideItem } from '../model/types'
import { ADVERTISER_GUIDE_MAX_CHARS, ADVERTISER_KEYWORD_MAX } from '../config'
import { AdvertiserGuideFields } from './AdvertiserGuideFields'

afterEach(() => cleanup())

/** The host the fields expect: it holds the guide and hands every change back. */
function Host({
  initial = EMPTY_GUIDE,
  items,
  onChange,
}: {
  initial?: AdvertiserGuide
  items?: readonly AdvertiserGuideItem[]
  onChange?: (next: AdvertiserGuide) => void
}) {
  const [value, setValue] = useState(initial)
  return (
    <AdvertiserGuideFields
      value={value}
      items={items}
      onChange={(next) => {
        setValue(next)
        onChange?.(next)
      }}
    />
  )
}

it('counts the guide text against its bound and keeps the counted items closed', async () => {
  const user = userEvent.setup()
  render(<Host />)

  expect(screen.getByText(`0 / ${ADVERTISER_GUIDE_MAX_CHARS}자`)).toBeInTheDocument()
  await user.type(screen.getByLabelText('광고주 가이드'), '대표 메뉴')
  expect(screen.getByText(`5 / ${ADVERTISER_GUIDE_MAX_CHARS}자`)).toBeInTheDocument()

  const counted = screen.getByRole('button', { name: '셀 수 있는 항목' })
  expect(counted).toHaveAttribute('aria-expanded', 'false')
  expect(screen.queryByLabelText('필수 키워드')).not.toBeInTheDocument()
})

it('adds a keyword on Enter, refuses a repeat and removes it again', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<Host onChange={onChange} />)
  await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))

  const keyword = screen.getByLabelText('필수 키워드')
  await user.type(keyword, '성수 카페{Enter}')
  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ keywords: ['성수 카페'] }))
  expect(keyword).toHaveValue('')

  await user.type(keyword, '성수카페{Enter}')
  expect(screen.getByText('이미 넣은 항목이에요.')).toBeInTheDocument()
  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ keywords: ['성수 카페'] }))

  await user.click(screen.getByRole('button', { name: '성수 카페 지우기' }))
  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ keywords: [] }))
})

it('stops taking keywords at the limit', async () => {
  const user = userEvent.setup()
  const keywords = Array.from({ length: ADVERTISER_KEYWORD_MAX }, (_, index) => `키워드${index}`)
  render(<Host initial={{ ...EMPTY_GUIDE, keywords }} />)
  await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))

  expect(screen.getByRole('textbox', { name: '필수 키워드' })).toBeDisabled()
  expect(screen.getByRole('list', { name: '필수 키워드' }).children).toHaveLength(
    ADVERTISER_KEYWORD_MAX,
  )
})

it('writes a hashtag without its # and shows it with one', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<Host onChange={onChange} />)
  await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))

  await user.type(screen.getByLabelText('필수 해시태그'), '#성수맛집{Enter}')

  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ hashtags: ['성수맛집'] }))
  expect(screen.getByRole('button', { name: '#성수맛집 지우기' })).toBeInTheDocument()
})

it('saves a minimum in range, holds one out of range with its reason, and clears to not asked', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<Host onChange={onChange} />)
  await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))
  const minPhotos = screen.getByLabelText('최소 사진 수')

  await user.type(minPhotos, '15')
  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ minPhotos: 15 }))

  await user.clear(minPhotos)
  expect(onChange).toHaveBeenLastCalledWith(expect.objectContaining({ minPhotos: undefined }))

  onChange.mockClear()
  await user.type(minPhotos, '0')
  expect(minPhotos).toHaveAttribute('aria-invalid', 'true')
  expect(screen.getByText(/부터 .*까지 넣을 수 있어요/)).toBeInTheDocument()
  expect(onChange).not.toHaveBeenCalled()
})

// CLIP-210: a clip offers keywords and hashtags only.
it('offers only the items its host names', async () => {
  const user = userEvent.setup()
  render(<Host items={['keywords', 'hashtags']} />)
  await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))

  expect(screen.getByLabelText('필수 키워드')).toBeInTheDocument()
  expect(screen.getByLabelText('필수 해시태그')).toBeInTheDocument()
  expect(screen.queryByLabelText('최소 글자 수')).not.toBeInTheDocument()
  expect(screen.queryByLabelText('최소 사진 수')).not.toBeInTheDocument()
})
