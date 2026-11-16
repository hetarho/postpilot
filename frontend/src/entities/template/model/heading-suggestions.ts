export interface HeadingSuggestionInput {
  writeModel: { providerId: string; modelId: string }
  targetKind: 'post' | 'clip'
  targetPath: readonly string[]
  sections: readonly { role: string; description: string }[]
  /** How the owner wants the headings worded (TMPL-73); it outranks the 기본 voice. */
  styleRequest?: string
  /** Heading lines of the owner's saved templates (TMPL-76): wording below the request and
   *  above the 기본 voice. */
  referenceHeadings?: readonly string[]
}
