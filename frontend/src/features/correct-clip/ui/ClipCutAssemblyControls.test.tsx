import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, it, vi } from 'vitest'
import { initializeI18n } from '@/app/providers/i18n'
import { clipTimelineFixture } from '@/test/clip-editing'
import { ClipCutAssemblyControls } from './ClipCutAssemblyControls'

afterEach(() => initializeI18n('ko'))
it.each(['ko', 'en'] as const)(
  'offers only server-authorized rates and two source-time split actions in %s',
  async (lang) => {
    initializeI18n(lang)
    const cut = clipTimelineFixture().plan.cuts[0]
    const onChange = vi.fn(),
      onSplit = vi.fn(async () => {})
    render(
      <ClipCutAssemblyControls
        cut={cut}
        allowedRates={[1000, 1250, 1500, 2000]}
        playheadMs={7000}
        onChange={onChange}
        onSplit={onSplit}
        native
      />,
    )
    await userEvent.click(screen.getByRole('combobox', { name: /재생 속도|Playback rate/ }))
    const slow = screen.getByRole('option', { name: /^0.5×/ })
    expect(slow).toHaveAttribute('aria-disabled', 'true')
    await userEvent.click(slow)
    expect(onChange).not.toHaveBeenCalled()
    await userEvent.click(screen.getByRole('option', { name: '2×' }))
    expect(onChange).toHaveBeenCalledWith({ type: 'rate', id: cut.id, ratePermille: 2000 })
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '6.123' } })
    await userEvent.click(
      screen.getByRole('button', { name: /입력한 시간에서 나누기|Split at entered time/ }),
    )
    expect(onSplit).toHaveBeenLastCalledWith(6123)
    await userEvent.click(
      screen.getByRole('button', { name: /재생 위치에서 나누기|Split at playhead/ }),
    )
    expect(onSplit).toHaveBeenLastCalledWith(7000)
  },
)
