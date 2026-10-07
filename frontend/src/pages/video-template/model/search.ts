export const videoTemplateSearchSchema = (
  search: Record<string, unknown>,
): { session?: string } => ({
  session:
    typeof search.session === 'string' && search.session.trim() !== '' ? search.session : undefined,
})
