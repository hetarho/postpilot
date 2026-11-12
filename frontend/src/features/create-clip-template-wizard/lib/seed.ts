import { CLIP_COMPOSITION_LIMITS } from '@/entities/clip-design'
import type { ClipRecipe } from '@/entities/clip-template'

/** What a clip hands the wizard (CLIP-233): one custom stage per storyline body paragraph, in
 *  order, and the clip's design selection. */
export interface ClipWizardSeed {
  stages: { line: string }[]
  /** A preset the clip never chose is absent, and the wizard keeps its own default. */
  design: Partial<Pick<ClipRecipe, 'introPreset' | 'outroPreset'>> &
    Pick<ClipRecipe, 'allowedCaptionStyles'>
}

const bounded = (text: string, max: number) => Array.from(text).slice(0, max).join('').trim()

/** A paragraph's first sentence on one line: up to its first sentence-ending mark or line break,
 *  cut at the stage line's bound (CLIP-141). */
export function firstSentence(paragraph: string): string {
  const text = paragraph.trim()
  const end = text.search(/[.!?。！？](\s|$)|\n/u)
  const sentence = end < 0 ? text : text.slice(0, text[end] === '\n' ? end : end + 1)
  return bounded(sentence.replace(/\s+/gu, ' '), CLIP_COMPOSITION_LIMITS.promptChars)
}

/** The seed a clip's storyline and design selection make. A paragraph with no text offers no
 *  stage. */
export function clipWizardSeed({
  paragraphs,
  design,
}: {
  paragraphs: readonly { text: string }[]
  design: {
    introPreset?: '' | ClipRecipe['introPreset']
    outroPreset?: '' | ClipRecipe['outroPreset']
    allowedCaptionStyles?: readonly string[]
  }
}): ClipWizardSeed {
  return {
    stages: paragraphs
      .map((paragraph) => firstSentence(paragraph.text))
      .filter((line) => line !== '')
      .map((line) => ({ line })),
    design: {
      ...(design.introPreset ? { introPreset: design.introPreset } : {}),
      ...(design.outroPreset ? { outroPreset: design.outroPreset } : {}),
      allowedCaptionStyles: [...(design.allowedCaptionStyles ?? [])],
    },
  }
}

/** The stage's name on the wizard's lists, until its own step names it. */
export const seedStageName = (line: string) => bounded(line, CLIP_COMPOSITION_LIMITS.labelChars)
