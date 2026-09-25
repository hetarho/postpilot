import type { PostListItem } from '@/entities/post'
import { matchedTags } from './narrow'

function post(overrides: Partial<PostListItem> & { slug: string }): PostListItem {
  return {
    title: '',
    status: 'draft',
    updatedAt: '2026-08-28T11:58:00Z',
    voice: { id: 'voice-1', name: '기본', deleted: false, sourceLanguage: 'ko' },
    template: { id: '', name: '' },
    activeJob: undefined,
    pendingExperimentId: '',
    targetLanguage: 'ko',
    contentLanguage: undefined,
    tags: [],
    ...overrides,
  }
}

const JEJU = post({ slug: 'jeju', title: '제주 3일', tags: ['제주', '카페', '제주 카페'] })

// The narrowing itself is the server's (POST-91, backend list_test.go); what stays here is naming
// the tags a kept row matched, which has to normalize the same way the server's match did.
describe('matchedTags', () => {
  it('names nothing while nothing is searched', () => {
    expect(matchedTags(JEJU, undefined)).toEqual([])
    expect(matchedTags(JEJU, '   ')).toEqual([])
    expect(matchedTags(JEJU, '#')).toEqual([])
  })

  it('names only the tags that matched, in their order', () => {
    expect(matchedTags(JEJU, '카페')).toEqual(['카페', '제주 카페'])
  })

  it('names no tag for a hit on the title alone', () => {
    expect(matchedTags(JEJU, '3일')).toEqual([])
  })

  it('reads a leading # as naming a tag', () => {
    expect(matchedTags(JEJU, '#제주')).toEqual(['제주', '제주 카페'])
  })

  it('ignores case and surrounding or repeated whitespace', () => {
    const seoul = post({ slug: 'seoul', tags: ['Seoul Cafe'] })
    expect(matchedTags(seoul, '  CAFE  ')).toEqual(['Seoul Cafe'])
    expect(matchedTags(seoul, 'seoul   cafe')).toEqual(['Seoul Cafe'])
  })
})
