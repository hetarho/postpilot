/** `IMG_1.HEIC` → `{ stem: 'IMG_1', extension: 'heic' }`. The extension is lower-cased
 *  and has no dot; a name with no dot has an empty extension. Any path prefix is dropped. */
export function splitFilename(name: string): { stem: string; extension: string } {
  const base = name.slice(name.lastIndexOf('/') + 1)
  const dot = base.lastIndexOf('.')
  if (dot === -1) return { stem: base, extension: '' }
  return { stem: base.slice(0, dot), extension: base.slice(dot + 1).toLowerCase() }
}

export function fileExtension(name: string): string {
  return splitFilename(name).extension
}

/** The name the uploaded copy is filed under: the original's stem with a `.jpg`
 *  extension, since that is what the object actually is after conversion. */
export function jpegFilename(originalName: string): string {
  const stem = splitFilename(originalName).stem.trim().replace(/\s+/g, ' ')
  return `${stem || 'photo'}.jpg`
}

/** `IMG_1.jpg` → `IMG_1 (2).jpg`, `IMG_1 (3).jpg`, … until it is not in `taken`.
 *
 *  A filename is unique within a post on the server (spec/legacy/policy/uploads.md); renaming
 *  here, before asking, is what turns that constraint into a non-event for the user. */
export function dedupeFilename(name: string, taken: Iterable<string>): string {
  const used = new Set(taken)
  if (!used.has(name)) return name
  const { stem, extension } = splitFilename(name)
  const suffix = extension ? `.${extension}` : ''
  for (let serial = 2; ; serial += 1) {
    const candidate = `${stem} (${serial})${suffix}`
    if (!used.has(candidate)) return candidate
  }
}
