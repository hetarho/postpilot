/** Original EXIF only, before canvas re-encoding discards it. Never read photo pixels. */
const MAX_EXIF_BYTES = 128 * 1024

function exifInstant(date: string, offset: string): string | undefined {
  const parts = /^(\d{4}):(\d{2}):(\d{2}) (\d{2}):(\d{2}):(\d{2})$/u.exec(date)
  const zone = /^([+-])(\d{2}):(\d{2})$/u.exec(offset)
  if (!parts || !zone) return
  const [year, month, day, hour, minute, second] = parts.slice(1).map(Number) as [
    number,
    number,
    number,
    number,
    number,
    number,
  ]
  const zoneHour = Number(zone[2]),
    zoneMinute = Number(zone[3])
  if (
    year < 1 ||
    month < 1 ||
    month > 12 ||
    day < 1 ||
    day > 31 ||
    hour > 23 ||
    minute > 59 ||
    second > 59 ||
    zoneHour > 23 ||
    zoneMinute > 59
  )
    return
  const clock = new Date(0)
  clock.setUTCFullYear(year, month - 1, day)
  clock.setUTCHours(hour, minute, second, 0)
  if (
    clock.getUTCFullYear() !== year ||
    clock.getUTCMonth() !== month - 1 ||
    clock.getUTCDate() !== day
  )
    return
  const signedMinutes = (zoneHour * 60 + zoneMinute) * (zone[1] === '+' ? 1 : -1)
  const instant = new Date(clock.getTime() - signedMinutes * 60_000)
  if (instant.getUTCFullYear() < 1 || instant.getUTCFullYear() > 9999) return
  return instant.toISOString()
}

function readExif(view: DataView, start: number, end: number): string | undefined {
  if (start + 8 > end) return
  const order = view.getUint16(start)
  if (order !== 0x4949 && order !== 0x4d4d) return
  const little = order === 0x4949
  if (view.getUint16(start + 2, little) !== 42) return
  const entries = (relative: number) => {
    const at = start + relative
    if (relative < 8 || at + 2 > end) return
    const count = view.getUint16(at, little)
    if (at + 2 + count * 12 > end) return
    return { at: at + 2, count }
  }
  const first = entries(view.getUint32(start + 4, little))
  if (!first) return
  let exif = 0
  for (let i = 0; i < first.count; i++) {
    const at = first.at + i * 12
    if (
      view.getUint16(at, little) === 0x8769 &&
      view.getUint16(at + 2, little) === 4 &&
      view.getUint32(at + 4, little) === 1
    ) {
      exif = view.getUint32(at + 8, little)
      break
    }
  }
  const fields = entries(exif)
  if (!fields) return
  let date = '',
    offset = ''
  for (let i = 0; i < fields.count; i++) {
    const at = fields.at + i * 12
    const tag = view.getUint16(at, little)
    if ((tag !== 0x9003 && tag !== 0x9011) || view.getUint16(at + 2, little) !== 2) continue
    const count = view.getUint32(at + 4, little)
    if (count < 1 || count > 32) continue
    const valueAt = count <= 4 ? at + 8 : start + view.getUint32(at + 8, little)
    if (valueAt < start + 8 || valueAt + count > end) continue
    let text = ''
    for (let j = 0; j < count; j++) {
      const byte = view.getUint8(valueAt + j)
      if (byte === 0) break
      if (byte > 127) {
        text = ''
        break
      }
      text += String.fromCharCode(byte)
    }
    if (tag === 0x9003) date = text
    else offset = text
  }
  // A wall clock alone has no known UTC instant. Do not invent the browser's zone.
  return exifInstant(date, offset)
}

export async function readJpegCaptureTime(file: Blob): Promise<string | undefined> {
  try {
    const view = new DataView(await file.slice(0, MAX_EXIF_BYTES).arrayBuffer())
    if (view.byteLength < 4 || view.getUint16(0) !== 0xffd8) return
    let at = 2
    while (at + 4 <= view.byteLength) {
      if (view.getUint8(at++) !== 0xff) return
      while (at < view.byteLength && view.getUint8(at) === 0xff) at++
      if (at >= view.byteLength) return
      const marker = view.getUint8(at++)
      if (marker === 0xda || marker === 0xd9) return
      if (marker === 0x01 || (marker >= 0xd0 && marker <= 0xd8)) continue
      if (at + 2 > view.byteLength) return
      const size = view.getUint16(at)
      if (size < 2 || at + size > view.byteLength) return
      const body = at + 2
      if (
        marker === 0xe1 &&
        size >= 16 &&
        view.getUint32(body) === 0x45786966 &&
        view.getUint16(body + 4) === 0
      ) {
        const captured = readExif(view, body + 6, at + size)
        if (captured) return captured
      }
      at += size
    }
  } catch {
    // Capture metadata is optional; bad tags never turn a readable photo into a failed upload.
  }
}
