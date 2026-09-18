import { Stretch } from '@soundtouchjs/core'

/** WSOLA changes duration while preserving pitch, as the server's atempo does. */
export function stretchStereo(
  channels: readonly Float32Array[],
  sampleRate: number,
  rate: number,
  outputFrames: number,
  gain = 1,
) {
  if (
    channels.length !== 2 ||
    channels[0].length !== channels[1].length ||
    rate <= 0 ||
    !Number.isFinite(rate) ||
    !Number.isSafeInteger(outputFrames) ||
    outputFrames < 0
  )
    throw new Error('Invalid time stretch')
  const output = [new Float32Array(outputFrames), new Float32Array(outputFrames)]
  if (rate === 1) {
    output.forEach((channel, index) => {
      for (let frame = 0; frame < Math.min(outputFrames, channels[index].length); frame++)
        channel[frame] = channels[index][frame] * gain
    })
    return output
  }
  const stretch = new Stretch({ sampleRate, createBuffers: true })
  stretch.tempo = rate
  const block = new Float32Array(4096 * 2)
  let read = 0,
    written = 0
  // Silence drains the last WSOLA window; only the declared output length is retained.
  const padding = stretch.sampleReq * 4 + stretch.overlapLength
  while (written < outputFrames && read < channels[0].length + padding) {
    const count = Math.min(block.length / 2, channels[0].length + padding - read)
    block.fill(0)
    for (let index = 0; index < count && read + index < channels[0].length; index++) {
      block[index * 2] = channels[0][read + index]
      block[index * 2 + 1] = channels[1][read + index]
    }
    stretch.inputBuffer!.putSamples(block, 0, count)
    stretch.process()
    read += count
    while (stretch.outputBuffer!.frameCount && written < outputFrames) {
      const available = Math.min(
        block.length / 2,
        stretch.outputBuffer!.frameCount,
        outputFrames - written,
      )
      stretch.outputBuffer!.extract(block, 0, available)
      stretch.outputBuffer!.receive(available)
      for (let index = 0; index < available; index++) {
        output[0][written + index] = block[index * 2] * gain
        output[1][written + index] = block[index * 2 + 1] * gain
      }
      written += available
    }
  }
  stretch.clear()
  if (written !== outputFrames) throw new Error('Incomplete time stretch')
  return output
}
