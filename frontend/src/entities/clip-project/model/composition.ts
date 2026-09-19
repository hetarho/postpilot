import type { ClipSourceAssociation } from '@/entities/clip-design/@x/clip-project'

export type { ClipSourceAssociation }

export interface ClipCompositionInputs {
  values: Record<string, string>
  items: Record<string, Array<{ id: string; values: Record<string, string> }>>
  associations: ClipSourceAssociation[]
}
export interface ClipProjectComposition {
  snapshot: { version: number; body: string; templateId: string; legacy: boolean }
  inputs: ClipCompositionInputs
}
