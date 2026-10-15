// POST-117, POST-118: ①'s 체험단 글 switch and the advertiser guide under it.
import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { USER, openStep, resetEditorTest } from '@/test/editor'
import { publishedPostRow, type FakeDraftSave } from '@/test/posts'
import { clearCaret } from '@/features/edit-post-content/model/caret-handoff'

afterEach(() => {
  resetEditorTest()
  // Module state, so an unconsumed handoff would leak into the next test.
  clearCaret()
})

describe('the 체험단 글 switch', () => {
  const AUTOSAVED = { timeout: 4_000 }
  const SLUG = '20260820-seongsu'
  const TEMPLATES = [
    { id: 'visit', name: '방문 기록', body: '<write>인트로</write>\n<ask label="방문일"/>' },
  ]
  const following = (first: Node, second: Node) =>
    Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING)
  const campaignSwitch = () => screen.findByRole('switch', { name: '체험단 글' })
  const guideText = () => screen.queryByRole('textbox', { name: '광고주 가이드' })

  it('lays out the switch and guide between the 가제 and the fields', async () => {
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [{ slug: SLUG, title: '성수 카페', campaign: true, template: TEMPLATES[0] }],
        templates: [{ id: 'visit', name: '방문 기록' }],
      },
      templates: { templates: TEMPLATES },
    })

    const answer = await screen.findByLabelText('방문일')
    const title = screen.getByLabelText('제목')
    const toggle = await campaignSwitch()
    const guide = guideText()!
    const memo = screen.getByLabelText('메모')
    expect(following(title, toggle)).toBe(true)
    expect(following(toggle, guide)).toBe(true)
    expect(following(guide, answer)).toBe(true)
    expect(following(answer, memo)).toBe(true)
    expect(screen.getByText('0 / 3000자')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '셀 수 있는 항목' })).toHaveAttribute(
      'aria-expanded',
      'false',
    )
  })

  it('autosaves the switch and the guide', async () => {
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: { draftSaves, posts: [{ slug: SLUG, title: '성수 카페', campaign: true }] },
    })

    await campaignSwitch()
    await user.type(guideText()!, '대표 메뉴를 꼭 넣어 주세요')
    await user.click(screen.getByRole('button', { name: '셀 수 있는 항목' }))
    await user.type(screen.getByRole('textbox', { name: '필수 키워드' }), '성수 카페{Enter}')

    await waitFor(
      () =>
        expect(draftSaves.at(-1)?.advertiserGuide).toEqual({
          text: '대표 메뉴를 꼭 넣어 주세요',
          keywords: ['성수 카페'],
          minChars: undefined,
          minPhotos: undefined,
          hashtags: [],
        }),
      AUTOSAVED,
    )
    // An ordinary title save carries no campaign part it did not change.
    expect(draftSaves.every((save) => save.campaign === undefined)).toBe(true)
    const saves = draftSaves.length
    await user.type(screen.getByLabelText('제목'), ' 투어')
    await waitFor(() => expect(draftSaves.length).toBeGreaterThan(saves), AUTOSAVED)
    expect(draftSaves.at(-1)).toMatchObject({
      campaign: undefined,
      advertiserGuide: undefined,
      checklistTicks: undefined,
    })
  })

  it('hides but keeps the guide while the switch is off', async () => {
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        draftSaves,
        posts: [
          {
            slug: SLUG,
            title: '성수 카페',
            campaign: true,
            advertiserGuide: { text: '대표 메뉴를 꼭 넣어 주세요' },
          },
        ],
      },
    })

    const toggle = await campaignSwitch()
    expect(toggle).toBeChecked()
    expect(guideText()).toHaveValue('대표 메뉴를 꼭 넣어 주세요')
    await user.click(toggle)
    expect(guideText()).toBeNull()

    await waitFor(() => expect(draftSaves.at(-1)?.campaign).toBe(false), AUTOSAVED)
    // Only the switch moved: the guide stays as the server holds it.
    expect(draftSaves.at(-1)?.advertiserGuide).toBeUndefined()

    await user.click(toggle)
    expect(guideText()).toHaveValue('대표 메뉴를 꼭 넣어 주세요')
  })

  it('starts a new post with the switch on', async () => {
    const draftSaves: FakeDraftSave[] = []
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', { user: USER, posts: { draftSaves } })

    const toggle = await campaignSwitch()
    expect(toggle).toBeChecked()
    // The unminted order is the same: 가제 → the switch with its guide → memo.
    expect(following(screen.getByLabelText('제목'), toggle)).toBe(true)
    expect(following(toggle, guideText()!)).toBe(true)
    expect(following(guideText()!, screen.getByLabelText('메모'))).toBe(true)
    await user.type(screen.getByLabelText('제목'), '성수')

    await waitFor(
      () => expect(router.state.location.pathname).toBe('/posts/20260828-성수'),
      AUTOSAVED,
    )
    // A new post is a 체험단 post on the server too, so the create need not say so.
    expect(draftSaves[0]?.campaign).toBeUndefined()
    expect(await campaignSwitch()).toBeChecked()
  })

  it('shows a post saved as not a 체험단 post with the switch off', async () => {
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: { posts: [{ slug: SLUG, title: '성수 카페', campaign: false }] },
    })

    expect(await campaignSwitch()).not.toBeChecked()
    expect(guideText()).toBeNull()
  })

  it('says a refused guide under the guide and takes it back', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [{ slug: SLUG, title: '성수 카페', campaign: true }],
        refuseGuide: 'ADVERTISER_GUIDE_ITEM_INVALID',
      },
    })

    await campaignSwitch()
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

  it('shows a published post’s switch and guide read-only', async () => {
    const user = userEvent.setup()
    renderAppAt(`/posts/${SLUG}`, {
      user: USER,
      posts: {
        posts: [
          publishedPostRow({
            slug: SLUG,
            title: '성수 카페',
            campaign: true,
            advertiserGuide: { text: '대표 메뉴를 꼭 넣어 주세요' },
          }),
        ],
      },
    })

    // A published post opens on 글 완성; ① still shows its material.
    await openStep(user, '글 생성')
    expect(await campaignSwitch()).toBeDisabled()
    expect(guideText()).toBeDisabled()
  })
})
