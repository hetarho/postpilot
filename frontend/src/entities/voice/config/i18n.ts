import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    title: '말투',
    noVoice: '말투 없음',
    voiceLoadFailed: '말투를 불러오지 못했어요.',
    missing: '없는 말투예요.',
    deletedPrefix: '삭제된 말투',
    deletedRef: '삭제된 말투 · {{name}}',
    deletedAiReason: '삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.',
    unmadeAiReason: '아직 만들지 않은 말투예요. 말투를 만들거나 다른 말투로 바꿔 주세요.',
    readiness: {
      label: '말투 학습에 필요한 정보',
      value: '{{percent}}% 확보',
      ready: '이제 말투를 만들 수 있어요',
      missing: '아직 없는 부분: {{parts}}',
      part: { opening: '글머리', description: '본문', closing: '마무리' },
    },
    deletedWarning:
      '삭제된 말투예요. 기록은 볼 수 있지만, 복원하기 전에는 배우거나 고칠 수 없어요.',
    warning: {
      deletedPost:
        '<voice>{{voice}}</voice>. 이 글은 읽고 직접 고치고 내보낼 수 있어요. AI 생성·수정·학습은 말투를 복원하거나 위에서 다른 말투로 바꾼 뒤에 할 수 있어요.',
      learn: '말투 학습하기',
    },
  },
  en: {
    title: 'Voices',
    noVoice: 'No voice',
    voiceLoadFailed: 'Could not load the voice.',
    missing: 'This voice does not exist.',
    deletedPrefix: 'Deleted voice',
    deletedRef: 'Deleted voice · {{name}}',
    deletedAiReason: 'This voice has been deleted. Restore it or choose another voice.',
    unmadeAiReason: "This voice isn't made yet. Make it or pick another voice.",
    readiness: {
      label: 'What the voice needs',
      value: '{{percent}}% there',
      ready: 'You can make the voice now',
      missing: 'Still missing: {{parts}}',
      part: { opening: 'Opening', description: 'Body', closing: 'Closing' },
    },
    deletedWarning:
      'This voice has been deleted. You can view its history, but it cannot learn or be edited until it is restored.',
    warning: {
      deletedPost:
        '<voice>{{voice}}</voice>. You can still read, edit, and export this post. To generate, revise, or learn with AI, restore the voice or choose another one above.',
      learn: 'Teach this voice',
    },
  },
} as const satisfies I18nFragment
