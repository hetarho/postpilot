import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    ratio: { vertical: '세로 9:16', horizontal: '가로 16:9', square: '정방형 1:1' },
    preset: {
      restaurant: '음식점',
      cafe: '카페',
      stay: '숙소·여행',
      beauty: '뷰티',
      home: '생활용품·가전',
    },
    cta: {
      '': '프리셋 기본값',
      blog: '자세한 후기는 블로그에',
      place: '위치는 프로필 정보 태그에서',
      save: '저장해두고 방문해보세요',
    },
    accent: {
      none: '기본',
      coral: '코랄',
      amber: '앰버',
      lime: '라임',
      teal: '청록',
      blue: '파랑',
      violet: '보라',
      pink: '분홍',
    },
  },
  en: {
    ratio: { vertical: 'Vertical 9:16', horizontal: 'Horizontal 16:9', square: 'Square 1:1' },
    preset: {
      restaurant: 'Restaurant',
      cafe: 'Cafe',
      stay: 'Stay & travel',
      beauty: 'Beauty',
      home: 'Home & appliances',
    },
    cta: {
      '': "The preset's default",
      blog: 'Full review on the blog',
      place: 'Location in the profile tags',
      save: 'Save it and drop by',
    },
    accent: {
      none: 'Neutral',
      coral: 'Coral',
      amber: 'Amber',
      lime: 'Lime',
      teal: 'Teal',
      blue: 'Blue',
      violet: 'Violet',
      pink: 'Pink',
    },
  },
} as const satisfies I18nFragment
