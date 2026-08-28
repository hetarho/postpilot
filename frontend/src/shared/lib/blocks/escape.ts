/** Escape text placed in HTML text or attribute contexts. */
export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

/** JSON strings are valid YAML double-quoted scalars and cover every required escape. */
export function yamlString(value: string): string {
  return JSON.stringify(value)
}

/** One uploaded basename as a relative URL path segment (never a presigned URL). */
export function relativeFileUrl(filename: string): string {
  return encodeURIComponent(filename).replace(
    /[!'()*]/g,
    (character) => `%${character.charCodeAt(0).toString(16).toUpperCase()}`,
  )
}

/** Keep an HTML comment open even when a user filename contains the closing delimiter. */
export function escapeHtmlComment(value: string): string {
  return value.replaceAll('--', '-\u200b-').replaceAll('\u0000', '\ufffd')
}

/** Escape the delimiters that can terminate a Markdown image label. */
export function escapeMarkdownLabel(value: string): string {
  return escapeHtml(value).replaceAll('\\', '\\\\').replaceAll('[', '\\[').replaceAll(']', '\\]')
}
