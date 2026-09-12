export interface CompositionLimits {
  sourceChars: number
  nodes: number
  fields: number
  items: number
  cuts: number
  cues: number
  labelChars: number
  promptChars: number
  answerChars: number
  copyChars: number
  guideChars: number
  maxDurationMs: number
  autoInsetMs: number
}
/** Half-open Unicode scalar offsets, shared with Go (not UTF-16 indices). */
export interface CompositionSpan {
  start: number
  end: number
  line: number
}
export interface CompositionNode {
  name: string
  attributes: Record<string, string>
  children: CompositionNode[]
  text: string
  span: CompositionSpan
}
export type CompositionReason =
  | 'syntax'
  | 'unsafe_construct'
  | 'node_limit'
  | 'duplicate_attribute'
  | 'invalid_entity'
  | 'invalid_limits'
  | 'invalid_unicode'
  | 'source_limit'
  | 'unknown_tag'
  | 'unknown_attribute'
  | 'unknown_version'
  | 'invalid_style'
  | 'invalid_accent'
  | 'invalid_pace'
  | 'invalid_id'
  | 'duplicate_id'
  | 'field_limit'
  | 'invalid_required'
  | 'unexpected_child'
  | 'unexpected_text'
  | 'empty_group'
  | 'guide_limit'
  | 'invalid_scope'
  | 'unknown_repeat'
  | 'empty_repeat'
  | 'invalid_kind'
  | 'invalid_role'
  | 'invalid_position'
  | 'invalid_align'
  | 'invalid_basis'
  | 'invalid_interval'
  | 'binding_scope'
  | 'unknown_field'
  | 'invalid_rows'
  | 'invalid_row_role'
  | 'copy_limit'
  | 'unknown_element'
  | 'cut_limit'
  | 'answer_limit'
  | 'required_binding'
  | 'unknown_group'
  | 'item_limit'
  | 'duplicate_item'
  | 'invalid_cut'
  | 'unknown_section'
  | 'unknown_item'
  | 'duration_limit'
  | 'missing_footage'
  | 'interval_outside'
  | 'expansion_limit'
  | 'cue_limit'
export class CompositionProblem extends Error {
  constructor(
    public elementId: string,
    public line: number,
    public reason: CompositionReason,
  ) {
    super(reason)
    this.name = 'CompositionProblem'
  }
}
export interface CompositionField {
  id: string
  group: string
  label: string
  prompt: string
  required: boolean
  span: CompositionSpan
}
export interface CompositionPart {
  literal: string
  field: string
}
export interface CompositionRow {
  role: string
  parts: CompositionPart[]
}
export interface CompositionElement {
  id: string
  kind: 'fixed' | 'ai'
  role: 'caption' | 'info' | 'badge' | 'hook' | 'ending'
  style: string
  position: string
  align: string
  basis: 'whole' | 'output-start' | 'output-end' | 'cut'
  startMs: number | null
  endMs: number | null
  parts: CompositionPart[]
  rows: CompositionRow[]
  span: CompositionSpan
}
export interface CompositionSection {
  id: string
  scope: string
  repeat: string
  guidance: string[]
  elements: CompositionElement[]
  span: CompositionSpan
}
export interface ClipComposition {
  source: string
  root: CompositionNode
  styles: string[]
  accent: string
  pace: string
  fields: CompositionField[]
  groups: string[]
  guidance: string[]
  sections: CompositionSection[]
  elements: CompositionElement[]
}
export interface CompositionItem {
  id: string
  values: Record<string, string>
}
export interface CompositionCut {
  id: string
  sectionId: string
  sourceId: string
  groupId: string
  itemId: string
  startMs: number
  endMs: number
  transitionMs: number
}
export interface CompositionInputs {
  values: Record<string, string>
  items: Record<string, CompositionItem[]>
  cuts: CompositionCut[]
}
export interface CompositionFact {
  fieldId: string
  groupId: string
  itemId: string
  value: string
}
export interface ResolvedCompositionElement {
  instanceId: string
  cutId: string
  groupId: string
  itemId: string
  element: CompositionElement
  /** Fixed output text, or AI guidance when element.kind is ai. */
  text: string
  rows: { role: string; text: string }[]
  facts: CompositionFact[]
  startMs: number
  endMs: number
  authoredTiming: boolean
}
export interface CompositionTimeline {
  durationMs: number
  elements: ResolvedCompositionElement[]
}
