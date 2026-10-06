/** Keep provider source and compiled payloads off the conversational surface. */
export function readableAuthoringProse(text: string): boolean {
  if (/<\/?[a-z][^>]*>/i.test(text)) return false
  const first = text.trim()[0]
  if (first === '[' || first === '{') {
    try {
      JSON.parse(text)
      return false
    } catch {
      // Bracketed prose remains ordinary readable text.
    }
  }
  return true
}
