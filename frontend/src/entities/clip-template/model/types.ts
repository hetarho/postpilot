import {
  CLIP_DEFAULT_REGION_PRESETS,
  type ClipRegionPresets,
} from '@/entities/clip-design/@x/clip-template'
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
/** The design a template starts a project in (CLIP-166): chosen in its preview, saved with it,
 *  and taken by a project when the template is selected (CLIP-168). The same ids a project's
 *  selection uses; an empty style list is a selection of none. */
export interface ClipTemplateDesign {
  introPreset: ClipRegionPresets['intro']
  outroPreset: ClipRegionPresets['outro']
  allowedCaptionStyles: string[]
}
/** A template is an outline body under a name with its starting design (CLIP-14, CLIP-166). */
export interface ClipRecipe extends ClipTemplateDesign {
  name: string
  compositionBody: string
}
export interface ClipTemplate extends ClipRecipe {
  id: string
  projectCount: number
  createdAt: string
  updatedAt: string
}

/** A new template starts at a new project's design: the shared presets and no styles. */
export function emptyClipRecipe(): ClipRecipe {
  return {
    name: '',
    compositionBody: '',
    introPreset: CLIP_DEFAULT_REGION_PRESETS.intro,
    outroPreset: CLIP_DEFAULT_REGION_PRESETS.outro,
    allowedCaptionStyles: [],
  }
}
export function normalizeRecipe(value: ClipRecipe): ClipRecipe {
  return { ...recipeOf(value), name: value.name.trim() }
}
export function recipeOf(value: ClipRecipe): ClipRecipe {
  return {
    name: value.name,
    compositionBody: value.compositionBody,
    introPreset: value.introPreset,
    outroPreset: value.outroPreset,
    allowedCaptionStyles: [...value.allowedCaptionStyles],
  }
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
