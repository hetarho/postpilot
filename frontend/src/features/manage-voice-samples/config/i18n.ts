import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    samples: {
      title: '학습 글',
      empty: '아직 학습 글이 없어요.',
      post: '붙여 넣은 글',
      answer: '문항 답',
      photo: '사진',
      photoAlt: '답에 쓴 사진',
      loadFailed: '학습 글을 불러오지 못했어요.',
      deleteTitle: '학습 글을 삭제할까요?',
      deleteDescription: '삭제한 학습 글은 되돌릴 수 없어요.',
    },
  },
  en: {
    samples: {
      title: 'Writing',
      empty: 'No writing yet.',
      post: 'Pasted post',
      answer: 'Prompt answer',
      photo: 'Photo',
      photoAlt: 'The photo the answer was written on',
      loadFailed: 'Could not load this writing.',
      deleteTitle: 'Delete this writing?',
      deleteDescription: 'Deleted writing cannot be brought back.',
    },
  },
} as const satisfies I18nFragment
