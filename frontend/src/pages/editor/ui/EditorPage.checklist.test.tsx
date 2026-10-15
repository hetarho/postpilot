// POST-119: ③'s pre-submission checklist for a 체험단 post.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { USER, resetEditorTest } from '@/test/editor'
import { finalizedPostRow, publishedPostRow, type FakeDraftSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('the pre-submission checklist', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const SLUG = '20260820-jeju'
  // POST_CONTENT_FIXTURE: 제주 in the title, two photos, tags 제주 · 산책 · 여행.
  const GUIDE = {
    text: '대표 메뉴를 소개해 주세요\n주차 정보 안내',
    keywords: ['제주', '주차'],
    minChars: 1000,
    minPhotos: 2,
    hashtags: ['제주', '바다'],
  }
  const following = (first: Node, second: Node) =>
    Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING)

  it('marks counted items and saves owner ticks', async () => {
    const calls: string[] = []
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      calls,
      posts: {
        calls,
        draftSaves,
        posts: [
          finalizedPostRow({
            slug: SLUG,
            campaign: true,
            advertiserGuide: GUIDE,
            checklistTicks: ['주차 정보 안내'],
          }),
        ],
      },
    })

    // A finalized post opens on 글 완성; the checklist stands above the export.
    const panel = await screen.findByRole('tabpanel', { name: '글 완성' })
    const checklist = within(panel).getByRole('region', { name: '제출 전 체크리스트' })
    expect(following(checklist, within(panel).getByRole('heading', { name: '내보내기' }))).toBe(
      true,
    )
    const counted = within(checklist).getByRole('list', { name: '셀 수 있는 항목' })
    const rows = within(counted)
      .getAllByRole('listitem')
      .map((row) => row.textContent)
    expect(rows).toEqual([
      '필수 키워드 “제주” · 들어 있어요',
      '필수 키워드 “주차” · 아직 없어요',
      expect.stringMatching(
        /^글자 수 \d+자 \(공백 제외 \d+자\), 최소 1000자 · 공백 제외 \d+자 더 필요해요$/,
      ),
      '사진 2장, 최소 2장 · 충분해요',
      '필수 해시태그 #제주 · 태그에 있어요',
      '필수 해시태그 #바다 · 태그에 없어요',
    ])

    const menu = within(checklist).getByRole('checkbox', { name: '대표 메뉴를 소개해 주세요' })
    const parking = within(checklist).getByRole('checkbox', { name: '주차 정보 안내' })
    expect(menu).not.toBeChecked()
    expect(parking).toBeChecked()
    const before = calls.length
    await user.click(menu)

    expect(menu).toBeChecked()
    await waitFor(
      () =>
        expect(draftSaves.at(-1)?.checklistTicks).toEqual([
          '대표 메뉴를 소개해 주세요',
          '주차 정보 안내',
        ]),
      AUTOSAVED,
    )
    // The tick save is the only request the checklist makes, and it carries nothing else.
    expect(draftSaves.at(-1)).toMatchObject({ campaign: undefined, advertiserGuide: undefined })
    expect(
      calls.slice(before).filter((call) => call !== 'SavePostDraft' && call !== 'GetPost'),
    ).toEqual([])
  })

  it('hides the checklist for a 내돈내산 post', async () => {
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: { posts: [finalizedPostRow({ slug: SLUG, campaign: false, advertiserGuide: GUIDE })] },
    })

    const panel = await screen.findByRole('tabpanel', { name: '글 완성' })
    await within(panel).findByRole('heading', { name: '내보내기' })
    expect(within(panel).queryByRole('region', { name: '제출 전 체크리스트' })).toBeNull()
  })

  it('hides the checklist while the guide asks for nothing', async () => {
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: { posts: [finalizedPostRow({ slug: SLUG, campaign: true })] },
    })

    const panel = await screen.findByRole('tabpanel', { name: '글 완성' })
    await within(panel).findByRole('heading', { name: '내보내기' })
    expect(within(panel).queryByRole('region', { name: '제출 전 체크리스트' })).toBeNull()
  })

  it('shows a published post’s checklist read-only', async () => {
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          publishedPostRow({
            slug: SLUG,
            campaign: true,
            advertiserGuide: GUIDE,
            checklistTicks: ['주차 정보 안내'],
          }),
        ],
      },
    })

    const checklist = await screen.findByRole('region', { name: '제출 전 체크리스트' })
    for (const box of within(checklist).getAllByRole('checkbox')) expect(box).toBeDisabled()
    expect(within(checklist).getByRole('checkbox', { name: '주차 정보 안내' })).toBeChecked()
  })
})
