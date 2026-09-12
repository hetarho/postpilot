export interface ClipSourceAssociation {
  groupId: string
  itemId: string
  sourceId: string
  fingerprint: string
  startMs: number
  endMs: number
}
export interface ClipCompositionInputs {
  values: Record<string, string>
  items: Record<string, Array<{ id: string; values: Record<string, string> }>>
  associations: ClipSourceAssociation[]
}
export interface ClipProjectComposition {
  snapshot: { version: number; body: string; templateId: string; legacy: boolean }
  inputs: ClipCompositionInputs
}
