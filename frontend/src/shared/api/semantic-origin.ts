import {
  OriginFieldKind,
  OriginReviewState,
  SemanticOriginCategory,
} from './gen/postpilot/v1/semantic_origin_pb'

export const semanticOriginCategoryNames = {
  [SemanticOriginCategory.UNSPECIFIED]: undefined,
  [SemanticOriginCategory.OWNER_INPUT]: 'owner_input',
  [SemanticOriginCategory.PHOTO_INTERPRETATION]: 'photo_interpretation',
  [SemanticOriginCategory.AI_ADDED]: 'ai_added',
} as const

export const originReviewStateNames = {
  [OriginReviewState.UNSPECIFIED]: undefined,
  [OriginReviewState.UNCONFIRMED]: 'unconfirmed',
  [OriginReviewState.UNREVIEWED]: 'unreviewed',
  [OriginReviewState.CONFIRMED]: 'confirmed',
} as const

export const originFieldKindNames = {
  [OriginFieldKind.UNSPECIFIED]: undefined,
  [OriginFieldKind.TITLE]: 'title',
  [OriginFieldKind.SUMMARY]: 'summary',
  [OriginFieldKind.TAG]: 'tag',
  [OriginFieldKind.BLOCK_CONTENT]: 'block_content',
  [OriginFieldKind.BLOCK_ITEM]: 'block_item',
  [OriginFieldKind.BLOCK_ALT]: 'block_alt',
  [OriginFieldKind.BLOCK_CAPTION]: 'block_caption',
} as const

export function semanticOriginCategoryFromProto(value: SemanticOriginCategory) {
  return semanticOriginCategoryNames[value]
}

export function originReviewStateFromProto(value: OriginReviewState) {
  return originReviewStateNames[value]
}

export function originFieldKindFromProto(value: OriginFieldKind) {
  return originFieldKindNames[value]
}
