import { afterEach, describe, expect, it, vi } from 'vitest'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { GUIDE, USER, guideTo, openStep, resetEditorTest } from '@/test/editor'
import { publishedPostRow, type FakeDraftSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  clearCaret()
  vi.restoreAllMocks()
})
const AUTOSAVED = { timeout: 4_000 }
const SLUG = '20260820-seongsu'
const guideText = () => screen.queryByRole('textbox', { name: '광고주 가이드' })

describe('campaign-led writing', () => {
  it('starts sponsored, shows every requirement directly, and applies only tapped suggestions', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: { draftSaves, posts: [{ slug: SLUG, campaign: true }] },
    })
    await guideTo(user, GUIDE.kind)
    expect(screen.getByRole('button', { name: '체험단으로 다녀왔어요' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    await user.click(screen.getByRole('button', { name: '다음' }))
    // THEME-55: the new question takes focus, so a screen reader hears the screen changed.
    expect(
      screen.getByRole('heading', { name: '광고주가 부탁한 내용을 알려 주세요' }),
    ).toHaveFocus()
    expect(screen.queryByText('셀 수 있는 항목')).not.toBeInTheDocument()
    expect(screen.getByRole('heading', { name: '글에 꼭 담을 내용' })).toBeVisible()
    for (const name of ['필수 키워드', '최소 글자 수', '최소 사진 수', '필수 해시태그'])
      expect(screen.getByRole('textbox', { name })).toBeVisible()
    fireEvent.change(guideText()!, {
      target: { value: '사진 8장 이상, 1,500자 이상\n키워드: 성수 카페\n#성수맛집' },
    })
    expect(screen.getByLabelText('최소 사진 수')).toHaveValue('')
    await user.click(screen.getByRole('button', { name: '사진 최소 8장 추가' }))
    expect(screen.getByLabelText('최소 사진 수')).toHaveValue('8')
    await user.click(screen.getByRole('button', { name: '키워드 · 성수 카페 추가' }))
    await user.click(screen.getByRole('button', { name: '#성수맛집 제안 지우기' }))
    await waitFor(
      () =>
        expect(draftSaves.at(-1)?.advertiserGuide).toMatchObject({
          minPhotos: 8,
          keywords: ['성수 카페'],
          hashtags: [],
          minChars: undefined,
        }),
      AUTOSAVED,
    )
    await guideTo(user, GUIDE.photos)
    expect(screen.getByRole('complementary', { name: '광고주 요청 챙기기' })).toHaveTextContent(
      '사진 0 / 8장',
    )
    await guideTo(user, GUIDE.story)
    await user.click(screen.getByRole('button', { name: '건너뛰고 확인' }))
    expect(screen.getByRole('heading', { name: '이대로 써 볼까요?' })).toBeInTheDocument()
    expect(screen.getByText('아직 입력하지 않은 키워드: 성수 카페')).toBeInTheDocument()
  })
  it('hides the guide and its tracker for self-paid posts, preserving the guide when switched back', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        draftSaves,
        posts: [{ slug: SLUG, campaign: true, advertiserGuide: { text: '대표 메뉴' } }],
      },
    })
    await guideTo(user, GUIDE.kind)
    await user.click(screen.getByRole('button', { name: '내 돈으로 다녀왔어요' }))
    await waitFor(() => expect(draftSaves.at(-1)?.campaign).toBe(false), AUTOSAVED)
    expect(draftSaves.at(-1)?.advertiserGuide).toBeUndefined()
    expect(
      within(screen.getByRole('navigation', { name: '글쓰기 단계' })).queryByRole('button', {
        name: '요청',
      }),
    ).toBeNull()
    await user.click(screen.getByRole('button', { name: '다음' }))
    expect(screen.getByRole('heading', { name: '사진을 골라 주세요' })).toBeInTheDocument()
    expect(screen.queryByRole('complementary')).toBeNull()
    await guideTo(user, GUIDE.kind)
    await user.click(screen.getByRole('button', { name: '체험단으로 다녀왔어요' }))
    await guideTo(user, GUIDE.requirements)
    expect(guideText()).toHaveValue('대표 메뉴')
  })
  it('keeps the requirements screen through the first save mint', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', { user: USER })
    await guideTo(user, GUIDE.kind)
    expect(screen.getByRole('button', { name: '체험단으로 다녀왔어요' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    await guideTo(user, GUIDE.requirements)
    fireEvent.change(guideText()!, { target: { value: '사진 5장 이상' } })
    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-untitled'),
      AUTOSAVED,
    )
    expect(await screen.findByRole('textbox', { name: '광고주 가이드' })).toHaveValue(
      '사진 5장 이상',
    )
    expect(screen.getByRole('button', { name: '요청' })).toHaveAttribute('aria-current', 'step')
    expect(screen.getByLabelText('최소 사진 수')).toHaveValue('')
  })
  it('mints from the self-paid choice without scrolling or losing the choice', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })
    await guideTo(user, GUIDE.kind)
    const scroll = vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
    await user.click(screen.getByRole('button', { name: '내 돈으로 다녀왔어요' }))
    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-untitled'),
      AUTOSAVED,
    )
    expect(draftSaves[0]?.campaign).toBe(false)
    expect(screen.getByRole('button', { name: '내 돈으로 다녀왔어요' })).toHaveAttribute(
      'aria-pressed',
      'true',
    )
    expect(scroll).not.toHaveBeenCalled()
  })
  it('flushes the guide before opening the template wizard with this post as its return', async () => {
    const user = userEvent.setup()
    const draftSaves: FakeDraftSave[] = []
    const { router } = renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: { draftSaves, posts: [{ slug: SLUG, campaign: true }] },
    })
    await guideTo(user, GUIDE.requirements)
    fireEvent.change(guideText()!, { target: { value: '마지막으로 넣은 안내' } })
    await guideTo(user, GUIDE.template)
    await user.click(screen.getByRole('button', { name: '새 템플릿 만들기' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/templates/new'), AUTOSAVED)
    expect(router.state.location.search).toMatchObject({ post: SLUG })
    expect(draftSaves.at(-1)?.advertiserGuide?.text).toBe('마지막으로 넣은 안내')
  })
  it('shows refused guide saves and restores the saved value', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [{ slug: SLUG, campaign: true }],
        refuseGuide: 'ADVERTISER_GUIDE_ITEM_INVALID',
      },
    })
    await guideTo(user, GUIDE.requirements)
    await user.type(guideText()!, '대표 메뉴')
    expect(
      await screen.findByText(
        '광고주 가이드 항목을 확인해 주세요. 범위를 벗어났거나 빈 항목이 있어요.',
        undefined,
        AUTOSAVED,
      ),
    ).toBeInTheDocument()
    expect(guideText()).toHaveValue('')
  })
  it('keeps a published post’s choices and guide read-only', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          publishedPostRow({ slug: SLUG, campaign: true, advertiserGuide: { text: '대표 메뉴' } }),
        ],
      },
    })
    await openStep(user, '글 생성')
    await guideTo(user, GUIDE.kind)
    expect(screen.getByRole('button', { name: '내 돈으로 다녀왔어요' })).toBeDisabled()
    await guideTo(user, GUIDE.requirements)
    expect(guideText()).toBeDisabled()
    expect(screen.getByLabelText('최소 사진 수')).toBeDisabled()
  })
})
