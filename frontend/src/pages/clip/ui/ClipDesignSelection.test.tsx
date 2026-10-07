import {
  failLocalPresetSamplesForControls,
  localSamplesForControls,
} from '@/test/clip-local-samples'
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
  introPreset: 'b' as const,
  outroPreset: 'e' as const,
  allowedCaptionStyles: [] as string[],
}
afterEach(() => {
  discardClipDraftQueues()
  failLocalPresetSamplesForControls(false)
})

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
    // Nothing was chosen, so the new project stores intro A and outro B
    // (CLIP-111, CLIP-139).
    expect(writes[0].introPreset).toBe('a')
    expect(writes[0].outroPreset).toBe('b')
  })

  it('shows an existing project in the presets it already renders in, drawn by the renderer', async () => {
    const labels: string[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [template], projects: [project], regionSampleLabels: labels },
    })
    const intro = await screen.findByRole('radiogroup', { name: '인트로 디자인' })
    const outro = screen.getByRole('radiogroup', { name: '아웃트로 디자인' })
    // Every preset of each region is offered (CLIP-111, CDS-70).
    expect(within(intro).getAllByRole('radio')).toHaveLength(8)
    expect(within(outro).getAllByRole('radio')).toHaveLength(7)
    expect(within(intro).getByRole('radio', { name: 'B 위아래 가로선' })).toHaveAttribute(
      'aria-checked',
      'true',
    )
    expect(within(outro).getByRole('radio', { name: 'E 점수 강조' })).toHaveAttribute(
      'aria-checked',
      'true',
    )
    expect(within(outro).getByRole('radio', { name: '원형 도장' })).toHaveAttribute(
      'aria-checked',
      'false',
    )
    // Each tile is the renderer's drawing of that preset, numbered with ①'s own
    // label (CLIP-165).
    await waitFor(() =>
      expect(intro.querySelector('svg [data-preset="intro-sticker"]')).toBeInTheDocument(),
    )
    expect(outro.querySelector('svg [data-preset="outro-stamp"]')).toHaveTextContent('슬롯 1')
    expect(labels).toEqual([])
  })

  it('still lets the owner choose when the local drawings fail', async () => {
    failLocalPresetSamplesForControls(true)
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: {
        templates: [template],
        projects: [project],
        projectWrites: writes,
        regionSamplesFail: true,
      },
    })
    const intro = await screen.findByRole('radiogroup', { name: '인트로 디자인' })
    expect(intro.querySelector('svg')).not.toBeInTheDocument()
    await user.click(within(intro).getByRole('radio', { name: '매거진 커버' }))
    await waitFor(() => expect(writes.at(-1)?.introPreset).toBe('cover'), { timeout: 4000 })
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
    expect(within(styles).getAllByText(/프레임마다 그림/).length).toBeGreaterThan(0)
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
    const intro = await screen.findByRole('radiogroup', { name: '인트로 디자인' })
    await user.click(within(intro).getByRole('radio', { name: 'A 크기만' }))
    await waitFor(() => expect(writes.at(-1)?.introPreset).toBe('a'), { timeout: 4000 })
    expect(within(intro).getByRole('radio', { name: 'A 크기만' })).toHaveAttribute(
      'aria-checked',
      'true',
    )
    // Only the chosen tile is a tab stop, and the arrows move focus and the
    // choice together.
    expect(within(intro).getByRole('radio', { name: 'A 크기만' })).toHaveAttribute('tabindex', '0')
    expect(within(intro).getByRole('radio', { name: '주아 스티커' })).toHaveAttribute(
      'tabindex',
      '-1',
    )
    await user.keyboard('{ArrowRight}')
    await waitFor(() => expect(writes.at(-1)?.introPreset).toBe('b'), { timeout: 4000 })
    expect(within(intro).getByRole('radio', { name: 'B 위아래 가로선' })).toHaveFocus()
    await user.keyboard('{ArrowLeft}{ArrowLeft}')
    await waitFor(() => expect(writes.at(-1)?.introPreset).toBe('sticker'), { timeout: 4000 })
    const outro = screen.getByRole('radiogroup', { name: '아웃트로 디자인' })
    await user.click(within(outro).getByRole('radio', { name: '칩 줄' }))
    await waitFor(() => expect(writes.at(-1)?.outroPreset).toBe('chips'), { timeout: 4000 })
  })

  // CLIP-168: choosing a template takes its selection in the same write, silently, and the form
  // shows it; the pace and the accent stay the project's own; 없음 carries nothing away.
  it('takes the template’s design selection on choosing it and keeps it on 없음', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    const designed = {
      ...template,
      introPreset: 'cover' as const,
      outroPreset: 'e' as const,
      allowedCaptionStyles: ['neon'],
    }
    renderAppAt('/clips/project', {
      user: { id: 'alice' },
      clips: { templates: [designed], projects: [project], projectWrites: writes },
    })
    const outro = await screen.findByRole('radiogroup', { name: '아웃트로 디자인' })
    await user.click(within(outro).getByRole('radio', { name: '원형 도장' }))
    await waitFor(() => expect(writes.at(-1)?.outroPreset).toBe('stamp'), { timeout: 4000 })
    const picker = screen.getByRole('combobox', { name: /^영상 템플릿/ })
    await chooseOption(user, picker, '여행')
    await waitFor(
      () => {
        const last = writes.at(-1)
        expect(last?.videoTemplateId).toBe('template')
        expect(last?.introPreset).toBe('cover')
        expect(last?.outroPreset).toBe('e')
        expect(last?.allowedCaptionStyles).toEqual(['neon'])
        expect(last?.captionPace).toBe('')
        expect(last?.accent).toBe('')
      },
      { timeout: 4000 },
    )
    const intro = screen.getByRole('radiogroup', { name: '인트로 디자인' })
    expect(within(intro).getByRole('radio', { name: '매거진 커버' })).toHaveAttribute(
      'aria-checked',
      'true',
    )
    expect(within(outro).getByRole('radio', { name: 'E 점수 강조' })).toHaveAttribute(
      'aria-checked',
      'true',
    )
    const styles = screen.getByRole('group', { name: '자막 스타일' })
    expect(within(styles).getByRole('checkbox', { name: /네온 사인/ })).toBeChecked()
    // The settled form is in sync: no further write pushes the old selection back.
    const settled = writes.length
    await new Promise((resolve) => setTimeout(resolve, 1500))
    expect(writes).toHaveLength(settled)
    await chooseOption(user, screen.getByRole('combobox', { name: /^영상 템플릿/ }), '없음')
    await waitFor(
      () => {
        const last = writes.at(-1)
        expect(last?.videoTemplateId).toBe('')
        expect(last?.introPreset).toBe('cover')
        expect(last?.allowedCaptionStyles).toEqual(['neon'])
      },
      { timeout: 4000 },
    )
  })

  it('mints a project in the chosen template’s selection', async () => {
    const user = userEvent.setup()
    const writes: ClipProjectDraft[] = []
    const designed = { ...template, introPreset: 'serif' as const, outroPreset: 'chips' as const }
    renderAppAt('/clips/new', {
      user: { id: 'alice' },
      clips: { templates: [designed], projectWrites: writes },
    })
    await chooseOption(user, await screen.findByRole('combobox', { name: /^영상 템플릿/ }), '여행')
    await user.type(screen.getByLabelText('클립 제목'), '고기')
    await user.click(screen.getByRole('button', { name: '클립 만들기' }))
    await waitFor(() => expect(writes).toHaveLength(1))
    expect(writes[0].introPreset).toBe('serif')
    expect(writes[0].outroPreset).toBe('chips')
  })
})

vi.mock('@/entities/clip-preview/ui/useLocalSamples', () => localSamplesForControls)
