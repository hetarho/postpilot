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

/** 400 ms blocks / 75% overlap, absolute −70 LUFS and relative −10 LU gates. */
export function integratedLoudness48k(channels: readonly Float32Array[]) {
  checkedStereo(channels)
  const step = 4800
  const bins = new Float64Array(Math.floor(channels[0].length / step))
  for (const channel of channels) {
    const shelf = biquad(SHELF),
      highPass = biquad(HIGH_PASS)
    for (let index = 0; index < bins.length * step; index++) {
      if (!Number.isFinite(channel[index])) throw new Error('Non-finite audio sample')
      const sample = highPass(shelf(channel[index]))
      bins[Math.floor(index / step)] += sample * sample
    }
  }
  const absolute: number[] = []
  for (let index = 0; index + 3 < bins.length; index++) {
    const power = (bins[index] + bins[index + 1] + bins[index + 2] + bins[index + 3]) / (step * 4)
    if (db(power) > -70) absolute.push(power)
  }
  if (!absolute.length) return -Infinity
  const gate = db(average(absolute)) - 10
  return db(average(absolute.filter((power) => db(power) > gate)))
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

/** One linear gain. A peak-limited shortfall remains measurable for the output verdict. */
export function normalizeLoudness48k(
  channels: Float32Array[],
  targetLUFS: number,
  ceilingDBTP: number,
) {
  const before = integratedLoudness48k(channels)
  const peak = truePeak48k(channels)
  if (!Number.isFinite(before))
    return { loudnessLUFS: undefined, truePeakDBTP: peak, silent: true, gain: 1 }
  const gainDB = Math.min(targetLUFS - before, ceilingDBTP - peak)
  const gain = 10 ** (gainDB / 20)
  for (const channel of channels)
    for (let index = 0; index < channel.length; index++) channel[index] *= gain
  return {
    loudnessLUFS: integratedLoudness48k(channels),
    truePeakDBTP: peak + gainDB,
    silent: false,
    gain,
  }
}
