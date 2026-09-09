import { File as NodeFile } from 'node:buffer'
import { webcrypto, createHash } from 'node:crypto'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { readVideoMetadata } from '@/shared/lib/video'
import { CLIP_SOURCE_MAX_FILE_BYTES, CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES } from '@/shared/config'
import {
  checkSourceFiles,
  ClipSelectionError,
  readSourceManifest,
  sourceFingerprint,
} from './manifest'

vi.mock('@/shared/lib/video', () => ({ readVideoMetadata: vi.fn() }))
const metadata = { durationMs: 1234, width: 1920, height: 1080 }
const file = (name = 'a.mp4', type = 'video/mp4', content = 'clip') =>
  new NodeFile([content], name, { type }) as unknown as File
const sized = (size: number, name = 'a.mp4', type = 'video/mp4') => ({ name, type, size }) as File
afterEach(() => {
  vi.resetAllMocks()
  vi.unstubAllGlobals()
})

describe('clip manifest and bounded fingerprint', () => {
  it('pins the versioned binary digest and ignores filename and modification date', async () => {
    vi.stubGlobal('crypto', webcrypto)
    const prefix = Buffer.alloc(30)
    prefix[0] = 1
    prefix.writeBigUInt64BE(4n, 1)
    prefix.writeUInt32BE(9, 9)
    prefix.write('video/mp4', 13)
    prefix.writeBigUInt64BE(1234n, 22)
    const expected = createHash('sha256').update(prefix).update('clipclip').digest('hex')
    expect(await sourceFingerprint(file(), metadata)).toBe(expected)
    expect(await sourceFingerprint(file('renamed.mp4'), metadata)).toBe(expected)
    expect(await sourceFingerprint(file(), { ...metadata, durationMs: 1235 })).not.toBe(expected)
    expect(await sourceFingerprint(file('a.mov', 'video/quicktime'), metadata)).not.toBe(expected)
    expect(await sourceFingerprint(file('a.mp4', 'video/mp4', 'cliq'), metadata)).not.toBe(expected)
    expect(await sourceFingerprint(file('a.mp4', 'video/mp4', 'clips'), metadata)).not.toBe(
      expected,
    )
  })
  it('reads only two bounded slices even for a 2 GiB source', async () => {
    vi.stubGlobal('crypto', webcrypto)
    const arrayBuffer = vi.fn(async () => new ArrayBuffer(CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES))
    const slice = vi.fn(() => ({ arrayBuffer }))
    const large = { ...sized(CLIP_SOURCE_MAX_FILE_BYTES), slice } as unknown as File
    await sourceFingerprint(large, metadata)
    expect(slice.mock.calls).toEqual([[0, 65536], [CLIP_SOURCE_MAX_FILE_BYTES - 65536]])
    expect(arrayBuffer).toHaveBeenCalledTimes(2)
  })
  it.each([
    ['count', []],
    ['count', Array.from({ length: 21 }, () => sized(1))],
    ['fileBytes', [sized(0)]],
    ['fileBytes', [sized(CLIP_SOURCE_MAX_FILE_BYTES + 1)]],
    ['batchBytes', Array.from({ length: 5 }, () => sized(CLIP_SOURCE_MAX_FILE_BYTES))],
    ['container', [sized(1, 'a.avi')]],
    ['container', [sized(1, 'a.mp4', '')]],
    ['container', [sized(1, 'a.mov', 'video/mp4')]],
    ['filename', [sized(1, `${'a'.repeat(252)}.mp4`)]],
    ['filename', [sized(1, 'a\n.mp4')]],
  ] as const)('rejects %s before metadata reads or reservations', (reason, files) => {
    expect(() => checkSourceFiles(files)).toThrow(new ClipSelectionError(reason))
    expect(readVideoMetadata).not.toHaveBeenCalled()
  })
  it.each([
    ['mp4', 'video/mp4'],
    ['MOV', 'video/quicktime'],
    ['m4v', 'video/x-m4v'],
    ['m4v', 'video/mp4'],
    ['webm', 'video/webm'],
  ])('allows %s / %s', (ext, type) => {
    expect(() => checkSourceFiles([sized(1, `source.${ext}`, type)])).not.toThrow()
  })
  it.each([
    { ...metadata, width: 0 },
    { ...metadata, height: NaN },
    { ...metadata, durationMs: -1 },
    { ...metadata, durationMs: 1.5 },
  ])('rejects invalid metadata %o', async (invalid) => {
    vi.mocked(readVideoMetadata).mockResolvedValue(invalid)
    await expect(readSourceManifest([file()], new AbortController().signal)).rejects.toMatchObject({
      reason: 'metadata',
    })
  })
  it('checks combined duration and identical renamed sources', async () => {
    vi.stubGlobal('crypto', webcrypto)
    vi.mocked(readVideoMetadata).mockResolvedValue({ ...metadata, durationMs: 1_000_000 })
    await expect(
      readSourceManifest([file(), file('b.mp4')], new AbortController().signal),
    ).rejects.toMatchObject({ reason: 'duration' })
    vi.mocked(readVideoMetadata).mockResolvedValue(metadata)
    await expect(
      readSourceManifest([file(), file('b.mp4')], new AbortController().signal),
    ).rejects.toMatchObject({ reason: 'duplicate' })
  })
  it('returns metadata only and distinguishes unreadable media, crypto failure and abort', async () => {
    vi.stubGlobal('crypto', webcrypto)
    vi.mocked(readVideoMetadata).mockResolvedValue(metadata)
    const manifest = await readSourceManifest([file()], new AbortController().signal)
    expect(manifest[0]).toEqual({
      ...metadata,
      filename: 'a.mp4',
      contentType: 'video/mp4',
      bytes: 4,
      fingerprint: expect.stringMatching(/^[a-f0-9]{64}$/),
    })
    vi.mocked(readVideoMetadata).mockRejectedValueOnce(new Error('decode'))
    await expect(readSourceManifest([file()], new AbortController().signal)).rejects.toMatchObject({
      reason: 'metadata',
    })
    vi.stubGlobal('crypto', {})
    await expect(readSourceManifest([file()], new AbortController().signal)).rejects.toMatchObject({
      reason: 'fingerprint',
    })
    const controller = new AbortController()
    controller.abort()
    await expect(readSourceManifest([file()], controller.signal)).rejects.toMatchObject({
      name: 'AbortError',
    })
  })
})
