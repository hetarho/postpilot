import { readFileSync } from 'node:fs'
import {
  BlobSource,
  EncodedPacketSink,
  EncodedVideoPacketSource,
  Input,
  MP4,
  Mp4OutputFormat,
  Output,
} from 'mediabunny'
import { describe, expect, it } from 'vitest'
import { boundedMediaCopyTarget, runMediaSizeAttempts } from './analysis-copy'

const fixture = readFileSync(new URL('./testdata/analysis-silent.mp4', import.meta.url))
async function remux(cap: number) {
  const input = new Input({ formats: [MP4], source: new BlobSource(new Blob([fixture])) })
  const target = boundedMediaCopyTarget(cap),
    output = new Output({
      format: new Mp4OutputFormat({ fastStart: false }),
      target: target.target,
    })
  try {
    const video = (await input.getVideoTracks())[0],
      source = new EncodedVideoPacketSource('avc')
    output.addVideoTrack(source, { frameRate: 15 })
    await output.start()
    const decoderConfig = (await video.getDecoderConfig())!
    for await (const packet of new EncodedPacketSink(video).packets())
      await source.add(packet, { decoderConfig })
    source.close()
    await output.finalize()
    return target.result()
  } finally {
    await output.cancel()
    input.dispose()
  }
}
describe('finite position-aware completed media target', () => {
  it('finalizes actual MP4 packets and header rewrites inside a fixed allocation', async () => {
    const buffer = await remux(8 * 1024 * 1024)
    const input = new Input({ formats: [MP4], source: new BlobSource(new Blob([buffer])) })
    try {
      const video = (await input.getVideoTracks())[0]
      expect(await video.getCodec()).toBe('avc')
      expect((await video.computePacketStats()).packetCount).toBe(30)
      expect(await video.computeDuration()).toBeCloseTo(2, 5)
      expect(buffer.byteLength).toBeLessThan(8 * 1024 * 1024)
    } finally {
      input.dispose()
    }
  })
  it('refuses cap crossing during actual mux instead of publishing a truncated MP4', async () => {
    await expect(remux(64)).rejects.toThrow('CLIP_ANALYSIS_COPY_TOO_LARGE')
  })
  it('retries the actual over-cap mux once at the specified lower bitrate and only returns a finalized copy', async () => {
    const attempts: number[] = []
    const buffer = await runMediaSizeAttempts(
      [900000, 650000],
      new AbortController().signal,
      async (bitrate) => {
        attempts.push(bitrate)
        return await remux(bitrate === 900000 ? 64 : 8 * 1024 * 1024)
      },
    )
    expect(attempts).toEqual([900000, 650000])
    const input = new Input({ formats: [MP4], source: new BlobSource(new Blob([buffer])) })
    try {
      expect(await input.computeDuration()).toBeCloseTo(2, 5)
    } finally {
      input.dispose()
    }
    let failures = 0
    await expect(
      runMediaSizeAttempts([900000, 650000], new AbortController().signal, async () => {
        failures++
        throw new Error('CLIP_ANALYSIS_COPY_COVERAGE')
      }),
    ).rejects.toThrow('CLIP_ANALYSIS_COPY_COVERAGE')
    expect(failures).toBe(1)
  })
})
