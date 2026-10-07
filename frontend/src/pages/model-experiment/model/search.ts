import { safeInternalPath } from '@/shared/lib/navigation'

/** Origins are bounded navigation contexts, never an arbitrary return URL. */
export const modelReviewSearchSchema = (search: Record<string, unknown>): { from?: 'compare' } => ({
  from: search.from === 'compare' ? 'compare' : undefined,
})

export const postReviewSearchSchema = (search: Record<string, unknown>): { from?: 'posts' } => ({
  from: search.from === 'posts' ? 'posts' : undefined,
})

export function testRecordSearchSchema(search: Record<string, unknown>): {
  entry?: string
  source?: string
} {
  return {
    entry:
      typeof search.entry === 'string' && safeInternalPath(search.entry) ? search.entry : undefined,
    source: typeof search.source === 'string' && search.source ? search.source : undefined,
  }
}
