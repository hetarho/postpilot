import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16): ③'s pre-submission checklist. */
export const i18n = {
  namespace: 'posts',
  ko: {
    checklist: {
      title: '제출 전 체크리스트',
      help: '광고주 가이드를 지금 글과 비교했어요. 참고용이라 복사, 완성, 발행은 그대로 할 수 있어요.',
      counted: '셀 수 있는 항목',
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
      linesHelp: '셀 수 없는 요청은 글을 읽고 직접 체크하세요.',
    },
  },
  en: {
    checklist: {
      title: 'Pre-submission checklist',
      help: 'The advertiser guide compared with the post as it is now. It is advisory: copying, finishing and publishing stay available.',
      counted: 'Countable items',
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
      linesHelp: 'Read the post and tick the requests that cannot be counted.',
    },
  },
} as const satisfies I18nFragment
