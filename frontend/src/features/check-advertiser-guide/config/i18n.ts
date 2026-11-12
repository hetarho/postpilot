import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16): ③'s pre-submission checklist. */
export const i18n = {
  namespace: 'posts',
  ko: {
    checklist: {
      title: '제출 전 체크리스트',
      help: '광고주 가이드를 지금 글과 비교했어요. 참고용이라 복사, 완성, 발행은 그대로 할 수 있어요.',
      counted: '글에 꼭 담을 내용',
      keyword: '필수 키워드 “{{keyword}}”',
      keywordMet: '들어 있어요',
      keywordMissing: '아직 없어요',
      characters: '글자 수 {{withSpaces}}자 (공백 제외 {{withoutSpaces}}자), 최소 {{minimum}}자',
      charactersMet: '충분해요',
      charactersShort: '공백 제외 {{short}}자 더 필요해요',
      photos: '사진 {{count}}장, 최소 {{minimum}}장',
      photosMet: '충분해요',
      photosShort: '{{short}}장 더 필요해요',
      hashtag: '필수 해시태그 #{{hashtag}}',
      hashtagMet: '태그에 있어요',
      hashtagMissing: '태그에 없어요',
      lines: '광고주 요청 확인',
      linesHelp: '나머지 요청은 글을 읽고 직접 확인해 주세요.',
      clip: {
        help: '광고주 가이드를 확정한 영상과 업로드 문구에 비교했어요. 참고용이라 다운로드, 복사, 완료 표시는 그대로 할 수 있어요.',
        counted: '영상에 꼭 담을 내용',
        hashtagMet: '업로드 문구 태그에 있어요',
        hashtagMissing: '업로드 문구 태그에 없어요',
        linesHelp: '나머지 요청은 영상을 보고 직접 확인해 주세요.',
      },
    },
  },
  en: {
    checklist: {
      title: 'Pre-submission checklist',
      help: 'The advertiser guide compared with the post as it is now. It is advisory: copying, finishing and publishing stay available.',
      counted: 'What your post needs',
      keyword: 'Required keyword “{{keyword}}”',
      keywordMet: 'Included',
      keywordMissing: 'Not yet included',
      characters:
        '{{withSpaces}} characters ({{withoutSpaces}} without spaces), at least {{minimum}}',
      charactersMet: 'Enough',
      charactersShort: '{{short}} more characters without spaces needed',
      photos: '{{count}} photos, at least {{minimum}}',
      photosMet: 'Enough',
      photosShort: '{{short}} more photos needed',
      hashtag: 'Required hashtag #{{hashtag}}',
      hashtagMet: 'In the tags',
      hashtagMissing: 'Not in the tags',
      lines: 'Advertiser requests to confirm',
      linesHelp: 'Read the post and check the remaining requests yourself.',
      clip: {
        help: 'The advertiser guide compared with the confirmed video and its upload text. It is advisory: downloading, copying and finishing stay available.',
        counted: 'What your video needs',
        hashtagMet: 'In the upload text tags',
        hashtagMissing: 'Not in the upload text tags',
        linesHelp: 'Watch the video and check the remaining requests yourself.',
      },
    },
  },
} as const satisfies I18nFragment
