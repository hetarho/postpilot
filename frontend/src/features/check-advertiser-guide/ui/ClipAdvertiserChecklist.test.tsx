import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { EMPTY_GUIDE } from '@/entities/advertiser-guide'
import { ClipAdvertiserChecklist } from './AdvertiserChecklist'

afterEach(cleanup)

const guide = {
  ...EMPTY_GUIDE,
  text: '주차 정보 안내\n대표 메뉴 강조',
  keywords: ['크림라떼', '주차장'],
  hashtags: ['성수카페'],
}
const coverage = {
  title: '성수 카페',
  captions: ['크림 라떼가 맛있어요'],
  segments: [],
  regionText: [],
  upload: { body: '성수 데이트', tags: ['성수카페'] },
}

it('marks each counted item present or missing in words, the hashtag against the upload tags', () => {
  render(
    <ClipAdvertiserChecklist
      guide={guide}
      coverage={coverage}
      ticks={[]}
      readOnly={false}
      onTicksChange={() => {}}
    />,
  )
  const counted = screen.getByRole('list', { name: '영상에 꼭 담을 내용' })
  const rows = within(counted).getAllByRole('listitem')
  expect(rows.map((row) => row.textContent)).toEqual([
    '필수 키워드 “크림라떼” · 들어 있어요',
    '필수 키워드 “주차장” · 아직 없어요',
    '필수 해시태그 #성수카페 · 업로드 문구 태그에 있어요',
  ])
  expect(screen.queryByText(/글자 수|사진/)).not.toBeInTheDocument()
})

it('saves a tick by the line text and leaves an edited line unticked', async () => {
  const onTicksChange = vi.fn()
  render(
    <ClipAdvertiserChecklist
      guide={guide}
      coverage={coverage}
      // A tick of a line the guide no longer holds, as after the line was edited.
      ticks={['주차 정보를 안내']}
      readOnly={false}
      onTicksChange={onTicksChange}
    />,
  )
  const parking = screen.getByRole('checkbox', { name: '주차 정보 안내' })
  expect(parking).not.toBeChecked()
  expect(screen.getByRole('checkbox', { name: '대표 메뉴 강조' })).not.toBeChecked()
  await userEvent.click(parking)
  expect(onTicksChange).toHaveBeenLastCalledWith(['주차 정보 안내'])
})
