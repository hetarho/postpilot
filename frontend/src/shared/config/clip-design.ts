/** The clip design system (CDS).
 *
 *  `clip-design.json` is byte-identical to the file the renderer embeds — a Go
 *  test compares the two — so every number the preview draws and every limit the
 *  validators check is the same number the render uses. Nothing here is typed by
 *  hand; changing a value means editing that file, on both sides at once. */
import design from './clip-design.json'

export const CLIP_DESIGN = design

export type ClipStyleId = 'clean' | 'memo' | 'bold' | 'mark'
export type ClipRatioId = 'vertical' | 'horizontal' | 'square'
export type ClipPresetId = keyof typeof design.presets
export type ClipDisclosureId = keyof typeof design.disclosure
export type ClipCTAId = keyof typeof design.cta

export const CLIP_STYLES = Object.keys(design.styles) as ClipStyleId[]
export const CLIP_PRESETS = Object.keys(design.presets) as ClipPresetId[]
export const CLIP_DISCLOSURES = Object.keys(design.disclosure) as ClipDisclosureId[]
export const CLIP_CTAS = Object.keys(design.cta) as ClipCTAId[]

/** The style's own drawing rule: plate, padding, stroke, shadow, limits and
 *  default placement (CDS-22 through CDS-26). */
export function clipStyle(style: ClipStyleId) {
  return design.styles[style]
}
/** The type scale entry a style typesets in (CDS-19). */
export function clipType(style: ClipStyleId) {
  return design.type[design.styles[style].type as keyof typeof design.type]
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
export const CLIP_GUARDS = design.guards
/** CDS-36's transitions: the default, the fade a scene change earns and the
 *  fade-through-black nothing offers yet. */
export const CLIP_TRANSITION = design.transition
export const CLIP_CLASSES = design.classes
export const CLIP_SCENE_STYLES = design.scene_styles
export const CLIP_FACTS = design.facts
export const CLIP_ACCENT_HEX = design.accent
/** The voice CDS-42 refuses: emoji and these tokens. */
export const CLIP_VOICE = design.voice
/** The type scale itself, for the two texts that answer to a role rather than to
 *  a copy style: the hook card's sentence and its category chip (CDS-28). */
export const CLIP_TYPE = design.type
