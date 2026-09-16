export {
  useClipTemplates,
  useClipTemplateMutations,
  clipTemplatesKey,
  toClipTemplate,
} from './api/clip-template'
export {
  CLIP_TEMPLATE_LIMITS,
  CLIP_ACCENTS,
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
  ClipAccent,
  FieldError,
} from './model/types'
export { CompositionBuilder } from './ui/CompositionBuilder'
export { CompositionPreview } from './ui/CompositionPreview'
export { clipCompositionGuide, CLIP_COMPOSITION_EXAMPLE } from './model/composition-guide'
export {
  parseClipComposition,
  parseClipTemplate,
  readStoredClipComposition,
  replaceCompositionNode,
  replaceCompositionSpan,
  compositionMilliseconds,
  compositionCharacters,
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

export {
  compositionSkeleton,
  compositionDesign,
  rebuildCompositionSkeleton,
} from './lib/composition-skeleton'
export type { CompositionDesign } from './lib/composition-skeleton'
export { CompositionDesignStep } from './ui/CompositionDesignStep'
