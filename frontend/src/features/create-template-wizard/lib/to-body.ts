import { newBlock, toBody, type BuilderBlock } from '@/entities/template'
import type { SectionDraft } from '../model/wizard-machine'

/** The template body the wizard's answers make (TMPL-71): each checked section, in order, becomes
 *  its heading as one 고정 문구 line, then what stands there, then its photo place when it has
 *  photos. A repeating section wraps its words and its photos in `사진마다 반복`; a 데이터 받기
 *  field never repeats, because the grammar refuses one inside a repeat. The title area stays
 *  empty (TMPL-52), so this builds the body alone. */
export function toWizardBlocks(sections: readonly SectionDraft[]): BuilderBlock[] {
  const blocks: BuilderBlock[] = []
  for (const section of sections) {
    if (!section.checked) continue
    blocks.push({ ...literal(section.heading.trim()) })
    const content = contentBlock(section)
    const photo: BuilderBlock | undefined =
      section.photos > 0
        ? { ...newBlock('photo'), kind: 'photo', count: section.photos }
        : undefined
    if (section.repeat && photo && section.content !== 'ask') {
      blocks.push({ ...newBlock('repeat'), kind: 'repeat', children: [content, photo] })
      continue
    }
    blocks.push(content)
    if (photo) blocks.push(photo)
  }
  return blocks
}

export function toWizardBody(sections: readonly SectionDraft[]): string {
  return toBody(toWizardBlocks(sections))
}

function literal(text: string): BuilderBlock {
  return { ...newBlock('text'), kind: 'text', text }
}

function contentBlock(section: SectionDraft): BuilderBlock {
  switch (section.content) {
    case 'write':
      // The AI writes about the role in the owner's voice; the instruction is what the section
      // covers, in the words the owner saw on the checklist.
      return { ...newBlock('write'), kind: 'write', text: section.help.trim() || section.name }
    case 'ask':
      return {
        ...newBlock('text'),
        kind: 'text',
        text: '',
        ask: section.question.trim(),
        ...(section.required ? { required: true } : {}),
      }
    case 'literal':
      return literal(section.literal.trim())
  }
}
