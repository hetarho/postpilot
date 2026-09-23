import i18next from 'i18next'
import { describe, expect, it } from 'vitest'
import {
  BLOG_FIELD_IDS,
  NO_BLOG_FIELD,
  blogFieldLabelKey,
  isBlogFieldId,
  type BlogFieldChoice,
} from './blog-field'

describe('the 분야 catalogue', () => {
  // QUAL-23's list and order. The server stores these ids, so a rename here is a data change.
  it('is the nine v1 ids in order', () => {
    expect(BLOG_FIELD_IDS).toEqual([
      'restaurant',
      'cafe',
      'domestic_travel',
      'fashion_beauty',
      'product_review',
      'parenting_marriage',
      'pets',
      'interior_diy',
      'daily_life',
    ])
  })

  it('recognizes an id and nothing that merely resembles one', () => {
    for (const id of BLOG_FIELD_IDS) expect(isBlogFieldId(id)).toBe(true)
    for (const value of [NO_BLOG_FIELD, '맛집', 'Restaurant', 'fashion beauty', ' cafe'])
      expect(isBlogFieldId(value), value).toBe(false)
  })

  it('names 없음 and each 분야 by its own label key', () => {
    expect(blogFieldLabelKey(NO_BLOG_FIELD)).toBe('blogField.none')
    for (const id of BLOG_FIELD_IDS) expect(blogFieldLabelKey(id)).toBe(`blogField.${id}`)
  })

  it('renders every label in Korean and English', async () => {
    const choices: BlogFieldChoice[] = [NO_BLOG_FIELD, ...BLOG_FIELD_IDS]
    const label = (choice: BlogFieldChoice) => i18next.t(blogFieldLabelKey(choice), { ns: 'posts' })
    const previous = i18next.language
    try {
      await i18next.changeLanguage('ko')
      expect(choices.map(label)).toEqual([
        '없음',
        '맛집',
        '카페',
        '국내여행',
        '패션·미용',
        '상품리뷰',
        '육아·결혼',
        '반려동물',
        '인테리어·DIY',
        '일상·생각',
      ])
      await i18next.changeLanguage('en')
      expect(choices.map(label)).toEqual([
        'None',
        'Restaurants',
        'Cafés',
        'Domestic travel',
        'Fashion & beauty',
        'Product reviews',
        'Parenting & marriage',
        'Pets',
        'Interior & DIY',
        'Daily life & thoughts',
      ])
    } finally {
      await i18next.changeLanguage(previous)
    }
  })
})
