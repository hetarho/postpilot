/** Private builder metadata assists recovery; canonical template source remains authoritative. */
export function readBuilderMetadata(raw: string | undefined): Record<string, unknown> {
  try {
    const value: unknown = JSON.parse(raw || '{}')
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      return value as Record<string, unknown>
    }
  } catch {
    // Invalid metadata never prevents the owner from editing the raw source.
  }
  return {}
}
