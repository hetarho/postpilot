import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    giftLink: {
      label: '선물 링크',
      copy: '링크 복사',
      copied: '복사했어요',
      copyFailed: '복사하지 못했어요. 링크를 직접 선택해 복사해 주세요.',
    },
  },
  en: {
    giftLink: {
      label: 'Gift link',
      copy: 'Copy link',
      copied: 'Copied',
      copyFailed: 'Could not copy. Select the link and copy it yourself.',
    },
  },
} as const satisfies I18nFragment
