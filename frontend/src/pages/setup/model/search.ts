export function setupSearchSchema(search: Record<string, unknown>) {
  return { restart: search.restart === true || search.restart === 'true' }
}
