/** An explicit request to open the existing voice's learning chat; never creates or analyzes it. */
export function voiceMaterialsSearchSchema(search: Record<string, unknown>): { learn?: 'chat' } {
  return search.learn === 'chat' ? { learn: 'chat' } : {}
}
