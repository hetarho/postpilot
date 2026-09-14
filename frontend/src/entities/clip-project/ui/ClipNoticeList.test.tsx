import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'
import i18next from 'i18next'
import { ClipNoticeList } from './ClipNoticeList'
import { clipNoticeKey, clipNoticeKeys, type ClipNotice } from '../model/notices'

const notices: ClipNotice[] = [
  { code: 'plan_cut_rate', cutId: 'cut-a', elementId: '', action: 'repair' },
  { code: 'shorter_copy', cutId: 'cut-a', elementId: 'caption', action: 'repair' },
  { code: 'plan_target_duration', cutId: '', elementId: '', action: 'shortfall' },
]
afterEach(cleanup)

it.each(['ko', 'en'] as const)(
  'explains plan, cut and text notices in the project language (%s)',
  (language) => {
    render(<ClipNoticeList notices={notices} language={language} withTargets />)
    for (const notice of notices)
      expect(
        screen.getByText(i18next.t(clipNoticeKey(notice), { ns: 'clips', lng: language }), {
          exact: false,
        }),
      ).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  },
)

it('renders no placeholder for an ordinary delivered clip and clears on a refreshed response', () => {
  const view = render(<ClipNoticeList notices={notices} />)
  expect(screen.getAllByRole('listitem')).toHaveLength(3)
  view.rerender(<ClipNoticeList notices={[]} />)
  expect(view.container).toBeEmptyDOMElement()
})

it('uses the existing unknown-check wording without exposing a new raw code', () => {
  render(
    <ClipNoticeList
      notices={[{ code: 'private-future-check', action: 'repair', cutId: '', elementId: '' }]}
      language="ko"
    />,
  )
  expect(
    screen.getByText(i18next.t('inspection.detailUnknown', { ns: 'clips', lng: 'ko' })),
  ).toBeInTheDocument()
  expect(screen.queryByText(/private-future-check/)).not.toBeInTheDocument()
})

it('has calm Korean and English wording for every returned notice code', () => {
  for (const code of Object.keys(clipNoticeKeys)) {
    for (const action of ['repair', 'removal', 'shortfall']) {
      const key = clipNoticeKey({ code, action, cutId: '', elementId: '' })
      for (const lng of ['ko', 'en']) {
        expect(i18next.exists(key, { ns: 'clips', lng })).toBe(true)
        const text = i18next.t(key, { ns: 'clips', lng })
        expect(text).not.toMatch(/오류|실패|validator|validation|failed|error/i)
        expect(text).not.toContain(code)
      }
    }
  }
})
