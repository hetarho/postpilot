import { clipChannel, type ClipChannel } from '@/entities/clip-project'

/** `clip=new`: a new clip's offer opened this wizard, so 저장 goes back to a new clip in that
 *  channel with the template selected (CLIP-223). */
export const videoTemplateSearchSchema = (
  search: Record<string, unknown>,
): { session?: string; channel?: ClipChannel; clip?: 'new' } => ({
  channel: search.channel === undefined ? undefined : clipChannel(search.channel),
  session:
    typeof search.session === 'string' && search.session.trim() !== '' ? search.session : undefined,
  clip: search.clip === 'new' ? 'new' : undefined,
})
