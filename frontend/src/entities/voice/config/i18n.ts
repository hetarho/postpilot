import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    origin: {
      synthetic: 'AI가 만든 스타일',
      personal: '내 글에서 배운 말투',
      syntheticHelp:
        '아래 습관은 AI가 만든 가상 예문에서 센 값이에요. 내 답변을 추가하고 다시 분석하면 더 나답게 바꿀 수 있어요.',
    },
    title: '말투',
    noVoice: '말투 없음',
    voiceLoadFailed: '말투를 불러오지 못했어요.',
    missing: '없는 말투예요.',
    deletedPrefix: '삭제된 말투',
    deletedRef: '삭제된 말투 · {{name}}',
    deletedAiReason: '삭제된 말투예요. 말투를 복원하거나 다른 말투로 바꿔 주세요.',
    unmadeAiReason: '아직 만들지 않은 말투예요. 말투를 만들거나 다른 말투로 바꿔 주세요.',
    fingerprint: {
      unknown: '알 수 없음',
      label: {
        endings: '문장 끝',
        marks: '문장 끝 부호',
        emoji: '이모지와 자모',
        shape: '문장과 문단',
        openings: '여는 줄과 닫는 줄',
        adverbs: '자주 쓰는 말',
        person: '1인칭',
        headings: '소제목과 목록',
      },
      sentence: {
        endings:
          "문장의 {{da}}%를 '~다', {{haeyo}}%를 '~해요', {{seumnida}}%를 '~습니다'로 끝내요.",
        suffixes: '자주 쓰는 끝맺음은 {{suffixes}}예요.',
        marks:
          '문장의 {{exclaim}}%를 느낌표로, {{question}}%를 물음표로, {{tilde}}%를 물결표로 끝내요. 같은 부호를 겹쳐 쓰는 문장은 {{repeat}}%예요.',
        emoji:
          '100문장마다 이모지 {{emoji}}개, ㅎㅎ {{hh}}번, ㅋㅋ {{kk}}번, ㅠㅠ {{tears}}번 써요.',
        shapeOwnLine: '문장은 평균 {{average}}자, 문단마다 {{range}}문장이고 문장마다 줄을 바꿔요.',
        shapeRunOn:
          '문장은 평균 {{average}}자, 문단마다 {{range}}문장이고 여러 문장을 한 줄에 이어 써요.',
        openings: '여는 줄 {{lines}}',
        closings: '닫는 줄 {{lines}}',
        adverbs: '{{words}}',
        adverbRate: '{{word}} (100문장마다 {{count}}번)',
        adverbsNone: '눈에 띄게 반복하는 부사는 없어요.',
        person: "1인칭으로 '{{form}}'를 써요 (100문장마다 {{count}}번).",
        personNone: '1인칭을 거의 쓰지 않아요.',
        headings: '소제목의 {{emoji}}%가 이모지로 시작하고 {{question}}%가 질문형이에요.',
        lists: "목록은 '{{marker}}'로 써요.",
      },
    },
    readiness: {
      questionProgress: '질문 {{answered}} / {{required}}개',
      questionsMore: '질문 {{count}}개만 더 답하면 돼요.',
      coverage: '{{parts}} 답변도 하나 알려 주세요.',

      label: '나의 말투 찾기',
      value: '{{percent}}% 확보',
      ready: '이제 말투를 만들 수 있어요',
      more: '{{sentences}}문장이 더 필요해요.',
      missing: '아직 없는 부분: {{parts}}',
      part: { opening: '글머리', description: '본문', closing: '마무리' },
    },
    deletedWarning:
      '삭제된 말투예요. 기록은 볼 수 있지만, 복원하기 전에는 배우거나 고칠 수 없어요.',
    warning: {
      deletedPost:
        '<voice>{{voice}}</voice>. 이 글은 읽고 직접 고치고 내보낼 수 있어요. AI 생성·수정·학습은 말투를 복원하거나 위에서 다른 말투로 바꾼 뒤에 할 수 있어요.',
      learn: '말투 학습하기',
    },
  },
  en: {
    origin: {
      synthetic: 'AI-created style',
      personal: 'Learned from my writing',
      syntheticHelp:
        'These habits were measured from an AI-created fictional sample. Add your own answers and explicitly analyze again to make it personal.',
    },
    title: 'Voices',
    noVoice: 'No voice',
    voiceLoadFailed: 'Could not load the voice.',
    missing: 'This voice does not exist.',
    deletedPrefix: 'Deleted voice',
    deletedRef: 'Deleted voice · {{name}}',
    deletedAiReason: 'This voice has been deleted. Restore it or choose another voice.',
    unmadeAiReason: "This voice isn't made yet. Make it or pick another voice.",
    fingerprint: {
      unknown: 'Unknown',
      label: {
        endings: 'Sentence endings',
        marks: 'End marks',
        emoji: 'Emoji and jamo',
        shape: 'Sentences and paragraphs',
        openings: 'Openings and closings',
        adverbs: 'Frequent words',
        person: 'First person',
        headings: 'Headings and lists',
      },
      sentence: {
        endings:
          "{{da}}% of sentences end in '~다', {{haeyo}}% in '~해요' and {{seumnida}}% in '~습니다'.",
        suffixes: 'Frequent endings: {{suffixes}}.',
        marks:
          '{{exclaim}}% of sentences end with an exclamation mark, {{question}}% with a question mark and {{tilde}}% with a tilde; {{repeat}}% double a mark.',
        emoji: 'Per 100 sentences: {{emoji}} emoji, ㅎㅎ {{hh}}, ㅋㅋ {{kk}}, ㅠㅠ {{tears}}.',
        shapeOwnLine:
          'Sentences average {{average}} characters, {{range}} to a paragraph, each on its own line.',
        shapeRunOn:
          'Sentences average {{average}} characters, {{range}} to a paragraph, run together on one line.',
        openings: 'Opens with {{lines}}',
        closings: 'closes with {{lines}}',
        adverbs: '{{words}}',
        adverbRate: '{{word}} ({{count}} per 100 sentences)',
        adverbsNone: 'No adverb repeats noticeably.',
        person: "Uses '{{form}}' ({{count}} per 100 sentences).",
        personNone: 'Hardly uses the first person.',
        headings: '{{emoji}}% of headings start with an emoji and {{question}}% are questions.',
        lists: "Lists use '{{marker}}'.",
      },
    },
    readiness: {
      questionProgress: '{{answered}} / {{required}} questions',
      questionsMore: 'Answer {{count}} more questions.',
      coverage: 'Add an answer for {{parts}} too.',

      label: 'What the voice needs',
      value: '{{percent}}% there',
      ready: 'You can make the voice now',
      more: 'Sentences still needed: {{sentences}}.',
      missing: 'Still missing: {{parts}}',
      part: { opening: 'Opening', description: 'Body', closing: 'Closing' },
    },
    deletedWarning:
      'This voice has been deleted. You can view its history, but it cannot learn or be edited until it is restored.',
    warning: {
      deletedPost:
        '<voice>{{voice}}</voice>. You can still read, edit, and export this post. To generate, revise, or learn with AI, restore the voice or choose another one above.',
      learn: 'Teach this voice',
    },
  },
} as const satisfies I18nFragment
