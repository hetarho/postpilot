import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16): the fingerprint comparison's wording,
 *  shared by ②, 검증 and 말투 반영 비교 (VOICE-62). Item names are the entity's. */
export const i18n = {
  namespace: 'voices',
  ko: {
    comparison: {
      headline: '내 말투 {{voice}} · {{label}} {{text}}',
      facet: {
        endings: {
          da: "'~다'",
          haeyo: "'~해요'",
          seumnida: "'~습니다'",
          other: '그 밖의 끝',
          suffixes: '자주 쓰는 끝맺음',
        },
        marks: {
          exclaim: '느낌표',
          question: '물음표',
          tilde: '물결표',
          ellipsis: '말줄임표',
          period: '마침표',
          none: '부호 없이 끝남',
          repeat: '부호를 겹쳐 씀',
        },
        emoji: { emoji: '이모지', hh: 'ㅎㅎ', kk: 'ㅋㅋ', tears: 'ㅠㅠ' },
        shape: { average: '문장 길이', paragraph: '문단 크기', line_break: '문장마다 줄바꿈' },
        openings: { openings: '여는 줄', closings: '닫는 줄' },
        adverbs: { word: "'{{word}}'" },
        person: { jeo: "'저'", uri: "'우리'", na: "'나'", dominant: '주로 쓰는 1인칭' },
        headings: {
          emoji: '이모지로 여는 소제목',
          question: '질문형 소제목',
          numbered: '번호 붙은 소제목',
          list: '목록 줄',
          marker: '목록 기호',
        },
      },
      unit: {
        share: '{{value}}%',
        per_hundred: '100문장에 {{value}}번',
        chars: '{{value}}자',
        sentences: '{{value}}문장',
        none: '없음',
      },
      post: {
        heading: '말투 지문',
        textLabel: '이 글',
        failed: '말투 지문을 불러오지 못했어요.',
      },
    },
  },
  en: {
    comparison: {
      headline: 'Your voice {{voice}} · {{label}} {{text}}',
      facet: {
        endings: {
          da: "'~다'",
          haeyo: "'~해요'",
          seumnida: "'~습니다'",
          other: 'Other endings',
          suffixes: 'Frequent endings',
        },
        marks: {
          exclaim: 'Exclamation mark',
          question: 'Question mark',
          tilde: 'Tilde',
          ellipsis: 'Ellipsis',
          period: 'Full stop',
          none: 'No mark',
          repeat: 'Doubled marks',
        },
        emoji: { emoji: 'Emoji', hh: 'ㅎㅎ', kk: 'ㅋㅋ', tears: 'ㅠㅠ' },
        shape: {
          average: 'Sentence length',
          paragraph: 'Paragraph size',
          line_break: 'One sentence per line',
        },
        openings: { openings: 'Opening lines', closings: 'Closing lines' },
        adverbs: { word: "'{{word}}'" },
        person: { jeo: "'저'", uri: "'우리'", na: "'나'", dominant: 'Main first person' },
        headings: {
          emoji: 'Headings opening with an emoji',
          question: 'Question headings',
          numbered: 'Numbered headings',
          list: 'List lines',
          marker: 'List marker',
        },
      },
      unit: {
        share: '{{value}}%',
        per_hundred: '{{value}} per 100 sentences',
        chars: '{{value}} characters',
        sentences: '{{value}} sentences',
        none: 'None',
      },
      post: {
        heading: 'Voice fingerprint',
        textLabel: 'This post',
        failed: 'Could not load the voice fingerprint.',
      },
    },
  },
} as const satisfies I18nFragment
