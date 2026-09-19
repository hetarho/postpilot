import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    comparison: {
      loading: '비교 결과를 불러오는 중…',
      loadFailed: '비교 결과를 불러오지 못했어요.',
      wrongVoice: '이 비교는 다른 말투의 기록이에요.',
      back: '← 대조 규칙',
      title: '규칙 블라인드 비교',
      description: '두 결과는 같은 입력으로 만들었고 선택 전에는 규칙을 적용한 쪽을 숨깁니다.',
      retry: '실패한 후보 다시 만들기',
      actionAria: '후보 전환과 선택',
      selectAria: '선택할 후보',
      applied: '선택을 반영했어요.',
      prefer: '이 글이 더 나아요',
    },
  },
  en: {
    comparison: {
      loading: 'Loading comparison results…',
      loadFailed: 'Could not load comparison results.',
      wrongVoice: 'This comparison belongs to another voice.',
      back: '← Contrast rules',
      title: 'Blind rule comparison',
      description:
        'Both results use the same input. The side using the rule stays hidden until you choose.',
      retry: 'Regenerate failed candidate',
      actionAria: 'Switch and choose candidates',
      selectAria: 'Candidate to select',
      applied: 'Your choice was applied.',
      prefer: 'This version is better',
    },
  },
} as const satisfies I18nFragment
