import { afterEach, describe, expect, it } from 'vitest'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import { CLIP_COMPOSITION_EXAMPLE } from '@/entities/clip-template'
import { CLIP_PROJECT_LIMITS, type ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'

const template = {
  id: 'menu',
  name: '여러 메뉴',
  compositionBody: CLIP_COMPOSITION_EXAMPLE,
  compositionLegacy: false,
  informationFields: [],
  cutGuidance: '',
  accent: '' as const,
  preset: '' as const,
}
afterEach(() => discardClipDraftQueues())

describe("the project's own instruction", () => {
  it.each([
    ['untouched', '', ''],
    ['written', '고기 굽는 소리를 살려 주세요.', '고기 굽는 소리를 살려 주세요.'],
  ])('is optional and saves an %s instruction', async (_name, typed, saved) => {
    const user = userEvent.setup(),
      writes: ClipProjectDraft[] = []
    renderAppAt('/clips/new', {
      user: { id: 'alice' },
      clips: { templates: [template], projectWrites: writes },
    })
    await user.type(await screen.findByLabelText('클립 제목'), '고기')
    await user.click(screen.getByRole('combobox', { name: /^영상 템플릿/ }))
    await user.click(await screen.findByRole('option', { name: '여러 메뉴' }))
    const instruction = screen.getByLabelText(/클립에 담고 싶은 내용/)
    expect(instruction).toHaveValue('')
    fireEvent.change(screen.getAllByLabelText('메뉴 이름')[0], { target: { value: '삼겹살' } })
    if (typed) fireEvent.change(instruction, { target: { value: typed } })
    await user.click(screen.getByRole('button', { name: '클립 만들기' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].instruction).toBe(saved)
  })

  it('accepts no character past its maximum and counts down beside the control', async () => {
    const user = userEvent.setup()
    renderAppAt('/clips/new', { user: { id: 'alice' }, clips: { templates: [template] } })
    await user.type(await screen.findByLabelText('클립 제목'), '고기')
    const max = CLIP_PROJECT_LIMITS.instruction
    const instruction = screen.getByLabelText(/클립에 담고 싶은 내용/)
    fireEvent.change(instruction, { target: { value: '가'.repeat(max) } })
    expect(instruction).toHaveValue('가'.repeat(max))
    expect(screen.getByText('0자 남음')).toBeInTheDocument()
    // A paste past the bound is cut to it in front of the counter, never
    // silently dropped later (CLIP-117).
    fireEvent.change(instruction, { target: { value: '가'.repeat(max + 40) } })
    expect(instruction).toHaveValue('가'.repeat(max))
    fireEvent.change(instruction, { target: { value: '가'.repeat(max - 3) } })
    expect(screen.getByText('3자 남음')).toBeInTheDocument()
  })
})
