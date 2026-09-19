import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    page: {
      description:
        '말투마다 프로필과 학습 기록이 따로 쌓여요. 새 글은 기본 말투로 시작하고, 글마다 다른 말투를 고를 수 있어요.',
      active: '사용 중',
      deleted: '삭제된 말투 {{count}}개',
      deleted_one: '삭제된 말투 {{count}}개',
      deleted_other: '삭제된 말투 {{count}}개',
      deletedHelp:
        '글과 학습 기록은 그대로 남아 있어요. 복원하면 다시 고를 수 있고, 같은 이름의 말투가 이미 있으면 먼저 이름을 바꿔 주세요.',
    },
    loadFailed: '말투 목록을 불러오지 못했어요.',
  },
  en: {
    page: {
      description:
        'Each voice keeps its own profile and learning history. New posts start with the default, and you can choose a different voice for each post.',
      active: 'Active',
      deleted: '{{count}} deleted voices',
      deleted_one: '{{count}} deleted voice',
      deleted_other: '{{count}} deleted voices',
      deletedHelp:
        'Posts and learning history remain intact. Restore a voice to use it again; if its name is already in use, rename the other voice first.',
    },
    loadFailed: 'Could not load the voice list.',
  },
} as const satisfies I18nFragment
