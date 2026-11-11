// A new post on an account with no made voice or no template opens on the offer to make it
// (POST-121).
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetOfferDismissalMemory } from '@/features/offer-creation-setup'
import { renderAppAt } from '@/test/app'
import { USER, resetEditorTest } from '@/test/editor'

const TEMPLATE = { id: 'template-visit', name: '방문 후기' }
const WITH_TEMPLATE = { templates: { templates: [TEMPLATE] }, posts: { templates: [TEMPLATE] } }
const NO_VOICE = { voice: { voices: [] } }
const offerDialog = (name: string) => screen.findByRole('dialog', { name })
/** The editor standing is the page having decided; only then is "no dialog" an answer. */
async function expectNoOffer() {
  expect(await screen.findByLabelText('제목')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
}

beforeEach(() => {
  localStorage.clear()
  resetOfferDismissalMemory()
})
afterEach(() => {
  resetEditorTest()
})

describe('the new-post offer', () => {
  it('names both kinds when the account has no made voice and no template', async () => {
    renderAppAt('/posts/new', { user: USER, creationOffers: true, ...NO_VOICE })

    const dialog = await offerDialog('말투와 템플릿이 아직 없어요')
    for (const name of ['새 말투 만들기', '새 템플릿 만들기', '이대로 쓰기', '다시 묻지 않기'])
      expect(within(dialog).getByRole('button', { name })).toBeInTheDocument()
  })

  it('offers only the template, opening the wizard that returns to a new draft', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', { user: USER, creationOffers: true })

    const dialog = await offerDialog('템플릿이 아직 없어요')
    expect(within(dialog).queryByRole('button', { name: '새 말투 만들기' })).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '새 템플릿 만들기' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/templates/new'))
    expect(router.state.location.search).toMatchObject({ post: 'new' })
  })

  it('offers only the voice when a template exists, and opens the create sheet over the draft', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, creationOffers: true, ...NO_VOICE, ...WITH_TEMPLATE })

    const dialog = await offerDialog('말투가 아직 없어요')
    expect(
      within(dialog).queryByRole('button', { name: '새 템플릿 만들기' }),
    ).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: '새 말투 만들기' }))

    expect(await offerDialog('새 말투')).toBeInTheDocument()
    expect(screen.queryByRole('dialog', { name: '말투가 아직 없어요' })).not.toBeInTheDocument()
  })

  it('continues a voice still being made instead of starting a second one', async () => {
    const user = userEvent.setup()
    const { router } = renderAppAt('/posts/new', {
      user: USER,
      creationOffers: true,
      voice: { voices: [{ id: 'voice-half', name: '반쯤 만든 말투', made: false }] },
      ...WITH_TEMPLATE,
    })

    const dialog = await offerDialog('말투가 아직 없어요')
    await user.click(within(dialog).getByRole('button', { name: '말투 이어서 만들기' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/voices/voice-half'))
  })

  it('closes for this creation on 이대로 쓰기 and on Escape', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, creationOffers: true })

    await user.click(
      within(await offerDialog('템플릿이 아직 없어요')).getByRole('button', {
        name: '이대로 쓰기',
      }),
    )
    await expectNoOffer()
    await user.type(screen.getByLabelText('제목'), '그냥 쓰는 글')
    expect(screen.getByLabelText('제목')).toHaveValue('그냥 쓰는 글')

    cleanup()
    renderAppAt('/posts/new', { user: USER, creationOffers: true })
    await offerDialog('템플릿이 아직 없어요')
    await user.keyboard('{Escape}')
    await expectNoOffer()
  })

  it('stops asking this account on this browser after 다시 묻지 않기, but not another account', async () => {
    const user = userEvent.setup()
    renderAppAt('/posts/new', { user: USER, creationOffers: true })
    await user.click(
      within(await offerDialog('템플릿이 아직 없어요')).getByRole('button', {
        name: '다시 묻지 않기',
      }),
    )
    await expectNoOffer()

    cleanup()
    renderAppAt('/posts/new', { user: USER, creationOffers: true })
    await expectNoOffer()

    cleanup()
    renderAppAt('/posts/new', { user: { id: 'bob' }, creationOffers: true })
    expect(await offerDialog('템플릿이 아직 없어요')).toBeInTheDocument()
  })

  it('does not ask again when the offer’s own wizard hands a template back', async () => {
    renderAppAt(`/posts/new?template=${TEMPLATE.id}`, {
      user: USER,
      creationOffers: true,
      ...NO_VOICE,
      ...WITH_TEMPLATE,
    })
    await expectNoOffer()
  })

  // TMPL-32: the offer's wizard saves back to a new draft open on 템플릿, its template chosen —
  // not on 글 종류, where nothing would show what was just made.
  it('opens the handed-back new draft on 템플릿 with the template chosen', async () => {
    renderAppAt(`/posts/new?template=${TEMPLATE.id}`, {
      user: USER,
      creationOffers: true,
      ...NO_VOICE,
      ...WITH_TEMPLATE,
    })
    expect(await screen.findByRole('heading', { name: '어떤 틀로 써 볼까요?' })).toBeVisible()
    expect(screen.queryByRole('heading', { name: '어떤 글을 쓰시나요?' })).toBeNull()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /방문 후기/ })).toHaveAttribute(
        'aria-pressed',
        'true',
      ),
    )
  })

  it('does not ask when first-use setup opened the creation', async () => {
    renderAppAt('/posts/new?from=setup', { user: USER, creationOffers: true, ...NO_VOICE })
    await expectNoOffer()
  })

  it('opens nothing when the template directory cannot be read', async () => {
    const calls: string[] = []
    renderAppAt('/posts/new', {
      user: USER,
      creationOffers: true,
      calls,
      ...NO_VOICE,
      templates: { listFails: true, calls },
    })
    await expectNoOffer()
    // The editor stays up: its pickers' own refetch of the failed directory does not take the
    // page back to its placeholder and round again.
    expect(await screen.findByText(/템플릿 목록을 불러오지 못했어요/)).toBeInTheDocument()
    expect(calls.filter((call) => call === 'ListTemplates').length).toBeLessThanOrEqual(3)
  })

  it('opens nothing for an account with a made voice and a template', async () => {
    renderAppAt('/posts/new', { user: USER, creationOffers: true, ...WITH_TEMPLATE })
    await expectNoOffer()
  })

  it('never opens on an existing post', async () => {
    renderAppAt('/posts/existing', {
      user: USER,
      creationOffers: true,
      ...NO_VOICE,
      posts: { posts: [{ slug: 'existing', title: '이미 있는 글' }] },
    })
    expect(await screen.findByLabelText('제목')).toHaveValue('이미 있는 글')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
