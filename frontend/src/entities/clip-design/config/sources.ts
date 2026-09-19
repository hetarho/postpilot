/** T071 admission mirrors. The API and media probe remain authoritative; these
 *  are what lets the picker refuse a file before it is read. */
export const CLIP_SOURCE_MAX_COUNT = 20
export const CLIP_SOURCE_MAX_FILENAME_CHARS = 255
export const CLIP_SOURCE_MAX_DURATION_MS = 30 * 60 * 1000
export const CLIP_SOURCE_MAX_FILE_BYTES = 2 * 1024 * 1024 * 1024
export const CLIP_SOURCE_MAX_BATCH_BYTES = 8 * 1024 * 1024 * 1024
export const CLIP_SOURCE_FINGERPRINT_CHUNK_BYTES = 64 * 1024
export const CLIP_SOURCE_CONTAINERS: Readonly<Record<string, readonly string[]>> = {
  mp4: ['video/mp4'],
  mov: ['video/quicktime'],
  m4v: ['video/x-m4v', 'video/mp4'],
  webm: ['video/webm'],
}
