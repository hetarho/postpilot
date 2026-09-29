import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    finalize: {
      title: '확정',
      already: '이 내용은 이미 확정했어요. 글 완성에서 내보낼 수 있습니다.',
      description: '다 다듬었다면 지금 내용을 확정해 주세요. 확정하면 글 완성으로 넘어갑니다.',
      failed: '글을 확정하지 못했어요.',
      goFinish: '글 완성으로 가기',
      action: '확정하기',
    },
  },
  en: {
    finalize: {
      title: 'Finalize',
      already: 'This content is already finalized. You can export it from Finish.',
      description: 'When you are done refining, finalize this content to continue to Finish.',
      failed: 'Could not finalize the post.',
      goFinish: 'Go to Finish',
      action: 'Finalize',
    },
  },
} as const satisfies I18nFragment
