import { afterEach, describe, expect, it } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderAppAt } from '@/test/app'
import type { ClipProjectDraft } from '@/entities/clip-project'
import { discardClipDraftQueues } from '@/features/edit-clip-project'
import { chooseOption } from '@/test/listbox'

const template = {
  id: 'template',
  name: '여행',
  informationFields: [{ label: '장소', prompt: '어디인가요?' }],
  cutGuidance: '',
  accent: 'teal' as const,
  preset: 'restaurant' as const,
  captionPace: 'rapid' as const,
  compositionBody:
    '<clip version="1" intro="a" caption="bold" outro="b" accent="teal" pace="rapid">' +
    '<field id="place" label="장소">어디인가요?</field>' +
    '<text id="hook" kind="fixed" role="hook" basis="output-start"/>' +
    '<text id="ending" kind="fixed" role="ending" basis="output-end"/></clip>',
}
const project = {
  id: 'project',
  title: '제주 여행',
  videoTemplateId: '',
  ratio: 'vertical' as const,
  targetDurationMs: 30000,
  disclosure: 'ad' as const,
  cta: '' as const,
  answers: [],
  introPreset: 'b' as const,
  outroPreset: 'e' as const,
  allowedCaptionStyles: [] as string[],
}
afterEach(() => discardClipDraftQueues())

describe('① chooses the design, the caption styles and an optional template', () => {
  it('mints without a template, with 없음 selected', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/new', {
      user: { id: 'alice' },
      clips: { templates: [template], projectWrites: writes },
    })
    const picker = await screen.findByRole('combobox', { name: /^영상 템플릿/ })
    expect(picker).toHaveAccessibleName('영상 템플릿 없음')
    await user.type(await screen.findByLabelText('클립 제목'), '고기')
    await user.click(screen.getByRole('button', { name: '클립 만들기' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].videoTemplateId).toBe('')
    // With no template the server seeds the shared defaults, which is what ①
    // then offers as the project's own selection.
    expect(writes[0].introPreset).toBe('b')
    expect(writes[0].outroPreset).toBe('e')
  })

  it('offers each style with its own drawing, says which are drawn frame by frame, and saves the selection', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: {
        templates: [template],
        projects: [project],
        projectWrites: writes,
        sequenceStyles: ['word-pop'],
      },
    })
    const styles = await screen.findByRole('group', { name: '자막 스타일' })
    // An empty selection is not "unset": it reads as the default style alone.
    expect(screen.getByText('아무것도 고르지 않으면 크게 강조 하나만 써요.')).toBeInTheDocument()
    expect(within(styles).getByRole('checkbox', { name: /크게 강조/ })).not.toBeChecked()
    // Each style is offered as the RENDERER draws it, and the ones drawn frame
    // by frame say so where they are chosen (CDS-81, CDS-83).
    await waitFor(() =>
      expect(styles.querySelector('svg [data-style="word-pop"]')).toBeInTheDocument(),
    )
    expect(within(styles).getByText(/프레임마다 그림/)).toBeInTheDocument()
    await user.click(within(styles).getByRole('checkbox', { name: /키노트/ }))
    await waitFor(() => expect(writes.at(-1)?.allowedCaptionStyles).toEqual(['keynote']), {
      timeout: 4000,
    })
  })

  it('changes the intro and outro presets on the project itself', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [template], projects: [project], projectWrites: writes },
    })
    const intro = await screen.findByRole('tablist', { name: '인트로 디자인' })
    expect(within(intro).getByRole('tab', { name: /위아래 가로선|B/ })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    await user.click(within(intro).getAllByRole('tab')[0])
    await waitFor(() => expect(writes.at(-1)?.introPreset).toBe('a'), { timeout: 4000 })
    const outro = screen.getByRole('tablist', { name: '아웃트로 디자인' })
    await user.click(within(outro).getAllByRole('tab')[0])
    await waitFor(() => expect(writes.at(-1)?.outroPreset).toBe('b'), { timeout: 4000 })
  })

  it('fills all five from a template and leaves every one editable, and clearing it keeps them', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [template], projects: [project], projectWrites: writes },
    })
    const picker = await screen.findByRole('combobox', { name: /^영상 템플릿/ })
    await chooseOption(user, picker, '여행')
    await waitFor(
      () => {
        const last = writes.at(-1)
        expect(last?.videoTemplateId).toBe('template')
        expect(last?.introPreset).toBe('a')
        expect(last?.outroPreset).toBe('b')
        expect(last?.allowedCaptionStyles).toEqual(['bold'])
        expect(last?.captionPace).toBe('rapid')
        expect(last?.accent).toBe('teal')
      },
      { timeout: 4000 },
    )
    // Seeded, not fixed: the presets stay the project's to change afterwards.
    const outro = screen.getByRole('tablist', { name: '아웃트로 디자인' })
    await user.click(within(outro).getAllByRole('tab')[1])
    await waitFor(() => expect(writes.at(-1)?.outroPreset).toBe('e'), { timeout: 4000 })
    // Clearing the template carries nothing away: the values are the project's.
    await chooseOption(user, screen.getByRole('combobox', { name: /^영상 템플릿/ }), '없음')
    await waitFor(
      () => {
        const last = writes.at(-1)
        expect(last?.videoTemplateId).toBe('')
        expect(last?.introPreset).toBe('a')
        expect(last?.outroPreset).toBe('e')
        expect(last?.allowedCaptionStyles).toEqual(['bold'])
      },
      { timeout: 4000 },
    )
  })
})
