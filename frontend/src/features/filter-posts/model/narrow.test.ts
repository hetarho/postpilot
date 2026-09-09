import type { PostListItem } from '@/entities/post'
import { narrowPosts } from './narrow'

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

const JEJU = post({ slug: 'jeju', title: '제주 3일', status: 'review', tags: ['제주', '카페'] })
const BUSAN = post({ slug: 'busan', title: '부산 밥상', status: 'draft', tags: ['맛집'] })
const SEOUL = post({ slug: 'seoul', title: 'Seoul Cafe Tour', status: 'finalized' })
const POSTS = [JEJU, BUSAN, SEOUL]

describe('narrowPosts', () => {
  it('keeps every post in server order when nothing narrows', () => {
    expect(narrowPosts(POSTS, {}).map((kept) => kept.post.slug)).toEqual(['jeju', 'busan', 'seoul'])
    expect(narrowPosts(POSTS, { q: '   ' }).map((kept) => kept.post.slug)).toHaveLength(3)
  })

  it('matches the title and reports no tag for a title hit', () => {
    const kept = narrowPosts(POSTS, { q: '밥상' })

    expect(kept).toHaveLength(1)
    expect(kept[0].post.slug).toBe('busan')
    expect(kept[0].matchedTags).toEqual([])
  })

  it('matches a tag and names only the tags that matched', () => {
    const kept = narrowPosts(POSTS, { q: '카페' })

    expect(kept).toHaveLength(1)
    expect(kept[0].post.slug).toBe('jeju')
    expect(kept[0].matchedTags).toEqual(['카페'])
  })

  it('reads a leading # as naming a tag rather than as part of it', () => {
    expect(narrowPosts(POSTS, { q: '#맛집' }).map((kept) => kept.post.slug)).toEqual(['busan'])
  })

  it('ignores case and surrounding or repeated whitespace', () => {
    expect(narrowPosts(POSTS, { q: '  CAFE  ' }).map((kept) => kept.post.slug)).toEqual(['seoul'])
    expect(narrowPosts(POSTS, { q: 'seoul   cafe' }).map((kept) => kept.post.slug)).toEqual([
      'seoul',
    ])
  })

  it('filters on the stored status', () => {
    expect(narrowPosts(POSTS, { status: 'finalized' }).map((kept) => kept.post.slug)).toEqual([
      'seoul',
    ])
  })

  // A generating draft is still a draft: the badge says AI 생성 중, the status does not (POST-66).
  it('keeps a post with a running job under its own status', () => {
    const generating = post({
      slug: 'generating',
      title: '생성 중',
      status: 'draft',
      activeJob: {
        id: 'job-1',
        kind: 'generate',
        status: 'running',
        stage: 'observe',
        progressDone: 0,
        progressTotal: 0,
        failure: undefined,
        postSlug: 'generating',
        observeModel: undefined,
        writeModel: undefined,
        createdAt: '2026-08-28T11:00:00Z',
        updatedAt: '2026-08-28T11:00:00Z',
        targetLanguage: 'ko',
      },
    })

    expect(narrowPosts([generating], { status: 'draft' })).toHaveLength(1)
  })

  it('composes the search and the filter as AND', () => {
    expect(narrowPosts(POSTS, { q: '제주', status: 'draft' })).toEqual([])
    expect(narrowPosts(POSTS, { q: '제주', status: 'review' }).map((k) => k.post.slug)).toEqual([
      'jeju',
    ])
  })

  it('matches nothing on a query no title or tag carries', () => {
    expect(narrowPosts(POSTS, { q: '없는말' })).toEqual([])
  })
})
