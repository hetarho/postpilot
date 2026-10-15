export const CLIP_CHANNELS = ['naver_clip', 'instagram_clip'] as const
export type ClipChannel = (typeof CLIP_CHANNELS)[number]
export const DEFAULT_CLIP_CHANNEL: ClipChannel = 'naver_clip'

/** Unknown hand-edited URL values open the default channel. */
export function clipChannel(value: unknown): ClipChannel {
  return value === 'instagram_clip' ? value : DEFAULT_CLIP_CHANNEL
}
