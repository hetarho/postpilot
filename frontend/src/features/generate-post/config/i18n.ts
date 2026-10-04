import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    storylineActions: {
      remake: '다시 만들기',
      write: '이 스토리로 글 쓰기',
      rewrite: '이 스토리로 다시 쓰기',
      request: '스토리라인 수정 요청',
      send: '스토리라인 수정 요청 보내기',
      remakeTitle: '스토리라인을 다시 만들까요?',
      remakeBody: '직접 고친 스토리라인이 새로 만든 것으로 바뀌어요.',
      rewriteTitle: '이 스토리로 다시 쓸까요?',
      rewriteBody: '직접 고친 글이 사라지고 이 스토리로 새로 쓰여요.',
      rewriteConfirm: '다시 쓰기',
    },
    noPhotos: {
      title: '사진 없이 만들까요?',
      body: '첨부된 사진이 없어요.',
      confirm: '사진 없이 만들기',
    },
  },
  en: {
    storylineActions: {
      remake: 'Make again',
      write: 'Write from this storyline',
      rewrite: 'Rewrite from this storyline',
      request: 'Ask to change the storyline',
      send: 'Send the storyline request',
      remakeTitle: 'Make the storyline again?',
      remakeBody: 'The storyline you edited is replaced by a newly made one.',
      rewriteTitle: 'Rewrite from this storyline?',
      rewriteBody: 'The post you edited is replaced by one written from this storyline.',
      rewriteConfirm: 'Rewrite',
    },
    noPhotos: {
      title: 'Continue without photos?',
      body: 'No photos are attached.',
      confirm: 'Continue without photos',
    },
  },
} as const satisfies I18nFragment
