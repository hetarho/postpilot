/** The clip design system (CDS).
 *
 *  `clip-design.json` is byte-identical to the file the renderer embeds — a Go
 *  test compares the two — so every number the preview draws and every limit the
 *  validators check is the same number the render uses. Nothing here is typed by
 *  hand; changing a value means editing that file, on both sides at once. */
import design from './clip-design.json'

export const CLIP_DESIGN = design

export type ClipRatioId = 'vertical' | 'horizontal' | 'square'
export type ClipDisclosureId = keyof typeof design.disclosure

export const CLIP_DISCLOSURES = Object.keys(design.disclosure) as ClipDisclosureId[]

export const CLIP_REGIONS = design.regions
export const CLIP_RULES = design.rule
/** The default caption treatment's id (CDS-25). A project that selects no styles
 *  may use this one alone, so it is also what an unset style means (CLIP-142). */
export const CLIP_DEFAULT_CAPTION_STYLE = 'bold'
/** Every intro and outro preset the design system defines (CDS-70). */
export type ClipIntroPresetId = keyof typeof design.regions.intro
export type ClipOutroPresetId = keyof typeof design.regions.outro
export type ClipRegionPresets = { intro: ClipIntroPresetId; outro: ClipOutroPresetId }
/** The presets a new project starts in where no template gives it others (CLIP-111,
 *  CLIP-168), a new template starts in (CLIP-166), and every surface that draws a region
 *  without a selection draws. */
export const CLIP_DEFAULT_REGION_PRESETS: ClipRegionPresets = { intro: 'a', outro: 'b' }
/** Every approved caption style, in the order the design spec names them. The
 *  set a project may actually assign from is its own selection (CLIP-142). */
export type ClipCaptionStyleId = keyof typeof design.regions.caption
export const CLIP_CAPTION_STYLES = Object.keys(design.regions.caption) as ClipCaptionStyleId[]
export function clipCaption() {
  return design.regions.caption.bold
}
export function clipCaptionRule(id?: string) {
  const rules: Record<string, typeof design.regions.caption.bold> = design.regions.caption
  return rules[id || CLIP_DEFAULT_CAPTION_STYLE] ?? clipCaption()
}
/** The size range a caption in this style may be set at: CDS-3's floor for its
 *  type role, and the size that role is set at, which V2 refuses to exceed. */
export function clipCaptionSizes(id?: string) {
  const role = design.type[clipCaptionRule(id).type as keyof typeof design.type]
  return { min: role.min, max: role.size }
}
export function clipRegion(kind: 'intro' | 'outro', id: string) {
  return Object.entries(design.regions[kind]).find(([key]) => key === id)?.[1]
}
/** A CDS colour token's hex and alpha, for an SVG attribute that takes neither a
 *  Tailwind class nor a CSS variable — a plate's fill opacity, say. The hex
 *  itself also lives in the palette, which is where components read colour. */
export function clipPaint(token: keyof typeof design.color) {
  const { hex, alpha } = design.color[token]
  return { hex, alpha }
}
/** The drop shadow an unplated style paints under its text (CDS-14). */
export const CLIP_SHADOW = design.shadow
export const CLIP_SPACING = design.spacing
export const CLIP_TIMING = design.timing
/** CDS-36's transitions: the default, the fade a scene change earns and the
 *  fade-through-black nothing offers yet. */
export const CLIP_TRANSITION = design.transition
/** CLIP-98's six fixed playback rates as integer permille, the 1x unit and the
 *  output cadence a slow rate still has to reach (CDS-68). */
export const CLIP_PLAYBACK = design.playback
/** The rates a cut may carry, ascending. Nothing else is a rate. */
export const CLIP_RATES = design.playback.rates_permille as readonly number[]
/** CDS-43: how many copies a cut may carry, the cut length the second one needs
 *  and how short the second sentence has to be to stand alone. */
export const CLIP_COPY = design.copy
export const CLIP_ACCENT_HEX = design.accent
/** The voice CDS-42 refuses: emoji and these tokens. */
export const CLIP_VOICE = design.voice
/** The type scale itself, for the two texts that answer to a role rather than to
 *  a copy style: the hook card's sentence and its category chip. */
export const CLIP_TYPE = design.type

export type ClipCaptionPace = 'steady' | 'rapid'
export const CLIP_RAPID = design.rapid
