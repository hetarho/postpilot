import { beforeEach, expect, it } from 'vitest'
import {
  rememberClipEntry,
  readClipReturnContext,
  clipReturnDestination,
  retainMintedClipEntry,
  markClipHistoryReturn,
  readClipHistoryReturn,
  completeClipHistoryReturn,
  forgetClipEntry,
} from './navigation-context'

beforeEach(() => sessionStorage.clear())
it.each(['/clips'] as const)(
  'retains the prepared-story filter and scroll on return to %s',
  (path) => {
    rememberClipEntry('alice', {
      path,
      section: 'clips',
      filters: { q: '이야기 & 기록', status: 'storyline_ready', private: 'omit' },
      scrollY: 740,
      targetId: 'ready',
    })
    const destination = clipReturnDestination('alice', 'ready')
    expect(destination.path).toBe(path)
    expect(
      Object.fromEntries(new URL(destination.href, 'https://test.invalid').searchParams),
    ).toEqual({
      q: '이야기 & 기록',
      status: 'storyline_ready',
    })
    expect(markClipHistoryReturn('alice', 'ready')).toBe(true)
    expect(readClipHistoryReturn('alice')).toMatchObject({ path, scrollY: 740 })
  },
)
it('retains creation through minting and reload without adopting another owner or record origin', () => {
  rememberClipEntry('alice', { path: '/clips', section: 'creation', filters: {}, scrollY: 40 })
  expect(retainMintedClipEntry('alice', 'minted')).toBe(true)
  expect(readClipReturnContext('alice', 'minted')?.path).toBe('/clips')
  expect(clipReturnDestination('alice', 'minted').href).toBe('/clips')
  expect(clipReturnDestination('bob', 'minted').href).toBe('/clips')
  expect(clipReturnDestination('alice', 'other').href).toBe('/clips')
})
it('keeps each clip origin when another clip or post context is entered and restores filtered history once', () => {
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'creation',
    filters: {},
    scrollY: 0,
    targetId: 'first',
  })
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'clips',
    filters: { q: '여행 & 50%', status: 'draft', private: 'omit' },
    scrollY: 740,
    targetId: 'second',
  })
  sessionStorage.setItem(
    'postpilot.return.alice',
    JSON.stringify({
      version: 1,
      ownerKey: 'alice',
      path: '/posts',
      section: 'posts',
      filters: {},
      scrollY: 3,
      targetId: 'post',
    }),
  )
  expect(clipReturnDestination('alice', 'first').href).toBe('/clips')
  const destination = clipReturnDestination('alice', 'second')
  const query = new URL(destination.href, 'https://test.invalid').searchParams
  expect(query.get('q')).toBe('여행 & 50%')
  expect(query.get('status')).toBe('draft')
  expect(query.has('private')).toBe(false)
  expect(markClipHistoryReturn('alice', 'second')).toBe(true)
  expect(readClipHistoryReturn('alice')?.scrollY).toBe(740)
  completeClipHistoryReturn('alice')
  expect(readClipHistoryReturn('alice')).toBeUndefined()
  expect(clipReturnDestination('alice', 'second').scrollY).toBe(740)
})
it('refuses external, malformed and deleted-record parents rather than routing to them', () => {
  for (const path of [
    'https://outside.test',
    '//outside.test',
    '/clips/deleted',
    '/%2f%2foutside.test',
    '/%',
  ]) {
    sessionStorage.setItem(
      'postpilot.return.alice.clip.saved',
      JSON.stringify({
        version: 1,
        ownerKey: 'alice',
        path,
        section: 'clips',
        filters: {},
        scrollY: 0,
        targetId: 'saved',
      }),
    )
    expect(readClipReturnContext('alice', 'saved')).toBeUndefined()
    expect(clipReturnDestination('alice', 'saved').href).toBe('/clips')
  }
  sessionStorage.setItem(
    'postpilot.return.alice.clip.saved',
    JSON.stringify({
      version: 1,
      ownerKey: 'bob',
      path: '/clips',
      section: 'creation',
      filters: {},
      scrollY: 0,
      targetId: 'saved',
    }),
  )
  expect(clipReturnDestination('alice', 'saved').href).toBe('/clips')
})
it('uses named safe parents when storage is unavailable and does not leak unsafe query controls', () => {
  const storage = {
    getItem: () => {
      throw Error('unavailable')
    },
    setItem: () => {
      throw Error('unavailable')
    },
    removeItem: () => {},
  }
  expect(clipReturnDestination('alice', undefined, storage).href).toBe('/clips')
  expect(clipReturnDestination('alice', 'saved', storage).href).toBe('/clips')
  expect(
    rememberClipEntry(
      'alice',
      { path: '/clips', section: 'creation', filters: {}, scrollY: 0 },
      storage,
    ),
  ).toBe(false)
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'clips',
    filters: { status: 'unknown', redirect: 'https://outside.test' },
    scrollY: 0,
    targetId: 'saved',
  })
  expect(clipReturnDestination('alice', 'saved').href).toBe('/clips')
})
it('confirmed deletion forgets the record origin while retaining the pending filtered history scroll', () => {
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'clips',
    filters: { q: '여행' },
    scrollY: 340,
    targetId: 'deleted',
  })
  markClipHistoryReturn('alice', 'deleted')
  forgetClipEntry('alice', 'deleted')
  expect(readClipReturnContext('alice', 'deleted')).toBeUndefined()
  expect(readClipHistoryReturn('alice')).toMatchObject({ path: '/clips', scrollY: 340 })
  expect(clipReturnDestination('alice', 'deleted').href).toBe('/clips')
})

it('returns to the saved history origin with its narrowing', () => {
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'clips',
    filters: { q: '제주' },
    scrollY: 10,
    targetId: 'clip',
  })
  expect(clipReturnDestination('alice', 'clip').href).toBe('/clips?q=%EC%A0%9C%EC%A3%BC')
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'clips',
    filters: {},
    scrollY: 10,
    targetId: 'clip',
  })
  expect(clipReturnDestination('alice', 'clip').href).toBe('/clips')
})

it.each(['/', '/library'])('maps the stored obsolete %s parent to the 영상 list', (path) => {
  sessionStorage.setItem(
    'postpilot.return.alice.clip.saved',
    JSON.stringify({
      version: 1,
      ownerKey: 'alice',
      targetId: 'saved',
      path,
      section: 'creation',
      filters: {},
      scrollY: 0,
    }),
  )
  expect(clipReturnDestination('alice', 'saved').href).toBe('/clips')
})

it('reads a stored /clips history entry as its history with the narrowing', () => {
  sessionStorage.setItem(
    'postpilot.return.alice.clip.saved',
    JSON.stringify({
      version: 1,
      ownerKey: 'alice',
      targetId: 'saved',
      path: '/clips',
      section: 'clips',
      filters: { status: 'draft' },
      scrollY: 120,
    }),
  )
  expect(readClipReturnContext('alice', 'saved')).toMatchObject({
    path: '/clips',
    section: 'clips',
  })
  expect(clipReturnDestination('alice', 'saved')).toMatchObject({
    href: '/clips?status=draft',
    scrollY: 120,
  })
  expect(markClipHistoryReturn('alice', 'saved')).toBe(true)
  expect(readClipHistoryReturn('alice')).toMatchObject({ path: '/clips', scrollY: 120 })
})

it('does not treat a creation entry as a history return', () => {
  rememberClipEntry('alice', {
    path: '/clips',
    section: 'video',
    filters: {},
    scrollY: 0,
    targetId: 'created',
  })
  expect(clipReturnDestination('alice', 'created').href).toBe('/clips')
  expect(markClipHistoryReturn('alice', 'created')).toBe(false)
})

it.each(['/naver-clip', '/insta-clip'])(
  'returns a context stored under the earlier %s address to the 영상 list',
  (path) => {
    sessionStorage.setItem(
      'postpilot.return.alice.clip.saved',
      JSON.stringify({
        version: 1,
        ownerKey: 'alice',
        path,
        section: 'clips',
        filters: { q: '여행', status: 'draft' },
        scrollY: 55,
        targetId: 'saved',
      }),
    )
    expect(readClipReturnContext('alice', 'saved')?.path).toBe('/clips')
    expect(clipReturnDestination('alice', 'saved')).toMatchObject({
      path: '/clips',
      href: '/clips?q=%EC%97%AC%ED%96%89&status=draft',
      scrollY: 55,
    })
  },
)
