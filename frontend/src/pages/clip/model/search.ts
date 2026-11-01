import { CLIP_CHANNELS, type ClipChannel } from '@/entities/clip-project'

/** `/clips/new`'s search. `from=setup`: first-use setup's finish opened this creation, so it
 *  offers nothing the owner just answered (CLIP-223). */
export function clipCreationSearchSchema(search: Record<string, unknown>): {
  channel?: ClipChannel
  from?: 'setup'
} {
  return {
    channel: CLIP_CHANNELS.includes(search.channel as ClipChannel)
      ? (search.channel as ClipChannel)
      : undefined,
    from: search.from === 'setup' ? 'setup' : undefined,
  }
}
