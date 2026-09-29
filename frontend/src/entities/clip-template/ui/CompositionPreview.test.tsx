import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { chooseOption } from '@/test/listbox'
import { parseClipComposition } from '../lib/composition-parse'
import { CompositionPreview } from './CompositionPreview'

// Left to itself the preview offers every intro and outro preset to look at the outline in,
// starting at a new project's; the editor hands it the template's own (CLIP-166).
it('draws the outline in whichever intro and outro preset the preview chooses', async () => {
  const user = userEvent.setup()
  const document = parseClipComposition(
    '<clip version="1"><text id="opening" kind="fixed" role="hook" basis="output-start" start="0" end="2.5"><row>연남동</row><row>숯불 한우</row><row>서울</row></text>' +
      '<text id="closing" kind="fixed" role="ending" basis="output-end" start="-3" end="0"><row>해미 한우</row><row>다시 올 집</row><row>2026 기록</row><row>블로그에</row></text></clip>',
  )
  const view = render(<CompositionPreview document={document} />)
  const region = (kind: string) => view.container.querySelector(`[data-region="${kind}"]`)
  expect(region('intro')).toHaveAttribute('data-preset', 'a')
  await chooseOption(
    user,
    screen.getByRole('combobox', { name: /^인트로 디자인/ }),
    '큰 외곽선 글자',
  )
  expect(region('intro')).toHaveAttribute('data-preset', 'outline')
  expect(region('intro')?.querySelector('text[data-slot="1"]')).toHaveAttribute('fill', 'none')
  // The outro shows at the clip's end: move the preview there, then choose the stamp.
  await chooseOption(user, screen.getByRole('combobox', { name: /^아웃트로 디자인/ }), '원형 도장')
  const time = screen.getByRole('slider', { name: /확인할 시점/ })
  fireEvent.change(time, { target: { value: String(Number(time.getAttribute('max')) - 1000) } })
  expect(region('outro')).toHaveAttribute('data-preset', 'stamp')
  expect(region('outro')?.querySelectorAll('circle[data-shape="ring"]')).toHaveLength(2)
})

// CLIP-171: the scrubber's end is the clip's last frame — the outro and the badge as they stand
// at the final instant — never an empty frame.
it('shows the outro and the badge at the scrubber’s end', () => {
  const document = parseClipComposition(
    '<clip version="1"><text id="opening" kind="fixed" role="hook" basis="output-start"><row>연남동</row></text>' +
      '<text id="closing" kind="fixed" role="ending" basis="output-end"><row>다시 올 집</row></text>' +
      '<text id="ad" kind="fixed" role="badge" basis="whole">광고</text></clip>',
  )
  const view = render(<CompositionPreview document={document} />)
  const time = screen.getByRole('slider', { name: /확인할 시점/ })
  fireEvent.change(time, { target: { value: time.getAttribute('max')! } })
  expect(view.container.querySelector('[data-region="outro"]')).toBeInTheDocument()
  expect(view.container.querySelector('[data-disclosure]')).toBeInTheDocument()
  expect(view.container.querySelector('[data-region="intro"]')).not.toBeInTheDocument()
})

// CLIP-169, CLIP-170: the sample duration spans CLIP-7's 15–60 s, and moving the scrubber shows
// the captions advancing, each in the template's next allowed style.
it('advances the sample captions in the template’s styles as the scrubber moves', () => {
  const document = parseClipComposition('<clip version="1"/>')
  const view = render(<CompositionPreview document={document} captionStyles={['neon', 'glitch']} />)
  const duration = screen.getByRole('slider', { name: /예시 영상 길이/ })
  expect(duration).toHaveAttribute('min', '15000')
  expect(duration).toHaveAttribute('max', '60000')
  const time = screen.getByRole('slider', { name: /확인할 시점/ })
  const shown = () => {
    const caption = view.container.querySelector('[data-role="caption"]')
    return [caption?.textContent, caption?.getAttribute('data-caption-style')]
  }
  fireEvent.change(time, { target: { value: '0' } })
  expect(shown()).toEqual(['샘플 자막 1', 'neon'])
  fireEvent.change(time, { target: { value: '4000' } })
  expect(shown()).toEqual(['샘플 자막 2', 'glitch'])
  fireEvent.change(time, { target: { value: '8000' } })
  expect(shown()).toEqual(['샘플 자막 3', 'neon'])
})
