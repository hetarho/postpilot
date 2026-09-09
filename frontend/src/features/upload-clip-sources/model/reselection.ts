import type { ClipSourceMetadata, RetainedClipSource } from '@/entities/clip-project'

export class ClipSourceMismatchError extends Error {
  constructor(
    readonly missing: string[],
    readonly unexpected: string[],
  ) {
    super('Clip sources do not match the saved plan')
    this.name = 'ClipSourceMismatchError'
  }
}
export function matchClipSources(
  manifest: readonly ClipSourceMetadata[],
  required: readonly RetainedClipSource[],
) {
  const selected = new Set(manifest.map((s) => s.fingerprint))
  const expected = new Set(required.map((s) => s.fingerprint))
  const missing = required.filter((s) => !selected.has(s.fingerprint)).map((s) => s.filename)
  const unexpected = manifest.filter((s) => !expected.has(s.fingerprint)).map((s) => s.filename)
  if (missing.length || unexpected.length || manifest.length !== required.length)
    throw new ClipSourceMismatchError(missing, unexpected)
}
