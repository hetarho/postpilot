export interface SpokenCreationSearch {
  draft?: string
  operation?: string
  qualification?: string
  copy?: string
}
export function spokenCreationSearchSchema(raw: Record<string, unknown>): SpokenCreationSearch {
  const result: SpokenCreationSearch = {}
  for (const key of ['draft', 'operation', 'qualification', 'copy'] as const) {
    const value = raw[key]
    if (typeof value === 'string' && /^[a-f0-9]{32}$/.test(value)) result[key] = value
  }
  return result
}
