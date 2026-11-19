import { describe, expect, it, vi } from 'vitest'
import { ADVERTISER_GUIDE_MAX_CHARS } from '@/entities/advertiser-guide'
import type { PostDraftSave } from '@/entities/post'
import { createPostFromClip } from './createPostFromClip'

async function seeded(guide: { text: string; keywords: string[]; hashtags: string[] }) {
  const save = vi.fn<(draft: PostDraftSave) => Promise<string>>().mockResolvedValue('slug')
  await createPostFromClip({
    project: {
      title: '성수 카페',
      instruction: '라떼 위주',
      campaign: true,
      advertiserGuide: guide,
      language: 'ko',
    },
    fallbackLanguage: 'ko',
    save,
  })
  return save.mock.calls[0][0]
}

// CLIP-233, POST-118: the clip's 필수 해시태그 reach the post inside its guide text.
describe('이 체험으로 글 쓰기', () => {
  it('folds the clip hashtags the text lacks into its last line and sends none', async () => {
    const draft = await seeded({
      text: '주차 안내\n#성수카페',
      keywords: ['크림라떼'],
      hashtags: ['성수카페', '라떼맛집'],
    })
    expect(draft.advertiserGuide).toMatchObject({
      text: '주차 안내\n#성수카페\n#라떼맛집',
      keywords: ['크림라떼'],
      hashtags: [],
    })
  })
  it('appends only tokens that keep the text within the guide limit', async () => {
    const text = 'x'.repeat(ADVERTISER_GUIDE_MAX_CHARS - 4)
    const draft = await seeded({ text, keywords: [], hashtags: ['ab', 'cd'] })
    expect(draft.advertiserGuide?.text).toBe(`${text}\n#ab`)
    expect(draft.advertiserGuide?.hashtags).toEqual([])
  })
})
