import { CLIP_CHANNELS, type ClipChannel } from '@/entities/clip-project'

/** `/clips/new`'s search. `template=<id>`: the video template the offer's wizard just saved, to
 *  start the clip with. `from=setup`: first-use setup's finish opened this creation. Either way
 *  the creation offers nothing the owner just answered (CLIP-223). */
export function clipCreationSearchSchema(search: Record<string, unknown>): {
  channel?: ClipChannel
  template?: string
  from?: 'setup'
} {
  return {
    channel: CLIP_CHANNELS.includes(search.channel as ClipChannel)
      ? (search.channel as ClipChannel)
      : undefined,
    template:
      typeof search.template === 'string' && search.template.trim() !== ''
        ? search.template
        : undefined,
    from: search.from === 'setup' ? 'setup' : undefined,
  }
}
