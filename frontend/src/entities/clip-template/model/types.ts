export const CLIP_TEMPLATE_LIMITS = {
  name: 40,
  guidance: 4000,
  fields: 10,
  label: 40,
  prompt: 200,
} as const

/** The four CDS copy styles, one per role: clean 깔끔하게 narrative, memo 메모 fact,
 *  bold 크게 강조 emotion and hook, mark 형광펜 one number or keyword. */
export const COPY_STYLES = ['clean', 'memo', 'bold', 'mark'] as const
export type CopyStyle = (typeof COPY_STYLES)[number]
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
export interface InformationField {
  label: string
  prompt: string
}
export interface ClipRecipe {
  name: string
  informationFields: InformationField[]
  cutGuidance: string
  copyStyles: CopyStyle[]
  accent: ClipAccent
}
export interface ClipTemplate extends ClipRecipe {
  id: string
  projectCount: number
  createdAt: string
  updatedAt: string
}

// Canvas-space measurements mirrored by the deterministic renderer (T074). The
// authoritative type scale now lives in shared/config/clip-design.json; T103
// takes the preview and the renderer onto it when it draws the four styles.
export const COPY_STYLE_MEASUREMENTS = {
  clean: { fontSize: 54, minFontSize: 36, weight: 600, padding: 28, radius: 24 },
  memo: { fontSize: 44, minFontSize: 32, weight: 600, padding: 22, radius: 16 },
  bold: { fontSize: 76, minFontSize: 48, weight: 800, padding: 24, radius: 0 },
  mark: { fontSize: 60, minFontSize: 52, weight: 800, padding: 24, radius: 0 },
} as const

/** An unplated style paints its text with a stroke instead of a box (CDS-25, CDS-26). */
export const PLATED_COPY_STYLES: readonly CopyStyle[] = ['clean', 'memo']

export function emptyClipRecipe(): ClipRecipe {
  return { name: '', informationFields: [], cutGuidance: '', copyStyles: ['clean'], accent: '' }
}
export function normalizeRecipe(value: ClipRecipe): ClipRecipe {
  return {
    ...value,
    name: value.name.trim(),
    informationFields: value.informationFields.map((field) => ({
      label: field.label.trim(),
      prompt: field.prompt.trim(),
    })),
  }
}
export function recipeOf(value: ClipRecipe): ClipRecipe {
  return {
    name: value.name,
    informationFields: value.informationFields.map((f) => ({ ...f })),
    cutGuidance: value.cutGuidance,
    copyStyles: [...value.copyStyles],
    accent: value.accent,
  }
}
export type FieldError = 'required' | 'tooLong' | 'duplicate' | 'invalid'
export function validateClipRecipe(value: ClipRecipe) {
  const recipe = normalizeRecipe(value)
  const length = (text: string) => Array.from(text).length
  const textError = (text: string, max: number, required = true): FieldError | undefined =>
    required && !text ? 'required' : length(text) > max ? 'tooLong' : undefined
  const labels = recipe.informationFields.map((f) => f.label)
  const fields = recipe.informationFields.map((f) => ({
    label:
      textError(f.label, CLIP_TEMPLATE_LIMITS.label) ??
      (labels.filter((v) => v === f.label).length > 1 ? 'duplicate' : undefined),
    prompt: textError(f.prompt, CLIP_TEMPLATE_LIMITS.prompt),
  }))
  const errors = {
    name: textError(recipe.name, CLIP_TEMPLATE_LIMITS.name),
    guidance: textError(recipe.cutGuidance, CLIP_TEMPLATE_LIMITS.guidance, false),
    fields,
    fieldCount: recipe.informationFields.length > CLIP_TEMPLATE_LIMITS.fields,
    styles:
      recipe.copyStyles.length === 0 ||
      // Every CDS fallback lands on 깔끔하게, so an approved set without it
      // cannot render; the server refuses one too.
      !recipe.copyStyles.includes('clean') ||
      new Set(recipe.copyStyles).size !== recipe.copyStyles.length ||
      recipe.copyStyles.some((v) => !COPY_STYLES.includes(v)),
    accent: !CLIP_ACCENTS.includes(recipe.accent),
  }
  return {
    ...errors,
    valid:
      !errors.name &&
      !errors.guidance &&
      !errors.fieldCount &&
      !errors.styles &&
      !errors.accent &&
      fields.every((f) => !f.label && !f.prompt),
  }
}
