import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CLIP_DEFAULT_REGION_PRESETS } from '@/entities/clip-design'
import { parseClipTemplate } from '@/entities/clip-template'
import { toClipWizardBody } from '../lib/to-composition'
import type { StageDraft } from '../model/wizard-machine'
import { ClipWizardPreview } from './ClipWizardPreview'

const stage = (patch: Partial<StageDraft>): StageDraft => ({
  id: 'hook',
  role: 'hook',
  name: '훅 (첫 장면)',
  heading: '훅 (첫 장면)',
  intent: '첫 장면에 보여 줄 것',
  checked: true,
  asks: false,
  fieldLabel: '',
  ...patch,
})
const draw = (stages: StageDraft[], props: Partial<Parameters<typeof ClipWizardPreview>[0]> = {}) =>
  render(
    <ClipWizardPreview
      stages={stages}
      // A blank heading does not parse; the wizard keeps the last parsed document on show.
      document={parseClipTemplate(
        toClipWizardBody(stages.map((s) => ({ ...s, heading: s.heading.trim() || s.name }))),
      )}
      design={{
        introPreset: CLIP_DEFAULT_REGION_PRESETS.intro,
        outroPreset: CLIP_DEFAULT_REGION_PRESETS.outro,
        allowedCaptionStyles: [],
      }}
      {...props}
    />,
  )
const listed = () => within(screen.getByRole('region', { name: '구성 단계' }))

// CLIP-221: stages are not drawn on the clip, so the preview lists them beside it.
describe('the video wizard preview', () => {
  it('lists the checked stages in order by heading and marks the one on screen', () => {
    draw(
      [
        stage({ id: 'hook', heading: '처음 만난 카페' }),
        stage({ id: 'detail', role: 'detail', name: '디테일·분위기', heading: '  ' }),
        stage({ id: 'drawbacks', role: 'drawbacks', name: '아쉬운 점', checked: false }),
        stage({ id: 'ending', role: 'ending', name: '마무리 한 줄', heading: '다시 올 집' }),
      ],
      { currentId: 'detail' },
    )
    const items = listed().getAllByRole('listitem')
    expect(items.map((item) => item.textContent)).toEqual([
      '1처음 만난 카페',
      '2디테일·분위기',
      '3다시 올 집',
    ])
    expect(items[1]).toHaveAttribute('aria-current', 'step')
    expect(items[0]).not.toHaveAttribute('aria-current')
    expect(screen.getByRole('region', { name: '구성 미리보기' })).toBeInTheDocument()
  })

  it('says how to start when no stage is checked', () => {
    draw([stage({ checked: false })])
    expect(listed().getByText('넣을 단계를 고르면 여기에 영상 흐름이 보여요.')).toBeInTheDocument()
  })

  it('draws the design selection without offering it unless the design step hands in a change', async () => {
    const user = userEvent.setup()
    const { unmount } = draw([stage({})])
    expect(screen.queryByRole('combobox', { name: /^인트로 디자인/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('group', { name: /자막 스타일/ })).not.toBeInTheDocument()
    unmount()
    const onDesignChange = vi.fn()
    draw([stage({})], { onDesignChange })
    expect(screen.getByRole('combobox', { name: /^인트로 디자인/ })).toBeInTheDocument()
    await user.click(screen.getByRole('checkbox', { name: '네온 사인' }))
    expect(onDesignChange).toHaveBeenCalledWith({ allowedCaptionStyles: ['neon'] })
  })
})
