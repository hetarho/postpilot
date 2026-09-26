import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, it } from 'vitest'
import { chooseOption } from '@/test/listbox'
import { parseClipComposition } from '../lib/composition-parse'
import { CompositionPreview } from './CompositionPreview'

// A template carries no design (CLIP-14), so the preview itself offers every intro
// and outro preset to look at the outline in, starting at a new project's.
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
