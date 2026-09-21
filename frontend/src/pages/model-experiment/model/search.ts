/** Origins are bounded navigation contexts, never an arbitrary return URL. */
export const modelReviewSearchSchema = (search: Record<string, unknown>): { from?: 'compare' } => ({
  from: search.from === 'compare' ? 'compare' : undefined,
})

export const postReviewSearchSchema = (search: Record<string, unknown>): { from?: 'posts' } => ({
  from: search.from === 'posts' ? 'posts' : undefined,
})
