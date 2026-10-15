export interface HeadingSuggestionInput {
  writeModel: { providerId: string; modelId: string }
  targetKind: 'post' | 'clip'
  targetPath: readonly string[]
  sections: readonly { role: string; description: string }[]
}
