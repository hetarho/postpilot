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
// The saved project uses the same fixture pair ClipPage's autosave test does:
// the instruction is written in ① now (CLIP-130), so it rides that same queue.
const legacyTemplate = {
  id: 'template',
  name: '여행',
  informationFields: [{ label: '장소', prompt: '어디인가요?' }],
  cutGuidance: '',
  accent: '' as const,
  preset: 'restaurant' as const,
}
const project = {
  id: 'project',
  title: '제주 여행',
  videoTemplateId: legacyTemplate.id,
  ratio: 'vertical' as const,
  targetDurationMs: 30000,
  disclosure: 'ad' as const,
  cta: '' as const,
  answers: [{ label: '장소', text: '제주도' }],
}
afterEach(() => discardClipDraftQueues())

describe("the project's own instruction", () => {
  it('is not asked for before there is footage to describe', async () => {
    const user = userEvent.setup()
    renderAppAt('/clips/new', { user: { id: 'alice' }, clips: { templates: [template] } })
    await user.type(await screen.findByLabelText('클립 제목'), '고기')
    await user.click(screen.getByRole('combobox', { name: /^영상 템플릿/ }))
    await user.click(await screen.findByRole('option', { name: '여러 메뉴' }))
    expect(screen.queryByLabelText(/클립에 담고 싶은 내용/)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '클립 만들기' })).toBeEnabled()
  })

  it('is optional and saves a written instruction', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [legacyTemplate], projects: [project], projectWrites: writes },
    })
    const instruction = await screen.findByLabelText(/클립에 담고 싶은 내용/)
    expect(instruction).toHaveValue('')
    // Nothing is pressed: ①'s settings save themselves a beat after the typing
    // stops (CLIP-39), and the instruction rides that same queue.
    await user.type(instruction, '소리를 살려 주세요')
    await waitFor(() => expect(writes.at(-1)?.instruction).toBe('소리를 살려 주세요'), {
      timeout: 4000,
    })
  })

  it('accepts no character past its maximum and counts down beside the control', async () => {
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [legacyTemplate], projects: [project] },
    })
    const max = CLIP_PROJECT_LIMITS.instruction
    const instruction = await screen.findByLabelText(/클립에 담고 싶은 내용/)
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
