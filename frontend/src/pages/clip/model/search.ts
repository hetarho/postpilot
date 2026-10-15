import { CLIP_CHANNELS, type ClipChannel } from '@/entities/clip-project'

export function clipCreationSearchSchema(search: Record<string, unknown>): {
  channel?: ClipChannel
} {
  return {
    channel: CLIP_CHANNELS.includes(search.channel as ClipChannel)
      ? (search.channel as ClipChannel)
      : undefined,
  }
}
