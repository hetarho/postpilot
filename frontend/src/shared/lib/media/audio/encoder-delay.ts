/** Find actual codec priming against the first audible input boundary, including
 * virtual leading silence so a periodic tone cannot match at a later period.
 * The search is bounded by the extra frames the decoder actually emitted. */
export function audioEncoderDelay(
  reference: readonly Float32Array[],
  decoded: readonly Float32Array[],
) {
  let peak = 0,
    channel = 0
  for (let c = 0; c < reference.length; c++) {
    for (const value of reference[c])
      if (Math.abs(value) > peak) {
        peak = Math.abs(value)
        channel = c
      }
  }
  if (!peak) return 0
  const original = reference[channel],
    actual = decoded[channel]
  const onset = original.findIndex((value) => Math.abs(value) >= peak * 0.02)
  const start = onset - 512
  const length = Math.min(4096, original.length - Math.max(0, start))
  const maxDelay = Math.max(0, actual.length - original.length)
  let best = Infinity,
    delay = 0
  for (let candidate = 0; candidate <= maxDelay; candidate++) {
    let error = 0
    for (let frame = 0; frame < length; frame++) {
      const position = start + frame
      const input = position >= 0 ? original[position] : 0
      const sample = position + candidate >= 0 ? actual[position + candidate] : 0
      error += (input - sample) ** 2
    }
    if (error < best) {
      best = error
      delay = candidate
    }
  }
  return delay
}
