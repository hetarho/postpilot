import { expect, it } from 'vitest'
import { audioEncoderDelay } from './encoder-delay'

it.each([0, 1024, 2112])(
  'measures %i priming samples without confusing a tone period with delay',
  (delay) => {
    const reference = Float32Array.from(
      { length: 10000 },
      (_, i) => 0.1 * Math.sin((2 * Math.PI * 1000 * i) / 48000),
    )
    const decoded = new Float32Array(reference.length + 3072)
    decoded.set(reference, delay)
    expect(audioEncoderDelay([reference], [decoded])).toBe(delay)
  },
)
it('finds the boundary after a silent opening instead of correlating only silence', () => {
  const reference = new Float32Array(20000)
  reference.set(
    Float32Array.from({ length: 4000 }, (_, i) => Math.sin(i * 0.7) * 0.1),
    12000,
  )
  const decoded = new Float32Array(reference.length + 3072)
  decoded.set(reference, 2112)
  expect(audioEncoderDelay([reference], [decoded])).toBe(2112)
})
