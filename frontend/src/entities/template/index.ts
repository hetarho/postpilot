export * from './config'
export type { Template, TemplateRef } from './model/types'
export {
  noTemplateLabel,
  NO_TEMPLATE_VALUE,
  TEMPLATE_LIMITS,
  TEMPLATE_PARSE_OPTIONS,
  canSaveTemplate,
  detachWarning,
  emptyTemplateRef,
  templateChars,
  remainingChars,
} from './model/types'
export { templateDirectoryQuery, useTemplates } from './api/useTemplates'
export { templatesQueryKey, toTemplate, toTemplateRef } from './api/template-queries'
export { invalidateTemplates } from './api/template-cache'
export { useCreateTemplate } from './api/useCreateTemplate'
export { useDeleteTemplate } from './api/useDeleteTemplate'
export { useUpdateTemplate } from './api/useUpdateTemplate'
export { templateErrorMessage } from './api/template-errors'
export { TemplateRefLabel } from './ui/TemplateRefLabel'
export {
  askFields,
  decode,
  encode,
  isBlank,
  parse,
  parseTemplate,
  serialize,
  templateAskFields,
  PARSE_REASONS,
  type ParseFailure,
  type ParseOptions,
  type ParseReason,
  type AskField,
  type SlotKind,
  type TemplateArea,
  type TemplateNode,
} from './lib/grammar'
export { useFormatGuide } from './api/useFormatGuide'
export {
  useCancelTemplateRequest,
  useStartTemplateRequest,
  useTemplateRequestEstimate,
  useTemplateRequestResult,
  type TemplateDraftTexts,
  type TemplateRequestInput,
  type TemplateRequestModel,
  type TemplateRequestResult,
} from './api/useTemplateRequest'
export { TemplateComposition } from './ui/TemplateComposition'
export { TemplateSource } from './ui/TemplateSource'
export { TemplatePreview } from './ui/TemplatePreview'
export {
  AnswerChip,
  AnswerStandIn,
  FixedTextStandIn,
  PhotoStandIn,
  PREVIEW_REPEAT_PASSES,
  PreviewLines,
  PreviewPostCard,
  RepeatStandIn,
  WriteStandIn,
} from './ui/PreviewStandIns'
export {
  newBlock,
  readCompositionWorkingState,
  toBody,
  toValidBody,
  toWorkingBody,
  type BuilderBlock,
  type CompositionWorkingState,
} from './model/blocks'
export type { HeadingSuggestionInput } from './model/heading-suggestions'
export {
  HEADING_SUGGESTION_SECTIONS_MAX,
  HEADING_CHOICES_PER_SECTION,
  HEADING_STYLE_REQUEST_MAX_CHARS,
} from './config'
export {
  useStartHeadingSuggestions,
  useHeadingSuggestionsEstimate,
  useHeadingSuggestionsResult,
} from './api/useHeadingSuggestions'

export { templateTargetsI18n } from './config/target-i18n'
export {
  nodeAt as templateTargetAt,
  ROLES as TEMPLATE_ROLE_DEFAULTS,
  TARGET_TREE as TEMPLATE_TARGET_TREE,
  leaves as templateTargetLeaves,
} from './config/target-catalog'
export type { RoleKey, TargetNode, SectionContent } from './config/target-catalog'
export { useWizardHeadingSuggestions } from './api/useWizardHeadingSuggestions'
export type { WizardHeadingSuggestionsOperation } from './api/useWizardHeadingSuggestions'

export { WizardHeadingSuggestions, WizardHeadingChoices } from './ui/WizardHeadingSuggestions'
