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
import { explicitSquarePixelsMp4 } from './square-pixel-mp4'
import { boundedMediaCopyTarget, runMediaSizeAttempts } from './analysis-copy'

const fixture = readFileSync('src/shared/lib/media/testdata/analysis-silent.mp4')
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
    await output.cancel().catch(() => undefined)
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

describe('explicit square-pixel container compatibility', () => {
  it('inserts unit pasp into trailing moov without moving media or packet timing and is idempotent', async () => {
    const original = await remux(8 * 1024 * 1024)
    const patched = explicitSquarePixelsMp4(original, 8 * 1024 * 1024)
    expect(patched.byteLength).toBe(original.byteLength + 16)
    expect(explicitSquarePixelsMp4(patched, 8 * 1024 * 1024)).toBe(patched)
    const before = Buffer.from(original),
      after = Buffer.from(patched),
      moov = before.indexOf('moov') - 4
    expect(after.subarray(0, moov)).toEqual(before.subarray(0, moov))
    const packets = async (buffer: ArrayBuffer) => {
      const input = new Input({ formats: [MP4], source: new BlobSource(new Blob([buffer])) })
      try {
        const video = (await input.getVideoTracks())[0]
        expect(await video.getPixelAspectRatio()).toEqual({ num: 1, den: 1 })
        const values = []
        for await (const packet of new EncodedPacketSink(video).packets())
          values.push({
            timestamp: packet.timestamp,
            duration: packet.duration,
            data: Buffer.from(packet.data).toString('hex'),
          })
        return values
      } finally {
        input.dispose()
      }
    }
    expect(await packets(patched)).toEqual(await packets(original))
    for (const type of ['moov', 'trak', 'mdia', 'minf', 'stbl', 'stsd', 'avc1']) {
      const offset = before.lastIndexOf(type) - 4
      expect(after.readUInt32BE(offset)).toBe(before.readUInt32BE(offset) + 16)
    }
  })
  it('refuses non-square or duplicated pasp, malformed bounds, nontrailing moov and cap crossing', async () => {
    const original = await remux(8 * 1024 * 1024),
      patched = explicitSquarePixelsMp4(original, 8 * 1024 * 1024)
    const wrong = patched.slice(0),
      bytes = Buffer.from(wrong),
      pasp = bytes.indexOf('pasp') - 4
    bytes.writeUInt32BE(2, pasp + 8)
    expect(() => explicitSquarePixelsMp4(wrong, 8 * 1024 * 1024)).toThrow(
      'CLIP_ANALYSIS_COPY_PROFILE',
    )
    const duplicate = Buffer.concat([
      Buffer.from(patched).subarray(0, pasp + 16),
      Buffer.from(patched).subarray(pasp, pasp + 16),
      Buffer.from(patched).subarray(pasp + 16),
    ])
    for (const type of ['moov', 'trak', 'mdia', 'minf', 'stbl', 'stsd', 'avc1']) {
      const offset = Buffer.from(patched).lastIndexOf(type) - 4
      duplicate.writeUInt32BE(duplicate.readUInt32BE(offset) + 16, offset)
    }
    expect(() =>
      explicitSquarePixelsMp4(Uint8Array.from(duplicate).buffer, 8 * 1024 * 1024),
    ).toThrow('CLIP_ANALYSIS_COPY_PROFILE')
    const malformed = original.slice(0)
    new DataView(malformed).setUint32(0, original.byteLength + 1)
    expect(() => explicitSquarePixelsMp4(malformed, 8 * 1024 * 1024)).toThrow(
      'CLIP_ANALYSIS_COPY_PROFILE',
    )
    const appended = new Uint8Array(original.byteLength + 8)
    appended.set(new Uint8Array(original))
    new DataView(appended.buffer).setUint32(original.byteLength, 8)
    appended.set([102, 114, 101, 101], original.byteLength + 4)
    expect(() => explicitSquarePixelsMp4(appended.buffer, 8 * 1024 * 1024)).toThrow(
      'CLIP_ANALYSIS_COPY_PROFILE',
    )
    expect(() => explicitSquarePixelsMp4(original, original.byteLength + 15)).toThrow(
      'CLIP_ANALYSIS_COPY_TOO_LARGE',
    )
  })
})
