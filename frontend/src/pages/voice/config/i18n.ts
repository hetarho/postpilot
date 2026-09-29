import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    screens: {
      versionsDescription:
        '분석, 직접 수정, 규칙 반영은 모두 이 말투의 새 버전으로 쌓입니다. 복원해도 예전 기록은 지워지지 않아요.',
      materialsBlocked: '삭제된 말투에는 학습 글을 더할 수 없어요. 먼저 복원해 주세요.',
      toMaterials: '학습 글 모으기',
      analysisStatus: '문체 분석 상태',
      analysisStatusFailed: '문체 분석 상태를 확인하지 못했어요.',
      profileLoadFailed: '문체 프로필을 불러오지 못했어요.',
    },
  },
  en: {
    screens: {
      versionsDescription:
        'Analysis, manual edits, and applied rules each create a new version. Restoring never deletes earlier history.',
      materialsBlocked: 'A deleted voice takes no new writing. Restore it first.',
      toMaterials: 'Gather writing',
      analysisStatus: 'Voice analysis status',
      analysisStatusFailed: 'Could not check voice analysis status.',
      profileLoadFailed: 'Could not load the voice profile.',
    },
  },
} as const satisfies I18nFragment
