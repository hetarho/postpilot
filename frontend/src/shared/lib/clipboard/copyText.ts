export type CopyFallbackElement = HTMLInputElement | HTMLTextAreaElement

/** Copy exact text when allowed, otherwise leave the fallback field fully selected. */
export async function copyText(
  text: string,
  fallbackElement?: CopyFallbackElement | null,
  canFallback: () => boolean = () => true,
): Promise<{ copied: boolean }> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return { copied: true }
    } catch {
      // Non-secure contexts and browser policy can reject an otherwise present API.
    }
  }

  if (canFallback()) {
    fallbackElement?.focus()
    fallbackElement?.select()
  }
  return { copied: false }
}
