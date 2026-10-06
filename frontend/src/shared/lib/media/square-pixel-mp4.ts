interface Box {
  start: number
  end: number
  header: number
  type: string
  extended: boolean
}
const invalid = (): never => {
  throw new Error('CLIP_ANALYSIS_COPY_PROFILE')
}

/** Make the square-pixel contract explicit for native decoders. Mediabunny
 * intentionally omits unit pasp; hardware H.264 may also omit unit SPS SAR.
 * Only a bounded classic MP4 with trailing moov is eligible: media bytes and
 * every absolute chunk offset remain in their original position. */
export function explicitSquarePixelsMp4(buffer: ArrayBuffer, maxBytes: number): ArrayBuffer {
  if (buffer.byteLength < 8 || buffer.byteLength > maxBytes) invalid()
  const bytes = new Uint8Array(buffer),
    view = new DataView(buffer)
  const text = (start: number) => String.fromCharCode(...bytes.subarray(start, start + 4))
  const boxes = (start: number, end: number): Box[] => {
    const result: Box[] = []
    while (start < end) {
      if (end - start < 8) invalid()
      const size32 = view.getUint32(start),
        extended = size32 === 1
      const header = extended ? 16 : 8
      if (end - start < header || size32 === 0) invalid()
      const rawSize = extended ? view.getBigUint64(start + 8) : BigInt(size32)
      if (rawSize < BigInt(header) || rawSize > BigInt(end - start)) invalid()
      const size = Number(rawSize)
      result.push({ start, end: start + size, header, type: text(start + 4), extended })
      if (result.length > 4096) invalid()
      start += size
    }
    return result
  }
  const child = (parent: Box, type: string) => {
    const found = boxes(parent.start + parent.header, parent.end).filter((box) => box.type === type)
    if (found.length !== 1) invalid()
    return found[0]
  }
  const top = boxes(0, buffer.byteLength)
  const moovs = top.filter((box) => box.type === 'moov'),
    mdats = top.filter((box) => box.type === 'mdat')
  if (moovs.length !== 1 || !mdats.length || top.some((box) => box.type === 'moof')) invalid()
  const moov = moovs[0]
  if (moov.end !== buffer.byteLength || mdats.some((box) => box.end > moov.start)) invalid()
  const tracks = boxes(moov.start + moov.header, moov.end).filter((box) => box.type === 'trak')
  const video = tracks.filter((track) => {
    const mdia = child(track, 'mdia'),
      handler = child(mdia, 'hdlr')
    if (handler.end - handler.start < handler.header + 12) invalid()
    return text(handler.start + handler.header + 8) === 'vide'
  })
  if (video.length !== 1) invalid()
  const trak = video[0],
    mdia = child(trak, 'mdia'),
    minf = child(mdia, 'minf'),
    stbl = child(minf, 'stbl'),
    stsd = child(stbl, 'stsd')
  if (stsd.end - stsd.start < stsd.header + 8 || view.getUint32(stsd.start + stsd.header + 4) !== 1)
    invalid()
  const entries = boxes(stsd.start + stsd.header + 8, stsd.end)
  const entry = entries[0]
  if (
    entries.length !== 1 ||
    !['avc1', 'avc3'].includes(entry.type) ||
    entry.end - entry.start < entry.header + 78
  )
    invalid()
  const extras = boxes(entry.start + entry.header + 78, entry.end),
    existing = extras.filter((box) => box.type === 'pasp')
  if (existing.length > 1) invalid()
  if (existing.length) {
    const pasp = existing[0]
    if (pasp.end - pasp.start !== pasp.header + 8) invalid()
    const width = view.getUint32(pasp.start + pasp.header),
      height = view.getUint32(pasp.start + pasp.header + 4)
    if (!width || width !== height) invalid()
    return buffer
  }
  if (buffer.byteLength + 16 > maxBytes) throw new Error('CLIP_ANALYSIS_COPY_TOO_LARGE')
  const insert = entry.end,
    output = new Uint8Array(buffer.byteLength + 16)
  output.set(bytes.subarray(0, insert))
  output.set(bytes.subarray(insert), insert + 16)
  const patched = new DataView(output.buffer)
  patched.setUint32(insert, 16)
  output.set([112, 97, 115, 112], insert + 4)
  patched.setUint32(insert + 8, 1)
  patched.setUint32(insert + 12, 1)
  for (const parent of [moov, trak, mdia, minf, stbl, stsd, entry]) {
    const size = parent.end - parent.start + 16
    if (parent.extended) patched.setBigUint64(parent.start + 8, BigInt(size))
    else patched.setUint32(parent.start, size)
  }
  return output.buffer
}
