import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `publishing` namespace (ARCH-16). */
export const i18n = {
  namespace: 'publishing',
  ko: {
    pair: {
      defaultLabel: '내 Mac',
      title: 'Mac 연결',
      description:
        '네이버 로그인은 Mac의 전용 브라우저에만 남습니다. 아래 코드를 Mac 설정 화면에 입력하세요.',
      label: '연결 이름',
      create: '연결 코드 만들기',
      code: '연결 코드',
      expires: '{{date}}까지 한 번만 사용할 수 있어요.',
      failed: '연결 코드를 만들지 못했어요.',
    },
  },
  en: {
    pair: {
      defaultLabel: 'My Mac',
      title: 'Connect a Mac',
      description:
        'Your Naver login stays only in the dedicated browser on your Mac. Enter the code below in the Mac settings screen.',
      label: 'Connection name',
      create: 'Create connection code',
      code: 'Connection code',
      expires: 'This code can be used once until {{date}}.',
      failed: 'Could not create a connection code.',
    },
  },
} as const satisfies I18nFragment
