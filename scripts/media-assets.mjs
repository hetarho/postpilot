// Portable digest of the media asset inputs, distinct from the per-architecture
// executable/runtime fingerprint printed by `media-worker manifest`.
import { createHash } from 'node:crypto'
import { readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { repoRoot } from './lib.mjs'

export function mediaAssetDigest(root = repoRoot) {
  const files = []
  function visit(path) {
    for (const entry of readdirSync(path, { withFileTypes: true })) {
      const next = resolve(path, entry.name)
      if (entry.isDirectory()) visit(next)
      else if (entry.isFile()) files.push(next)
    }
  }
  visit(resolve(root, 'backend/assets/fonts'))
  visit(resolve(root, 'backend/internal/clip/overlay/presets'))
  files.push(resolve(root, 'backend/internal/clip/design/design.json'))
  files.push(resolve(root, 'backend/internal/clip/design/metrics.json'))
  const hash = createHash('sha256')
  for (const path of files.sort()) {
    hash.update(relative(root, path).replaceAll('\\', '/')).update('\0')
    hash.update(createHash('sha256').update(readFileSync(path)).digest()).update('\n')
  }
  return hash.digest('hex')
}
export function checkMediaAssetLabel(root = repoRoot) {
  const declared = readFileSync(resolve(root, 'backend/Dockerfile'), 'utf8').match(/^ARG MEDIA_ASSET_DIGEST=([a-f0-9]{64})$/m)?.[1]
  if (declared !== mediaAssetDigest(root)) throw new Error('Media asset label is stale: run node scripts/media-assets.mjs --update')
}
if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  if (process.argv.includes('--check')) checkMediaAssetLabel()
  else if (process.argv.includes('--update')) {
    const path = resolve(repoRoot, 'backend/Dockerfile')
    writeFileSync(path, readFileSync(path, 'utf8').replace(/^ARG MEDIA_ASSET_DIGEST=.*$/m, `ARG MEDIA_ASSET_DIGEST=${mediaAssetDigest()}`))
  } else console.log(mediaAssetDigest())
}
