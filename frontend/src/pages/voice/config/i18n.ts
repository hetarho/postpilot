import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    screens: {
      profileDescription:
        '이 말투로 완성한 글을 직접 확정할 때마다 종결어미와 리듬을 한 편씩 배웁니다. 아무 글도 없어도 첫 글을 바로 만들 수 있어요.',
      versionsDescription:
        '분석, 직접 수정, 규칙 반영은 모두 이 말투의 새 버전으로 쌓입니다. 복원해도 예전 기록은 지워지지 않아요.',
      importTitle: '기존 글 가져오기',
      importDescription:
        '이미 쓴 글을 이 말투에 가져오고 싶은 경우에만 사용하세요. 첫 글 생성에는 필요하지 않고, 다른 말투의 글은 가져올 수 없어요.',
      importBlocked: '삭제된 말투에는 글을 가져올 수 없어요. 먼저 복원해 주세요.',
      analysisStatus: '문체 분석 상태',
      analysisStatusFailed: '문체 분석 상태를 확인하지 못했어요.',
      profileLoadFailed: '문체 프로필을 불러오지 못했어요.',
    },
  },
  en: {
    screens: {
      profileDescription:
        'Each finalized post teaches this voice more about endings and rhythm. You can generate the first post even with an empty profile.',
      versionsDescription:
        'Analysis, manual edits, and applied rules each create a new version. Restoring never deletes earlier history.',
      importTitle: 'Import existing posts',
      importDescription:
        'Use this only to bring writing you already completed into this voice. It is not needed for a first post, and posts from another voice cannot be imported.',
      importBlocked: 'Posts cannot be imported into a deleted voice. Restore it first.',
      analysisStatus: 'Voice analysis status',
      analysisStatusFailed: 'Could not check voice analysis status.',
      profileLoadFailed: 'Could not load the voice profile.',
    },
  },
} as const satisfies I18nFragment
