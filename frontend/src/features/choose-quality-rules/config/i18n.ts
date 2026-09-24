import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16), under its own key group. Every line says
 *  what was counted in the published posts PostPilot holds, never what ticking improves (QUAL-19,
 *  QUAL-21), and the published count is stated as the account's, not as the posts a metric read.
 *  The plural pairs exist in both languages so the two key sets stay equal. */
export const i18n = {
  namespace: 'posts',
  ko: {
    qualityRules: {
      heading: '발행 글 점검',
      source: 'PostPilot에 저장된 발행 글에서 센 값이에요.',
      explain: '{{metric}} 설명',
      adds: '체크하면 다음 생성에 이 규칙이 더해져요:',
      saveFailed: '설정을 저장하지 못했어요. 다시 눌러 주세요.',
      loading: '발행 글을 세는 중이에요.',
      failed: '발행 글 점검을 불러오지 못했어요.',
      values: {
        repetition: '반복 {{value}}',
        relevance: '제목 관련성 {{value}}',
        blockTypes: '블록 종류 {{value}}',
      },
      what: {
        title_saturation:
          '최근 발행 글의 제목 가운데, 가장 많은 제목에 들어간 명사가 들어 있는 제목의 비율이에요.',
        cross_post_phrases:
          '한 글에서 8어절 이상 이어지는 부분이 최근 발행 글에도 그대로 있는 비율을, 글마다 센 값의 중간값이에요.',
        in_post_repetition:
          '글마다 본문 명사 중 가장 잦은 명사의 비중과, 제목의 명사가 본문에도 나오는 비율을 센 값의 중간값이에요.',
        composition: '글마다 쓴 블록 종류의 수를 센 값의 중간값이에요.',
      },
      why: {
        title_saturation_one:
          '발행한 글이 {{count}}편 있고, 지금 값은 {{value}}예요. 기준은 {{edge}} 이하예요.',
        title_saturation_other:
          '발행한 글이 {{count}}편 있고, 지금 값은 {{value}}예요. 기준은 {{edge}} 이하예요.',
        cross_post_phrases_one:
          '발행한 글이 {{count}}편 있고, 지금 값은 {{value}}예요. 기준은 {{edge}} 이하예요.',
        cross_post_phrases_other:
          '발행한 글이 {{count}}편 있고, 지금 값은 {{value}}예요. 기준은 {{edge}} 이하예요.',
        in_post_repetition_one:
          '발행한 글이 {{count}}편 있고, 지금 반복은 {{repetition}}, 제목 관련성은 {{relevance}}예요. 기준은 반복 {{repetitionEdge}} 이하, 제목 관련성 {{relevanceEdge}} 이상이에요.',
        in_post_repetition_other:
          '발행한 글이 {{count}}편 있고, 지금 반복은 {{repetition}}, 제목 관련성은 {{relevance}}예요. 기준은 반복 {{repetitionEdge}} 이하, 제목 관련성 {{relevanceEdge}} 이상이에요.',
        composition_one:
          '발행한 글이 {{count}}편 있고, 지금 블록 종류는 {{value}}가지예요. 기준은 {{edge}}가지 초과예요.',
        composition_other:
          '발행한 글이 {{count}}편 있고, 지금 블록 종류는 {{value}}가지예요. 기준은 {{edge}}가지 초과예요.',
      },
    },
  },
  en: {
    qualityRules: {
      heading: 'Published post check',
      source: 'Counted from the published posts PostPilot holds.',
      explain: 'About {{metric}}',
      adds: 'Ticking it adds this rule to the next generation:',
      saveFailed: 'Couldn’t save that. Please press it again.',
      loading: 'Counting your published posts…',
      failed: 'Couldn’t load the published post check.',
      values: {
        repetition: 'repetition {{value}}',
        relevance: 'title relevance {{value}}',
        blockTypes: 'block types {{value}}',
      },
      what: {
        title_saturation:
          'Among your recent published titles, the share that contain the noun found in the most titles.',
        cross_post_phrases:
          'The median, per post, of the share that stands in a run of eight or more words also found verbatim in your recent published posts.',
        in_post_repetition:
          'The medians, per post, of the most frequent noun’s share of the body’s nouns and of the title’s nouns that the body also contains.',
        composition: 'The median number of block types each post uses.',
      },
      why: {
        title_saturation_one:
          'You have {{count}} published post, and the value is {{value}}. The threshold is {{edge}} or less.',
        title_saturation_other:
          'You have {{count}} published posts, and the value is {{value}}. The threshold is {{edge}} or less.',
        cross_post_phrases_one:
          'You have {{count}} published post, and the value is {{value}}. The threshold is {{edge}} or less.',
        cross_post_phrases_other:
          'You have {{count}} published posts, and the value is {{value}}. The threshold is {{edge}} or less.',
        in_post_repetition_one:
          'You have {{count}} published post; repetition is {{repetition}} and title relevance {{relevance}}. The thresholds are repetition {{repetitionEdge}} or less and title relevance {{relevanceEdge}} or more.',
        in_post_repetition_other:
          'You have {{count}} published posts; repetition is {{repetition}} and title relevance {{relevance}}. The thresholds are repetition {{repetitionEdge}} or less and title relevance {{relevanceEdge}} or more.',
        composition_one:
          'You have {{count}} published post, and it uses {{value}} block types. The threshold is more than {{edge}}.',
        composition_other:
          'You have {{count}} published posts, and they use {{value}} block types. The threshold is more than {{edge}}.',
      },
    },
  },
} as const satisfies I18nFragment
