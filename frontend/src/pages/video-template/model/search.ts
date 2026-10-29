import { clipChannel, type ClipChannel } from '@/entities/clip-project'

export const videoTemplateSearchSchema = (
  search: Record<string, unknown>,
): { session?: string; channel?: ClipChannel } => ({
  channel: search.channel === undefined ? undefined : clipChannel(search.channel),
  session:
    typeof search.session === 'string' && search.session.trim() !== '' ? search.session : undefined,
})
