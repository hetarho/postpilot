import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    screens: {
      materialsBlocked: '삭제된 말투에는 학습 글을 더할 수 없어요. 먼저 복원해 주세요.',
      toMaterials: '학습 글 모으기',
      analysisStatus: '문체 분석 상태',
      analysisStatusFailed: '문체 분석 상태를 확인하지 못했어요.',
      profileLoadFailed: '말투를 불러오지 못했어요.',
    },
    analysis: {
      counted: '숫자로 본 습관',
      ai: 'AI가 읽은 인상',
      impression: '전체 인상',
      tics: '말버릇',
      tic: '‘{{phrase}}’ — {{when}}',
      phrases: '이 사람만의 표현',
      noticeAdded: '새 학습 글 {{count}}편',
      noticeAdded_one: '새 학습 글 {{count}}편',
      noticeAdded_other: '새 학습 글 {{count}}편',
      noticeChanged: '학습 글이 바뀌었어요',
    },
  },
  en: {
    screens: {
      materialsBlocked: 'A deleted voice takes no new writing. Restore it first.',
      toMaterials: 'Gather writing',
      analysisStatus: 'Voice analysis status',
      analysisStatusFailed: 'Could not check voice analysis status.',
      profileLoadFailed: 'Could not load the voice.',
    },
    analysis: {
      counted: 'Counted habits',
      ai: 'What the AI read',
      impression: 'Overall impression',
      tics: 'Verbal tics',
      tic: '‘{{phrase}}’ — {{when}}',
      phrases: 'Signature phrases',
      noticeAdded: '{{count}} new writings',
      noticeAdded_one: '{{count}} new writing',
      noticeAdded_other: '{{count}} new writings',
      noticeChanged: 'The writing changed',
    },
  },
} as const satisfies I18nFragment
