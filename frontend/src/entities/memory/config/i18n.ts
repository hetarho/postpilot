import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `memories` namespace (ARCH-16): the five kind names and the two
 *  authoring fields, which the create sheet and the row edit both render. */
export const i18n = {
  namespace: 'memories',
  ko: {
    kind: {
      preference: '취향',
      persona: '설정',
      place: '장소',
      person: '인물',
      history: '이력',
    },
    field: {
      kind: '종류',
      tags: '태그',
      tagsPlaceholder: '연남동, 카페',
      tagsHelp: '쉼표로 구분해 최대 {{max}}개. 이 기억을 다시 쓸 글을 찾아낼 단어를 적어 주세요.',
      tagsTooMany: '태그는 {{max}}개까지예요. 지금 {{count}}개예요.',
    },
  },
  en: {
    kind: {
      preference: 'Preference',
      persona: 'Persona',
      place: 'Place',
      person: 'Person',
      history: 'History',
    },
    field: {
      kind: 'Kind',
      tags: 'Tags',
      tagsPlaceholder: 'yeonnam-dong, cafe',
      tagsHelp:
        'Comma separated, at most {{max}}. Use words that will show up in a post this fact belongs in.',
      tagsTooMany: 'At most {{max}} tags. There are {{count}} now.',
    },
  },
} as const satisfies I18nFragment
