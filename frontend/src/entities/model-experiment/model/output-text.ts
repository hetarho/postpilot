import type { CandidateOutput } from './types'

/** Export only the owner-visible retained payload, without model pricing or usage metadata. */
export function legacyOutputText(output: CandidateOutput): string {
  if (output.kind === 'voice') return output.text
  if (output.kind === 'observe') {
    const observations = output.observations.map(
      ({ file, scene, mood, visibleText, objects, peoplePresent, events, speech }) => ({
        file,
        scene,
        mood,
        visibleText,
        objects,
        peoplePresent,
        events,
        speech,
      }),
    )
    return JSON.stringify(observations, null, 2)
  }
  const content = output.content
  return [
    content.title,
    content.summary,
    ...content.blocks.map((block) => {
      if (block.items.length > 0) return block.items.map((item) => `• ${item}`).join('\n')
      if (block.file) return [block.file, block.caption].filter(Boolean).join(': ')
      if (block.files.length > 0)
        return [block.files.join(', '), block.caption].filter(Boolean).join(': ')
      return block.content
    }),
    content.tags.map((tag) => `#${tag}`).join(' '),
  ]
    .filter(Boolean)
    .join('\n\n')
}
