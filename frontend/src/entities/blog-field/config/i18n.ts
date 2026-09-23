import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16): the name of each 분야 and of 없음. */
export const i18n = {
  namespace: 'posts',
  ko: {
    blogField: {
      none: '없음',
      restaurant: '맛집',
      cafe: '카페',
      domestic_travel: '국내여행',
      fashion_beauty: '패션·미용',
      product_review: '상품리뷰',
      parenting_marriage: '육아·결혼',
      pets: '반려동물',
      interior_diy: '인테리어·DIY',
      daily_life: '일상·생각',
    },
  },
  en: {
    blogField: {
      none: 'None',
      restaurant: 'Restaurants',
      cafe: 'Cafés',
      domestic_travel: 'Domestic travel',
      fashion_beauty: 'Fashion & beauty',
      product_review: 'Product reviews',
      parenting_marriage: 'Parenting & marriage',
      pets: 'Pets',
      interior_diy: 'Interior & DIY',
      daily_life: 'Daily life & thoughts',
    },
  },
} as const satisfies I18nFragment
