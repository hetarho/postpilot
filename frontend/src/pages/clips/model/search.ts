import {
  CLIP_CHANNELS,
  clipHistoryFilter,
  type ClipChannel,
  type ClipHistoryFilter,
} from '@/entities/clip-project'

/** The narrowing the directory is showing, as an address (CLIP-41): opening a project and coming
 *  back, a reload and a shared link all keep it. Anything the directory cannot honour is dropped
 *  rather than refused — a hand-edited URL should show the directory, not an error. */
export const searchSchema = (
  search: Record<string, unknown>,
): { q?: string; status?: ClipHistoryFilter; channel?: ClipChannel } => ({
  channel: CLIP_CHANNELS.includes(search.channel as ClipChannel)
    ? (search.channel as ClipChannel)
    : undefined,
  q: typeof search.q === 'string' && search.q !== '' ? search.q : undefined,
  // A link saved before 완료 replaced 완성 still opens the finished clips (CLIP-226).
  status: clipHistoryFilter(search.status),
})
