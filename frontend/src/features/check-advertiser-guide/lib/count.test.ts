import { describe, expect, it } from 'vitest'
import { create } from '@bufbuild/protobuf'
import { EMPTY_GUIDE } from '@/entities/advertiser-guide'
import { BlockSchema, BlockType, GalleryLayout, PostContentSchema } from '@/shared/api'
import {
  bodyTexts,
  countCharacters,
  countPhotos,
  countedItems,
  hashtagPresent,
  keywordPresent,
  tickedLines,
  toggleTick,
} from './count'

const content = create(PostContentSchema, {
  title: '성수 카페 투어',
  summary: '요약은 세지 않아요',
  tags: ['#성수맛집', '크림 라떼'],
  blocks: [
    create(BlockSchema, { type: BlockType.TEXT, content: '라떼가 맛있었다.' }),
    create(BlockSchema, { type: BlockType.HEADING, level: 2, content: '메뉴' }),
    create(BlockSchema, { type: BlockType.QUOTE, content: '다시 올게요' }),
    create(BlockSchema, { type: BlockType.LIST, items: ['크림 라떼', '휘낭시에'] }),
    create(BlockSchema, {
      type: BlockType.IMAGE,
      file: 'IMG_1.jpg',
      alt: '대체 텍스트는 세지 않아요',
      caption: '창가 자리',
    }),
    create(BlockSchema, {
      type: BlockType.GALLERY,
      files: ['IMG_2.jpg', 'IMG_3.jpg', 'IMG_4.jpg'],
      layout: GalleryLayout.COLLAGE,
      caption: '디저트',
    }),
    create(BlockSchema, { type: BlockType.IMAGE, file: '', caption: '' }),
    create(BlockSchema, { type: BlockType.VIDEO, file: 'clip.mp4', caption: '영상 설명' }),
  ],
})

describe('the body the checklist reads', () => {
  it('takes TEXT, HEADING, QUOTE and LIST text and photo captions, never the title, tags or alt', () => {
    expect(bodyTexts(content)).toEqual([
      '라떼가 맛있었다.',
      '메뉴',
      '다시 올게요',
      '크림 라떼',
      '휘낭시에',
      '창가 자리',
      '디저트',
    ])
  })
})

describe('keywords', () => {
  it('match the title or the body, ignoring case and spaces on both sides', () => {
    expect(keywordPresent(content, '성수 카페')).toBe(true)
    expect(keywordPresent(content, '성수카페')).toBe(true)
    expect(keywordPresent(content, '크 림라떼')).toBe(true)
    expect(keywordPresent({ ...content, title: 'SEONGSU Cafe' }, 'seongsucafe')).toBe(true)
    expect(keywordPresent(content, '창가자리')).toBe(true)
  })

  it('never match tags, alt text, a video caption or across two blocks', () => {
    expect(keywordPresent(content, '성수맛집')).toBe(false)
    expect(keywordPresent(content, '대체 텍스트')).toBe(false)
    expect(keywordPresent(content, '영상 설명')).toBe(false)
    expect(keywordPresent(content, '메뉴다시')).toBe(false)
    expect(keywordPresent(content, '   ')).toBe(false)
  })
})

describe('characters', () => {
  it('count the body with and without whitespace, in code points', () => {
    // 라떼가 맛있었다.(9) 메뉴(2) 다시 올게요(6) 크림 라떼(5) 휘낭시에(4) 창가 자리(5) 디저트(3)
    expect(countCharacters(content)).toEqual({ withSpaces: 34, withoutSpaces: 30 })
    const spaced = create(PostContentSchema, {
      blocks: [create(BlockSchema, { type: BlockType.TEXT, content: 'a\tb\n c🙂' })],
    })
    expect(countCharacters(spaced)).toEqual({ withSpaces: 7, withoutSpaces: 4 })
  })
})

describe('photos', () => {
  it('count every placed photo, alone or in a group, and no empty marker or video', () => {
    expect(countPhotos(content)).toBe(4)
  })
})

describe('hashtags', () => {
  it('match a tag ignoring a leading # on either side, case and spaces', () => {
    expect(hashtagPresent(content.tags, '성수맛집')).toBe(true)
    expect(hashtagPresent(content.tags, '#크림라떼')).toBe(true)
    expect(hashtagPresent(['Seongsu'], '#seongsu')).toBe(true)
    expect(hashtagPresent(content.tags, '성수')).toBe(false)
    expect(hashtagPresent(content.tags, '#')).toBe(false)
  })
})

describe('the counted items', () => {
  it('follow the guide form order and mark the character minimum by the count without spaces', () => {
    const items = countedItems(content, {
      text: '',
      keywords: ['성수 카페', '주차'],
      minChars: 32,
      minPhotos: 4,
      hashtags: ['성수맛집'],
    })
    expect(items).toEqual([
      { kind: 'keyword', keyword: '성수 카페', met: true },
      { kind: 'keyword', keyword: '주차', met: false },
      {
        kind: 'characters',
        minimum: 32,
        count: { withSpaces: 34, withoutSpaces: 30 },
        met: false,
      },
      { kind: 'photos', minimum: 4, count: 4, met: true },
      { kind: 'hashtag', hashtag: '성수맛집', met: true },
    ])
    expect(countedItems(content, EMPTY_GUIDE)).toEqual([])
  })
})

describe('the free-text lines', () => {
  const guide = { ...EMPTY_GUIDE, text: '대표 메뉴 소개\n\n  주차 정보 안내  \n대표 메뉴 소개' }

  it('are every non-empty trimmed line, ticked by its text', () => {
    expect(tickedLines(guide, ['주차 정보 안내', '지워진 줄'])).toEqual([
      { line: '대표 메뉴 소개', ticked: false },
      { line: '주차 정보 안내', ticked: true },
      { line: '대표 메뉴 소개', ticked: false },
    ])
  })

  it('keep only ticks of lines the guide still holds, once each', () => {
    expect(toggleTick(guide, ['지워진 줄'], '대표 메뉴 소개', true)).toEqual(['대표 메뉴 소개'])
    expect(
      toggleTick(guide, ['대표 메뉴 소개', '주차 정보 안내'], '대표 메뉴 소개', false),
    ).toEqual(['주차 정보 안내'])
    // An edited line is another text, so it starts unticked.
    const edited = { ...guide, text: '대표 메뉴 두 개 소개' }
    expect(tickedLines(edited, ['대표 메뉴 소개'])).toEqual([
      { line: '대표 메뉴 두 개 소개', ticked: false },
    ])
  })
})
