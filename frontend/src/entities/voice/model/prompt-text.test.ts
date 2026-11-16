import { describe, expect, it } from 'vitest'
import { promptText } from './prompt-text'
import type { VoiceExperienceKindInfo } from './types'

const kinds: VoiceExperienceKindInfo[] = [
  { kind: 'food', label: '맛집', plain: '그곳' },
  { kind: 'product', label: '제품', plain: '그 제품' },
]

describe('promptText', () => {
  it('names the experience', () => {
    const prompt = { text: '{experience}, 어떻게 가게 됐어요?', experienceKind: 'food' as const }
    expect(promptText(prompt, kinds, { name: '보리밥집' })).toBe('보리밥집, 어떻게 가게 됐어요?')
  })

  it("reads the kind's plain word for an empty name or no experience", () => {
    const prompt = { text: '{experience}, 어땠어요?', experienceKind: 'product' as const }
    expect(promptText(prompt, kinds, { name: '' })).toBe('그 제품, 어땠어요?')
    expect(promptText(prompt, kinds)).toBe('그 제품, 어땠어요?')
  })

  it('returns situation text unchanged', () => {
    expect(promptText({ text: '오늘 뭐 먹었어요?' }, kinds, { name: 'x' })).toBe(
      '오늘 뭐 먹었어요?',
    )
  })
})
