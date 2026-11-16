import { beforeEach, expect, it } from 'vitest'
import {
  rememberPostEntry,
  readPostReturnContext,
  postReturnDestination,
  retainMintedPostEntry,
  markPostHistoryReturn,
  readPostHistoryReturn,
} from './navigation-context'

beforeEach(() => sessionStorage.clear())

it('retains creation context across minting and reload, scoped to the owner and post', () => {
  rememberPostEntry('owner', {
    path: '/posts',
    section: 'creation',
    filters: {},
    scrollY: 0,
    targetId: 'new',
  })
  expect(retainMintedPostEntry('owner', 'minted')).toBe(true)
  expect(postReturnDestination('owner', 'minted').href).toBe('/posts')
  expect(postReturnDestination('other', 'minted').href).toBe('/posts')
  expect(postReturnDestination('owner', 'different').href).toBe('/posts')
})

it('retains history filters and scroll in the channel return URL', () => {
  rememberPostEntry('owner', {
    path: '/posts',
    section: 'posts',
    filters: { q: '제주 & 여행', status: 'review' },
    scrollY: 820,
    targetId: 'saved',
  })
  const destination = postReturnDestination('owner', 'saved')
  expect(destination.path).toBe('/posts')
  expect(new URL(destination.href, 'https://test.invalid').searchParams.get('q')).toBe(
    '제주 & 여행',
  )
  expect(destination.scrollY).toBe(820)
  expect(readPostReturnContext('owner', 'saved')?.filters).toEqual({
    q: '제주 & 여행',
    status: 'review',
  })
})

it('rejects external, malformed, deleted-record and foreign-owner parents', () => {
  for (const path of [
    'https://external.test',
    '//external.test',
    '/posts/deleted',
    '/%2fexternal.test',
  ]) {
    sessionStorage.setItem(
      'postpilot.return.owner',
      JSON.stringify({
        version: 1,
        ownerKey: 'owner',
        path,
        section: 'posts',
        filters: {},
        scrollY: 0,
        targetId: 'saved',
      }),
    )
    expect(readPostReturnContext('owner', 'saved')).toBeUndefined()
  }
  sessionStorage.setItem(
    'postpilot.return.owner',
    JSON.stringify({
      version: 1,
      ownerKey: 'other',
      path: '/library',
      section: 'library',
      filters: {},
      scrollY: 0,
      targetId: 'saved',
    }),
  )
  expect(postReturnDestination('owner', 'saved').href).toBe('/posts')
})

it('falls back safely when storage is unavailable', () => {
  const storage = {
    getItem: () => {
      throw Error('unavailable')
    },
    setItem: () => {
      throw Error('unavailable')
    },
    removeItem: () => {},
  }
  expect(postReturnDestination('owner', undefined, storage).href).toBe('/posts')
  expect(
    rememberPostEntry(
      'owner',
      { path: '/posts', section: 'creation', filters: {}, scrollY: 0 },
      storage,
    ),
  ).toBe(false)
})

it('keeps each post parent when other posts and child contexts replace the generic entry', () => {
  rememberPostEntry('owner', {
    path: '/posts',
    section: 'creation',
    filters: {},
    scrollY: 0,
    targetId: 'first',
  })
  rememberPostEntry('owner', {
    path: '/posts',
    section: 'posts',
    filters: { q: '여행' },
    scrollY: 840,
    targetId: 'second',
  })
  sessionStorage.setItem(
    'postpilot.return.owner',
    JSON.stringify({
      version: 1,
      ownerKey: 'owner',
      path: '/library',
      section: 'clips',
      filters: {},
      scrollY: 300,
      targetId: 'child',
    }),
  )
  expect(postReturnDestination('owner', 'first').href).toBe('/posts')
  expect(postReturnDestination('owner', 'second').scrollY).toBe(840)
  expect(postReturnDestination('owner', 'second').href).toContain('q=')
})

it.each(['/', '/library'])(
  'maps the stored obsolete %s parent to the Naver Blog channel',
  (path) => {
    sessionStorage.setItem(
      'postpilot.return.owner.post.saved',
      JSON.stringify({
        version: 1,
        ownerKey: 'owner',
        targetId: 'saved',
        path,
        section: 'creation',
        filters: {},
        scrollY: 0,
      }),
    )
    expect(postReturnDestination('owner', 'saved').href).toBe('/posts')
  },
)

it('reads a stored /posts history entry as the channel history with its narrowing', () => {
  sessionStorage.setItem(
    'postpilot.return.owner.post.saved',
    JSON.stringify({
      version: 1,
      ownerKey: 'owner',
      targetId: 'saved',
      path: '/posts',
      section: 'posts',
      filters: { q: '여행', status: 'draft' },
      scrollY: 420,
    }),
  )
  expect(readPostReturnContext('owner', 'saved')).toMatchObject({
    path: '/posts',
    section: 'posts',
  })
  expect(postReturnDestination('owner', 'saved')).toMatchObject({
    href: '/posts?q=%EC%97%AC%ED%96%89&status=draft',
    scrollY: 420,
  })
})

it('marks and reads the history scroll return by section on the channel address', () => {
  rememberPostEntry('owner', {
    path: '/posts',
    section: 'posts',
    filters: { q: '여행' },
    scrollY: 640,
    targetId: 'saved',
  })
  expect(markPostHistoryReturn('owner', 'saved')).toBe(true)
  expect(readPostHistoryReturn('owner')).toMatchObject({ path: '/posts', scrollY: 640 })
  rememberPostEntry('owner', {
    path: '/posts',
    section: 'post',
    filters: {},
    scrollY: 0,
    targetId: 'created',
  })
  expect(markPostHistoryReturn('owner', 'created')).toBe(false)
})

it('returns a context stored under the earlier /naver-blog address to the 글 list', () => {
  sessionStorage.setItem(
    'postpilot.return.owner.post.saved',
    JSON.stringify({
      version: 1,
      ownerKey: 'owner',
      path: '/naver-blog',
      section: 'posts',
      filters: { q: '제주', status: 'review' },
      scrollY: 310,
      targetId: 'saved',
    }),
  )
  expect(readPostReturnContext('owner', 'saved')?.path).toBe('/posts')
  expect(postReturnDestination('owner', 'saved')).toMatchObject({
    path: '/posts',
    href: '/posts?q=%EC%A0%9C%EC%A3%BC&status=review',
    scrollY: 310,
  })
})
