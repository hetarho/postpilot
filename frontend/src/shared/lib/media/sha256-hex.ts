/** SHA-256 of `data` as lowercase hex, two characters per byte. */
export async function sha256Hex(data: BufferSource): Promise<string> {
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', data))
  return Array.from(digest, (byte) => byte.toString(16).padStart(2, '0')).join('')
}
