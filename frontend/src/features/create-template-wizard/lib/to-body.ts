import { newBlock, toBody, type BuilderBlock } from '@/entities/template'
import type { SectionDraft } from '../model/wizard-machine'

/** The template body the wizard's answers make (TMPL-71): each checked section, in order, becomes
 *  its heading as one 고정 문구 line, then what stands there, then its photo place when it has
 *  photos. 직접 입력 is a 데이터 받기 field of the AI가 쓰는 글 flavor: the author answers its
 *  question and the AI writes the section from that answer (TMPL-74). A repeating section wraps its words and its photos in `사진마다 반복`; a 데이터 받기
 *  field never repeats, because the grammar refuses one inside a repeat. The title area stays
 *  empty (TMPL-52), so this builds the body alone. */
export function toWizardBlocks(sections: readonly SectionDraft[]): BuilderBlock[] {
  const blocks: BuilderBlock[] = []
  for (const section of sections) {
    if (!section.checked) continue
    blocks.push({ ...literal(section.heading.trim()) })
    // 질문을 바탕으로 작성's optional fixed text (rating criteria above a 별점, TMPL-74) stands
    // directly between the heading and the field with no blank line, so a skipped answer takes
    // heading and text along (TMPL-45).
    const fixed = section.content === 'ask' ? closedUp(section.literal) : ''
    if (fixed !== '') blocks.push(literal(fixed))
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

/** The text's nonblank lines, trimmed at their ends, one under another. */
function closedUp(text: string): string {
  return text
    .split('\n')
    .map((line) => line.trimEnd())
    .filter((line) => line.trim() !== '')
    .join('\n')
}

function literal(text: string): BuilderBlock {
  return { ...newBlock('text'), kind: 'text', text }
}

function contentBlock(section: SectionDraft): BuilderBlock {
  // The AI writes about the role; the topic is what the section covers, in the words the owner
  // saw on the checklist.
  const topic = section.help.trim() || section.name
  switch (section.content) {
    case 'write':
      return { ...newBlock('write'), kind: 'write', text: topic }
    case 'ask':
      // The same topic, with the author's answer as its facts (TMPL-45): the answer is what the
      // part is written from, never a line pasted into the post.
      return {
        ...newBlock('write'),
        kind: 'write',
        text: topic,
        ask: section.question.trim(),
        ...(section.required ? { required: true } : {}),
      }
    case 'literal':
      return literal(section.literal.trim())
  }
}
