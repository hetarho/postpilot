import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    visibility: { public: '전체 공개', private: '비공개' },
    configure: {
      label: '연결 이름',
      category: '기본 카테고리',
      visibility: '기본 공개 설정',
      save: '기본값 저장',
      failed: '기본값을 저장하지 못했어요.',
    },
  },
  en: {
    visibility: { public: 'Public', private: 'Private' },
    configure: {
      label: 'Connection name',
      category: 'Default category',
      visibility: 'Default visibility',
      save: 'Save defaults',
      failed: 'Could not save the defaults.',
    },
  },
} as const satisfies I18nFragment
