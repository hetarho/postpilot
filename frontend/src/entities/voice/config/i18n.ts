import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    title: '말투',
    voiceLoadFailed: '말투를 불러오지 못했어요.',
    missing: '없는 말투예요.',
    deletedPrefix: '삭제된 말투',
    deletedRef: '삭제된 말투 · {{name}}',
    deletedAiReason: '삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.',
    learn: {
      label: '제목 (선택)',
      labelPlaceholder: '예: 제주 여행기',
      body: '내가 쓴 글',
      bodyPlaceholder: '기존에 쓴 글 한 편을 붙여 넣어 주세요',
      count: '{{count}} / {{min}}자',
      count_one: '{{count}} / {{min}}자',
      count_other: '{{count}} / {{min}}자',
      remaining: '{{count}}자 더 쓰면 학습할 수 있어요',
      remaining_one: '{{count}}자 더 쓰면 학습할 수 있어요',
      remaining_other: '{{count}}자 더 쓰면 학습할 수 있어요',
      selectModel: '모델을 선택하세요',
      action: '학습',
      pending: '학습을 시작하는 중',
      confirmTitle: '문체 규칙을 다시 쓸까요?',
      confirm: '다시 분석',
      confirmDescription:
        '재분석하면 현재 문체 규칙을 덮어씁니다. 직접 작성한 추가 규칙은 그대로 유지됩니다.',
      declaredLanguage: '{{language}}로 쓴 글만 붙여 넣어 주세요.',
    },
    deletedWarning:
      '삭제된 말투예요. 기록은 볼 수 있지만, 복원하기 전에는 배우거나 고칠 수 없어요.',
    warning: {
      deletedPost:
        '<voice>{{voice}}</voice>. 이 글은 읽고 직접 고치고 내보낼 수 있어요. AI 생성·수정·학습은 말투를 복원하거나 위에서 다른 말투로 바꾼 뒤에 할 수 있어요.',
      empty: '문체 프로필이 비어 있어요. 말투 탭에서 글 한 편을 학습시키면 내 문체로 나와요.',
      learn: '말투 학습하기',
    },
  },
  en: {
    title: 'Voices',
    voiceLoadFailed: 'Could not load the voice.',
    missing: 'This voice does not exist.',
    deletedPrefix: 'Deleted voice',
    deletedRef: 'Deleted voice · {{name}}',
    deletedAiReason: 'This voice has been deleted. Restore it or choose another voice.',
    learn: {
      label: 'Title (optional)',
      labelPlaceholder: 'For example: Jeju travel story',
      body: 'A post I wrote',
      bodyPlaceholder: 'Paste one of your existing posts',
      count: '{{count}} / {{min}} characters',
      count_one: '{{count}} / {{min}} character',
      count_other: '{{count}} / {{min}} characters',
      remaining: 'Write {{count}} more characters to start learning',
      remaining_one: 'Write {{count}} more character to start learning',
      remaining_other: 'Write {{count}} more characters to start learning',
      selectModel: 'Select a model',
      action: 'Learn',
      pending: 'Starting voice learning',
      confirmTitle: 'Rewrite the styleguide?',
      confirm: 'Analyze again',
      confirmDescription:
        'Reanalysis replaces the current styleguide. Additional rules you wrote manually stay unchanged.',
      declaredLanguage: 'Paste only writing in {{language}}.',
    },
    deletedWarning:
      'This voice has been deleted. You can view its history, but it cannot learn or be edited until it is restored.',
    warning: {
      deletedPost:
        '<voice>{{voice}}</voice>. You can still read, edit, and export this post. To generate, revise, or learn with AI, restore the voice or choose another one above.',
      empty:
        'The voice profile is empty. Teach it with one post in the Voice tab to generate in your style.',
      learn: 'Teach this voice',
    },
  },
} as const satisfies I18nFragment
