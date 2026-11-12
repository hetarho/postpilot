import { clipChannel, type ClipChannel } from '@/entities/clip-project'

/** `clip=new`: a new clip's offer opened this wizard, so 저장 goes back to a new clip in that
 *  channel with the template selected (CLIP-223). */
export const videoTemplateSearchSchema = (
  search: Record<string, unknown>,
): { session?: string; channel?: ClipChannel; clip?: string } => ({
  channel: search.channel === undefined ? undefined : clipChannel(search.channel),
  session:
    typeof search.session === 'string' && search.session.trim() !== '' ? search.session : undefined,
  // `new`: a new clip's offer opened the wizard (CLIP-223); an id: a saved clip's 템플릿 screen
  // did, and 저장 returns there (CLIP-225).
  clip:
    typeof search.clip === 'string' && /^[\w-]{1,64}$/.test(search.clip) ? search.clip : undefined,
})
