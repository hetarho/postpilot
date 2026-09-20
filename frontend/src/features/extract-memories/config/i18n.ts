import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    memories: {
      extract: '기억으로 저장',
      extractNeedsContent: '글이 만들어진 뒤에 기억을 뽑을 수 있어요.',
      extractFailed: '기억을 뽑지 못했어요. 다시 시도해 주세요.',
      candidateTitle: '기억으로 저장할 사실',
      candidateHelp: '체크한 사실만 저장돼요. 저장하지 않은 후보는 남지 않아요.',
      candidateEmpty: '이 글에서 저장할 만한 사실을 찾지 못했어요. 나중에 다시 눌러도 돼요.',
      candidateSave: '{{count}}개 저장',
    },
  },
  en: {
    memories: {
      extract: 'Save as memories',
      extractNeedsContent: 'Memories can be extracted once the post has been written.',
      extractFailed: 'Could not extract memories. Try again.',
      candidateTitle: 'Facts to remember',
      candidateHelp: 'Only the checked ones are saved. The rest are discarded with this sheet.',
      candidateEmpty: 'Nothing worth keeping was found in this post. You can try again later.',
      candidateSave: 'Save {{count}}',
    },
  },
} as const satisfies I18nFragment
