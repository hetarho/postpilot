import {
  CLIP_PRESETS,
  CLIP_SPACING,
  CLIP_STYLES,
  clipStyle,
  clipType,
  type ClipPresetId,
  type ClipStyleId,
} from '@/shared/config'

export const CLIP_TEMPLATE_LIMITS = {
  name: 40,
  guidance: 4000,
  fields: 10,
  label: 40,
  prompt: 200,
} as const

/** The four CDS copy styles, one per role, and the five category presets — both
 *  read from the design system rather than listed again here. */
export const COPY_STYLES = CLIP_STYLES
export type CopyStyle = ClipStyleId
export const CLIP_PRESETS_LIST = CLIP_PRESETS
export type ClipPreset = ClipPresetId
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
  /** One of the five category presets. Empty is only ever a template written
   *  before presets existed; a save must name one (CDS-50). */
  preset: ClipPreset | ''
}
export interface ClipTemplate extends ClipRecipe {
  id: string
  projectCount: number
  createdAt: string
  updatedAt: string
}

/** What the preview needs to draw a style, taken from the design system: the
 *  renderer reads the same bytes, so a preview cannot drift from a render. */
export function copyStyleMeasurements(style: CopyStyle) {
  const rule = clipStyle(style)
  const role = clipType(style)
  return {
    fontSize: role.size,
    minFontSize: role.min,
    weight: role.weight,
    tracking: role.tracking,
    padding: rule.padding,
    padLeft: rule.pad_left,
    radius: rule.plate === '' ? 0 : CLIP_SPACING.radius_box,
    plate: rule.plate,
    bar: rule.bar,
    dot: rule.dot,
    stroke:
      rule.stroke === 'mark'
        ? CLIP_SPACING.stroke_mark
        : rule.stroke === ''
          ? 0
          : CLIP_SPACING.stroke_text,
    shadow: rule.shadow !== '',
    highlight: rule.highlight,
    lines: rule.lines,
    chars: rule.chars,
    anchor: rule.anchor,
    align: rule.align,
  }
}

/** An unplated style paints its text with a stroke instead of a box (CDS-25, CDS-26). */
export const PLATED_COPY_STYLES: readonly CopyStyle[] = COPY_STYLES.filter(
  (style) => clipStyle(style).plate !== '',
)

export function emptyClipRecipe(): ClipRecipe {
  return {
    name: '',
    informationFields: [],
    cutGuidance: '',
    copyStyles: ['clean'],
    accent: '',
    preset: '',
  }
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
    preset: value.preset,
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
    // A template names its category, which fixes chip priority, CTA and accent.
    preset: !CLIP_PRESETS.includes(recipe.preset as ClipPresetId),
  }
  return {
    ...errors,
    valid:
      !errors.name &&
      !errors.guidance &&
      !errors.fieldCount &&
      !errors.styles &&
      !errors.accent &&
      !errors.preset &&
      fields.every((f) => !f.label && !f.prompt),
  }
}
