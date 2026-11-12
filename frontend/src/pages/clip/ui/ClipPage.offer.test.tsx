import { clipGuideTo } from '@/test/clip-preparation'
// A new clip on an account with no video template opens on the offer to make one, whose wizard
// returns to a new clip with it selected (CLIP-223).
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { cleanup, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'
import { resetOfferDismissalMemory } from '@/features/offer-creation-setup'
import { renderAppAt } from '@/test/app'

const ALICE = { id: 'alice' }
const TEMPLATE = {
  id: 'template-cafe',
  name: '카페 구성',
  compositionBody: '<clip version="1"/>',
  introPreset: 'serif' as const,
  outroPreset: 'chips' as const,
}
const OFFER = '영상 템플릿이 아직 없어요'
const offerDialog = () => screen.findByRole('dialog', { name: OFFER })
/** The form standing is the page having decided; only then is "no dialog" an answer. */
async function expectNoOffer() {
  await clipGuideTo('content')
  expect(await screen.findByLabelText('클립 제목')).toBeInTheDocument()
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
}

/** The guide screen marked current in its progress. */
async function currentScreen(name: string) {
  const progress = await screen.findByRole('navigation', { name: '클립 만들기 단계' })
  const step = within(progress).getByRole('button', { name })
  await waitFor(() => expect(step).toHaveAttribute('aria-current', 'step'))
  return step
}

beforeEach(() => {
  localStorage.clear()
  resetOfferDismissalMemory()
})
afterEach(() => {
  cleanup()
  discardClipDraftQueues()
})

describe('the new-clip offer', () => {
  it('offers only a video template to an account with none, never a voice', async () => {
    renderAppAt('/clips/new', { user: ALICE, creationOffers: true, voice: { voices: [] } })

    const dialog = await offerDialog()
    expect(
      within(dialog).getByRole('button', { name: '새 영상 템플릿 만들기' }),
    ).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '이대로 만들기' })).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: '다시 묻지 않기' })).toBeInTheDocument()
    expect(within(dialog).queryByRole('button', { name: /말투/ })).not.toBeInTheDocument()
  })

  it('opens the wizard in the clip’s channel and comes back to a new clip with the template', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    const { router } = renderAppAt('/clips/new?channel=instagram_clip', {
      user: ALICE,
      creationOffers: true,
      clips: { templates: [], projectWrites: writes },
    })
    await user.click(
      within(await offerDialog()).getByRole('button', { name: '새 영상 템플릿 만들기' }),
    )
    await waitFor(() => expect(router.state.location.pathname).toBe('/video-templates/new'))
    expect(router.state.location.search).toMatchObject({ channel: 'instagram_clip', clip: 'new' })

    await user.click(await screen.findByRole('button', { name: /^카페/ }))
    await user.click(screen.getByRole('button', { name: '다음' }))
    await user.click(await screen.findByRole('button', { name: /^디저트/ }))
    await user.click(screen.getByRole('button', { name: '다음' }))
    await screen.findByRole('heading', { name: '이런 영상은 보통 이렇게 이어져요' })
    for (const box of screen.getAllByRole('checkbox')) {
      if (!box.closest('label')?.textContent?.startsWith('훅')) await user.click(box)
    }
    for (let step = 0; step < 4; step++)
      await user.click(screen.getByRole('button', { name: '다음' }))
    await screen.findByRole('heading', { name: '영상의 디자인을 골라요' })
    await user.click(screen.getByRole('button', { name: '다음' }))
    await user.type(await screen.findByLabelText('영상 템플릿 이름'), '내 카페 클립')
    await user.dblClick(screen.getByRole('button', { name: '저장' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/clips/new'))
    expect(router.state.location.search).toMatchObject({ channel: 'instagram_clip' })
    const templateId = (router.state.location.search as { template?: string }).template
    expect(templateId).toBeTruthy()
    // The unminted guide opens on 템플릿 with the new template selected (CLIP-223, CLIP-225).
    expect(await currentScreen('템플릿')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^내 카페 클립/ })).toHaveAttribute(
        'aria-pressed',
        'true',
      ),
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    await clipGuideTo('content')
    await user.type(screen.getByLabelText('클립 제목'), '디저트 투어')
    await waitFor(() => expect(writes).toHaveLength(1), { timeout: 4000 })
    expect(writes[0]).toMatchObject({ videoTemplateId: templateId, channel: 'instagram_clip' })
  })

  it('starts from a handed template with its design, and leaves 없음 for an unknown one', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt(`/clips/new?template=${TEMPLATE.id}`, {
      user: ALICE,
      creationOffers: true,
      clips: { templates: [TEMPLATE], projectWrites: writes },
    })
    expect(await currentScreen('템플릿')).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^카페 구성/ })).toHaveAttribute(
        'aria-pressed',
        'true',
      ),
    )
    await clipGuideTo('content')
    await user.type(screen.getByLabelText('클립 제목'), '라떼')
    await waitFor(() => expect(writes).toHaveLength(1), { timeout: 4000 })
    expect(writes[0]).toMatchObject({
      videoTemplateId: TEMPLATE.id,
      introPreset: 'serif',
      outroPreset: 'chips',
    })

    cleanup()
    renderAppAt('/clips/new?template=gone', {
      user: ALICE,
      creationOffers: true,
      clips: { templates: [TEMPLATE] },
    })
    expect(await currentScreen('템플릿')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^없음/ })).toHaveAttribute('aria-pressed', 'true')
  })

  it('closes for this creation on 이대로 만들기 and on Escape', async () => {
    const user = userEvent.setup()
    renderAppAt('/clips/new', { user: ALICE, creationOffers: true })
    await user.click(within(await offerDialog()).getByRole('button', { name: '이대로 만들기' }))
    // 이대로 만들기 continues on 영상 종류 (CLIP-223, CLIP-225).
    expect(await currentScreen('영상 종류')).toBeInTheDocument()
    await expectNoOffer()

    cleanup()
    renderAppAt('/clips/new', { user: ALICE, creationOffers: true })
    await offerDialog()
    await user.keyboard('{Escape}')
    await expectNoOffer()
  })

  it('stops the clip offer after 다시 묻지 않기 while the post offer still asks', async () => {
    const user = userEvent.setup()
    renderAppAt('/clips/new', { user: ALICE, creationOffers: true })
    await user.click(within(await offerDialog()).getByRole('button', { name: '다시 묻지 않기' }))
    await expectNoOffer()

    cleanup()
    renderAppAt('/clips/new?channel=instagram_clip', { user: ALICE, creationOffers: true })
    await expectNoOffer()

    cleanup()
    renderAppAt('/posts/new', { user: ALICE, creationOffers: true })
    expect(await screen.findByRole('dialog', { name: '템플릿이 아직 없어요' })).toBeInTheDocument()
  })

  it('does not ask when first-use setup opened the creation', async () => {
    renderAppAt('/clips/new?from=setup', { user: ALICE, creationOffers: true })
    await expectNoOffer()
  })

  it('opens nothing when the directory cannot be read or holds a template', async () => {
    renderAppAt('/clips/new', {
      user: ALICE,
      creationOffers: true,
      clips: { listFails: true },
    })
    await expectNoOffer()

    cleanup()
    renderAppAt('/clips/new', {
      user: ALICE,
      creationOffers: true,
      clips: { templates: [TEMPLATE] },
    })
    await expectNoOffer()
  })

  it('never opens on an existing clip', async () => {
    renderAppAt('/clips/project', {
      user: ALICE,
      creationOffers: true,
      clips: {
        projects: [
          {
            id: 'project',
            title: '제주 여행',
            videoTemplateId: '',
            ratio: 'vertical',
            targetDurationMs: 30000,
            disclosure: 'ad',
          },
        ],
      },
    })
    await clipGuideTo('content')
    expect(await screen.findByLabelText('클립 제목')).toHaveValue('제주 여행')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
