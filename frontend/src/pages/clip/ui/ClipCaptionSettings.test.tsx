import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const template = {
  id: 'template',
  name: '여행',
  compositionBody: '<clip version="1"/>',
}
const project = {
  id: 'project',
  title: '제주 여행',
  videoTemplateId: template.id,
  ratio: 'vertical' as const,
  targetDurationMs: 30000,
  disclosure: 'ad' as const,
  captionPace: 'steady' as const,
  accent: 'teal' as const,
}
afterEach(() => discardClipDraftQueues())

describe("the clip's own caption pace and accent", () => {
  it('shows what this clip uses and saves a change through the draft queue', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [template], projects: [project], projectWrites: writes },
    })
    await user.click(await screen.findByRole('button', { name: '디자인과 자막 스타일' }))
    const pace = await screen.findByRole('tablist', { name: '자막 흐름' })
    expect(within(pace).getByRole('tab', { name: '문장형' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    const accents = screen.getByRole('radiogroup', { name: '강조 색상' })
    expect(within(accents).getByRole('radio', { name: /청록/ })).toHaveAttribute(
      'aria-checked',
      'true',
    )
    await user.click(within(pace).getByRole('tab', { name: '빠른 구절형' }))
    await waitFor(() => expect(writes.at(-1)?.captionPace).toBe('rapid'), { timeout: 4000 })
    await user.click(within(accents).getByRole('radio', { name: /분홍/ }))
    await waitFor(() => expect(writes.at(-1)?.accent).toBe('pink'), { timeout: 4000 })
  })

  it('is not asked for before the clip exists', async () => {
    const user = userEvent.setup()
    renderAppAt('/clips/new', { user: { id: 'alice' }, clips: { templates: [template] } })
    await user.type(await screen.findByLabelText('클립 제목'), '고기')
    expect(screen.queryByRole('radiogroup', { name: '강조 색상' })).not.toBeInTheDocument()
  })
})
