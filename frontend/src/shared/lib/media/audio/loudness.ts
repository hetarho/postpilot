// ITU-R BS.1770-5, Annex 1 tables 1/2 and Annex 2's four-phase true-peak FIR.
// Coefficients are defined at 48 kHz; callers resample before measurement.
const SHELF = [
  1.53512485958697, -2.69169618940638, 1.19839281085285, -1.69065929318241, 0.73248077421585,
] as const
const HIGH_PASS = [1, -2, 1, -1.99004745483398, 0.99007225036621] as const
const PEAK_FIR = [
  [0.001708984375, -0.0291748046875, -0.0189208984375, -0.00830078125],
  [0.010986328125, 0.029296875, 0.0330810546875, 0.014892578125],
  [-0.0196533203125, -0.0517578125, -0.0582275390625, -0.026611328125],
  [0.033203125, 0.089111328125, 0.1015625, 0.047607421875],
  [-0.0594482421875, -0.16650390625, -0.2003173828125, -0.102294921875],
  [0.1373291015625, 0.465087890625, 0.77978515625, 0.97216796875],
  [0.97216796875, 0.77978515625, 0.465087890625, 0.1373291015625],
  [-0.102294921875, -0.2003173828125, -0.16650390625, -0.0594482421875],
  [0.047607421875, 0.1015625, 0.089111328125, 0.033203125],
  [-0.026611328125, -0.0582275390625, -0.0517578125, -0.0196533203125],
  [0.014892578125, 0.0330810546875, 0.029296875, 0.010986328125],
  [-0.00830078125, -0.0189208984375, -0.0291748046875, 0.001708984375],
]
const db = (power: number) => -0.691 + 10 * Math.log10(power)
const average = (values: number[]) => values.reduce((sum, value) => sum + value, 0) / values.length
function checkedStereo(channels: readonly Float32Array[]) {
  if (
    channels.length < 1 ||
    channels.length > 2 ||
    channels.some((channel) => channel.length !== channels[0].length)
  )
    throw new Error('Invalid mono/stereo PCM')
}
function biquad(coefficients: readonly number[]) {
  let x1 = 0,
    x2 = 0,
    y1 = 0,
    y2 = 0
  return (sample: number) => {
    const out =
      coefficients[0] * sample +
      coefficients[1] * x1 +
      coefficients[2] * x2 -
      coefficients[3] * y1 -
      coefficients[4] * y2
    x2 = x1
    x1 = sample
    y2 = y1
    y1 = out
    return out
  }
}

function weightedPowerBins48k(channels: readonly Float32Array[], tailBins = 0) {
  checkedStereo(channels)
  const step = 4800
  const bins = new Float64Array(
    (tailBins ? Math.ceil(channels[0].length / step) : Math.floor(channels[0].length / step)) +
      tailBins,
  )
  for (const channel of channels) {
    const shelf = biquad(SHELF),
      highPass = biquad(HIGH_PASS)
    for (let index = 0; index < bins.length * step; index++) {
      const value = index < channel.length ? channel[index] : 0
      if (!Number.isFinite(value)) throw new Error('Non-finite audio sample')
      const sample = highPass(shelf(value))
      bins[Math.floor(index / step)] += sample * sample
    }
  }
  return bins
}
/** 400 ms blocks / 75% overlap, absolute −70 LUFS and relative −10 LU gates. */
export function integratedLoudness48k(channels: readonly Float32Array[]) {
  const step = 4800
  const bins = weightedPowerBins48k(channels)
  const absolute: number[] = []
  for (let index = 0; index + 3 < bins.length; index++) {
    const power = (bins[index] + bins[index + 1] + bins[index + 2] + bins[index + 3]) / (step * 4)
    if (db(power) > -70) absolute.push(power)
  }
  if (!absolute.length) return -Infinity
  const gate = db(average(absolute)) - 10
  return db(average(absolute.filter((power) => db(power) > gate)))
}

/** EBU Tech3342: 3s short-term windows at10Hz, absolute -70LUFS and
 * relative -20LU gates, then the10th/95th percentile spread. The file measure
 * includes the prescribed1.5s silent analysis tail; no PCM tail is exported.
 * https://tech.ebu.ch/docs/tech/tech3342.pdf */
export function loudnessRange48k(channels: readonly Float32Array[]) {
  const bins = weightedPowerBins48k(channels, 15)
  const powers: number[] = []
  let power = 0
  for (let index = 0; index < bins.length; index++) {
    power += bins[index]
    if (index >= 30) power -= bins[index - 30]
    if (index >= 29) {
      const value = Math.max(0, power / (30 * 4800))
      if (db(value) >= -70) powers.push(value)
    }
  }
  if (!powers.length) return 0
  const relativeGate = average(powers) / 100
  const levels = powers
    .filter((value) => value >= relativeGate)
    .map(db)
    .sort((a, b) => a - b)
  if (!levels.length) return 0
  return (
    levels[Math.round((levels.length - 1) * 0.95)] - levels[Math.round((levels.length - 1) * 0.1)]
  )
}

/** Four-times oversampling, including the filter tail. Floating point needs no attenuation. */
export function truePeak48k(channels: readonly Float32Array[]) {
  checkedStereo(channels)
  let peak = 0
  for (const channel of channels) {
    for (let index = 0; index < channel.length + PEAK_FIR.length - 1; index++) {
      const sample = index < channel.length ? channel[index] : 0
      if (!Number.isFinite(sample)) throw new Error('Non-finite audio sample')
      peak = Math.max(peak, Math.abs(sample))
      for (let phase = 0; phase < 4; phase++) {
        let interpolated = 0
        for (let tap = 0; tap < PEAK_FIR.length; tap++) {
          const source = index - tap
          if (source >= 0 && source < channel.length)
            interpolated += channel[source] * PEAK_FIR[tap][phase]
        }
        peak = Math.max(peak, Math.abs(interpolated))
      }
    }
  }
  return 20 * Math.log10(peak)
}

/** A peak-bound mix reaches the target the way loudnorm's dynamic fallback does
 * (CDS-35): an offline stereo-linked lookahead limiter, then re-measurement. */
const LIMITER = {
  lookaheadFrames: 72, // 1.5 ms at 48 kHz
  releaseFrames: 4800, // 100 ms recovery time constant
  marginDB: 1, // first sample-peak limit below the true-peak ceiling
  headroomDB: 0.5, // true-peak room left for AAC overshoot
  toleranceLU: 0.5,
  passes: 4,
} as const

function scale(channels: Float32Array[], gain: number) {
  for (const channel of channels)
    for (let index = 0; index < channel.length; index++) channel[index] *= gain
}

/** Each sample's gain is the trailing average of the forward-window minimum of
 * the required gain, so every sample peak lands at or below the limit. */
function limitPeaks(channels: Float32Array[], limit: number, envelope: Float32Array) {
  const length = channels[0].length,
    window = LIMITER.lookaheadFrames
  const required = (index: number) => {
    let peak = 0
    for (const channel of channels) peak = Math.max(peak, Math.abs(channel[index]))
    return peak > limit ? limit / peak : 1
  }
  // Monotonic ring deque over [index, index + window), filled from the end.
  const capacity = window + 1,
    indices = new Int32Array(capacity),
    values = new Float64Array(capacity)
  let head = 0,
    size = 0
  for (let index = length - 1; index >= 0; index--) {
    const value = required(index)
    while (size && values[(head + size - 1) % capacity] >= value) size--
    indices[(head + size) % capacity] = index
    values[(head + size) % capacity] = value
    size++
    while (indices[head] >= index + window) {
      head = (head + 1) % capacity
      size--
    }
    envelope[index] = values[head]
  }
  // Recovery never outruns the window minimum, which keeps every peak limited.
  const recovery = 1 - Math.exp(-1 / LIMITER.releaseFrames)
  for (let index = 1; index < length; index++)
    envelope[index] = Math.min(
      envelope[index],
      envelope[index - 1] + (1 - envelope[index - 1]) * recovery,
    )
  const trailing = new Float64Array(window).fill(1)
  let sum = window
  for (let index = 0; index < length; index++) {
    sum += envelope[index] - trailing[index % window]
    trailing[index % window] = envelope[index]
    const gain = Math.min(1, sum / window)
    for (const channel of channels) channel[index] *= gain
  }
}

/** One linear gain whenever it reaches the target under the ceiling; otherwise
 * the limiter path, re-measured until the loudness is within tolerance. */
export function normalizeLoudness48k(
  channels: Float32Array[],
  targetLUFS: number,
  ceilingDBTP: number,
) {
  const before = integratedLoudness48k(channels)
  const peak = truePeak48k(channels)
  if (!Number.isFinite(before))
    return { loudnessLUFS: undefined, truePeakDBTP: peak, silent: true, gain: 1, limited: false }
  if (targetLUFS - before <= ceilingDBTP - peak) {
    const gainDB = targetLUFS - before
    const gain = 10 ** (gainDB / 20)
    scale(channels, gain)
    return {
      loudnessLUFS: integratedLoudness48k(channels),
      truePeakDBTP: peak + gainDB,
      silent: false,
      gain,
      limited: false,
    }
  }
  const envelope = new Float32Array(channels[0].length)
  let loudness = before,
    truePeak = peak,
    gain = 1,
    limit = ceilingDBTP - LIMITER.marginDB
  for (let pass = 0; pass < LIMITER.passes; pass++) {
    const step = 10 ** ((targetLUFS - loudness) / 20)
    scale(channels, step)
    gain *= step
    limitPeaks(channels, 10 ** (limit / 20), envelope)
    loudness = integratedLoudness48k(channels)
    truePeak = truePeak48k(channels)
    const over = truePeak - (ceilingDBTP - LIMITER.headroomDB)
    if (over > 0) limit -= over
    else if (Math.abs(loudness - targetLUFS) <= LIMITER.toleranceLU) break
  }
  return { loudnessLUFS: loudness, truePeakDBTP: truePeak, silent: false, gain, limited: true }
}

/** CDS-35's final mix: a loudness range beyond its ceiling is refused before any gain. */
export function normalizeMix48k(
  channels: Float32Array[],
  targetLUFS: number,
  ceilingDBTP: number,
  rangeCeilingLU: number,
) {
  const loudnessRangeLU = loudnessRange48k(channels)
  if (!Number.isFinite(loudnessRangeLU) || loudnessRangeLU > rangeCeilingLU)
    throw new Error('AUDIO_LOUDNESS_RANGE_UNSUPPORTED')
  return { loudnessRangeLU, ...normalizeLoudness48k(channels, targetLUFS, ceilingDBTP) }
}
