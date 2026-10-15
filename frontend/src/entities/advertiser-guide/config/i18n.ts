import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16): the advertiser guide fields. */
export const i18n = {
  namespace: 'posts',
  ko: {
    advertiserGuide: {
      campaign: '체험단 글',
      campaignHelp:
        '켜 두면 광고주 가이드가 글에 반영되고, 글 맨 위에 대가 표시를 넣을 자리가 생겨요.',
      text: '광고주 가이드',
      placeholder: '체험단 사이트에서 받은 가이드를 그대로 붙여넣으세요.',
      textHelp:
        '광고주가 요청한 내용은 내 경험이 아니라 광고주의 자료로 글에 자연스럽게 담겨요. 겪지 않은 칭찬은 쓰지 않아요.',
      count: '{{count}} / {{max}}자',
      count_one: '{{count}} / {{max}}자',
      count_other: '{{count}} / {{max}}자',
      counted: '셀 수 있는 항목',
      keywords: '필수 키워드',
      keywordPlaceholder: '키워드를 쓰고 Enter',
      hashtags: '필수 해시태그',
      hashtagPlaceholder: '해시태그를 쓰고 Enter',
      minChars: '최소 글자 수',
      minPhotos: '최소 사진 수',
      notAsked: '요청 없음',
      remove: '{{item}} 지우기',
      tooMany: '{{max}}개까지 넣을 수 있어요.',
      repeated: '이미 넣은 항목이에요.',
      outOfRange: '{{min}}부터 {{max}}까지 넣을 수 있어요.',
    },
  },
  en: {
    advertiserGuide: {
      campaign: 'Sponsored review',
      campaignHelp:
        'While on, the advertiser guide shapes the post and the top of the post keeps a place for the disclosure.',
      text: 'Advertiser guide',
      placeholder: 'Paste the guide the campaign site gave you, as it is.',
      textHelp:
        'What the advertiser asks for is woven in as the advertiser’s material, not as your experience. Praise you did not give is never written.',
      count: '{{count}} / {{max}} characters',
      count_one: '{{count}} / {{max}} character',
      count_other: '{{count}} / {{max}} characters',
      counted: 'Countable items',
      keywords: 'Required keywords',
      keywordPlaceholder: 'Type a keyword and press Enter',
      hashtags: 'Required hashtags',
      hashtagPlaceholder: 'Type a hashtag and press Enter',
      minChars: 'Minimum characters',
      minPhotos: 'Minimum photos',
      notAsked: 'Not asked',
      remove: 'Remove {{item}}',
      tooMany: 'You can add up to {{max}}.',
      repeated: 'Already added.',
      outOfRange: 'Enter a number from {{min}} to {{max}}.',
    },
  },
} as const satisfies I18nFragment
