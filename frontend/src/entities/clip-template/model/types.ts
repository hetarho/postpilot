import { parseClipTemplate } from '../lib/composition-parse'
export const CLIP_TEMPLATE_LIMITS = {
  name: 40,
} as const

export const CLIP_ACCENTS = [
  '',
  'coral',
  'amber',
  'lime',
  'teal',
  'blue',
  'violet',
  'pink',
] as const
export type ClipAccent = (typeof CLIP_ACCENTS)[number]
/** A template is an outline body under a name (CLIP-14); nothing else is stored. */
export interface ClipRecipe {
  name: string
  compositionBody: string
}
export interface ClipTemplate extends ClipRecipe {
  id: string
  projectCount: number
  createdAt: string
  updatedAt: string
}

export function emptyClipRecipe(): ClipRecipe {
  return { name: '', compositionBody: '' }
}
export function normalizeRecipe(value: ClipRecipe): ClipRecipe {
  return { ...value, name: value.name.trim() }
}
export function recipeOf(value: ClipRecipe): ClipRecipe {
  return { name: value.name, compositionBody: value.compositionBody }
}
export type FieldError = 'required' | 'tooLong' | 'duplicate' | 'invalid'
export function validateClipRecipe(value: ClipRecipe) {
  const recipe = normalizeRecipe(value)
  const name: FieldError | undefined = !recipe.name
    ? 'required'
    : Array.from(recipe.name).length > CLIP_TEMPLATE_LIMITS.name
      ? 'tooLong'
      : undefined
  let composition: 'invalid' | undefined
  try {
    parseClipTemplate(recipe.compositionBody)
  } catch {
    composition = 'invalid'
  }
  return { name, composition, valid: !name && !composition }
}
