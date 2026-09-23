import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16), under its own key group. A value line
 *  counts this post's own text, and M2's names the account's published posts as PostPilot stores
 *  them: nothing here promises exposure (QUAL constraints) or implies a Naver post was read
 *  (QUAL-19). The plural pairs exist in both languages so the two key sets stay equal. */
export const i18n = {
  namespace: 'posts',
  ko: {
    quality: {
      metric: {
        title_saturation: '제목 도배율',
        cross_post_phrases: '글 간 고정 문구',
        in_post_repetition: '글 안 반복과 제목 관련성',
        composition: '분량·구성',
      },
      verdict: { over_band: '주의', within_band: '양호' },
      absent: '측정할 수 없어요',
      belowMinimum: '발행한 글이 {{minimum}}편 이상이면 비교해요. 지금은 {{count}}편이에요.',
      bandsOwn: '배지 기준은 PostPilot이 정한 값이에요.',
      band: {
        atMost: '기준 {{edge}} 이하',
        atLeast: '기준 {{edge}} 이상',
        above: '기준 {{edge}} 초과',
      },
      value: {
        sharedWithPublished: '저장된 발행 글과 겹치는 부분',
        topNounShare: '본문 명사 중 가장 잦은 명사',
        titleNounsInBody: '본문에도 나오는 제목 명사',
        charCount: '글자 수',
        photoCount: '사진',
        blockTypes: '블록 종류',
        sentenceLength: '평균 문장 길이',
      },
      amount: {
        chars: '{{value}}자',
        photos: '{{value}}장',
        types: '{{value}}가지',
        sentenceChars_one: '{{value}}자',
        sentenceChars_other: '{{value}}자',
        sentenceWords_one: '{{value}}단어',
        sentenceWords_other: '{{value}}단어',
      },
      post: {
        heading: '이 글의 측정값',
        loading: '측정하는 중이에요.',
        failed: '측정값을 불러오지 못했어요.',
      },
    },
  },
  en: {
    quality: {
      metric: {
        title_saturation: 'Title saturation',
        cross_post_phrases: 'Phrases shared across posts',
        in_post_repetition: 'Repetition and title relevance',
        composition: 'Length and structure',
      },
      verdict: { over_band: 'Caution', within_band: 'Good' },
      absent: 'Can’t be measured',
      belowMinimum: 'Compared once {{minimum}} posts are published. You have {{count}} now.',
      bandsOwn: 'The badge thresholds are PostPilot’s own.',
      band: {
        atMost: 'target {{edge}} or less',
        atLeast: 'target {{edge}} or more',
        above: 'target above {{edge}}',
      },
      value: {
        sharedWithPublished: 'Shared with your published posts stored here',
        topNounShare: 'Most frequent noun among the body’s nouns',
        titleNounsInBody: 'Title nouns also in the body',
        charCount: 'Characters',
        photoCount: 'Photos',
        blockTypes: 'Block types',
        sentenceLength: 'Average sentence length',
      },
      amount: {
        chars: '{{value}}',
        photos: '{{value}}',
        types: '{{value}}',
        sentenceChars_one: '{{value}} character',
        sentenceChars_other: '{{value}} characters',
        sentenceWords_one: '{{value}} word',
        sentenceWords_other: '{{value}} words',
      },
      post: {
        heading: 'This post’s measurements',
        loading: 'Measuring…',
        failed: 'Couldn’t load the measurements.',
      },
    },
  },
} as const satisfies I18nFragment
