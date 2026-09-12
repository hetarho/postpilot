export {
  useClipTemplates,
  useClipTemplateMutations,
  clipTemplatesKey,
  toClipTemplate,
} from './api/clip-template'
export {
  CLIP_TEMPLATE_LIMITS,
  COPY_STYLES,
  CLIP_ACCENTS,
  copyStyleMeasurements,
  PLATED_COPY_STYLES,
  CLIP_PRESETS_LIST,
  emptyClipRecipe,
  normalizeRecipe,
  recipeOf,
  validateClipRecipe,
} from './model/types'
export type {
  ClipTemplate,
  ClipRecipe,
  InformationField,
  CopyStyle,
  ClipAccent,
  FieldError,
} from './model/types'
export { CopyStylePreview } from './ui/CopyStylePreview'
export {
  parseClipComposition,
  replaceCompositionNode,
  replaceCompositionSpan,
  compositionMilliseconds,
} from './lib/composition-parse'
export { serializeCompositionNode } from './lib/composition-xml'
export { resolveClipComposition } from './lib/composition-resolve'
export { CompositionProblem } from './model/composition'
export type {
  ClipComposition,
  CompositionNode,
  CompositionSpan,
  CompositionElement,
  CompositionSection,
  CompositionField,
  CompositionPart,
  CompositionRow,
  CompositionInputs,
  CompositionItem,
  CompositionCut,
  CompositionFact,
  CompositionTimeline,
  ResolvedCompositionElement,
  CompositionLimits,
} from './model/composition'
