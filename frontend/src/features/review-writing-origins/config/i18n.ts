import type { I18nFragment } from '@/shared/lib'

export const i18n = {
  namespace: 'posts',
  ko: {
    originReview: {
      toggle: '출처 보기',
      find: '문구 출처 찾기',
      back: '문구 목록으로',
      pickerHelp: '문구를 선택하면 이 결과에 연결된 출처가 열립니다.',
      pickerPhrase: '{{field}} · {{category}} · {{phrase}}',
      legend: '의미의 출처',
      category: {
        owner_input: '직접 입력 기반',
        photo_interpretation: '사진에서 추론',
        ai_added: 'AI가 보탠 내용',
      },
      help: '문구를 선택하면 의미의 출처를 볼 수 있습니다. 출처는 사실 여부나 작성자를 판정하지 않습니다.',
      unconfirmedHelp:
        '출처 미확인: 현재 글에 연결된 출처를 확인할 수 없는 문구입니다. 글 수정과 내보내기는 그대로 이용할 수 있습니다.',
      phrase: '{{category}} 출처 보기: {{phrase}}',
      unconfirmed: '출처 미확인',
      detail: '문구의 출처',
      alt: '대체 텍스트 출처',
      field: {
        title: '제목',
        summary: '요약',
        tag: '태그 {{number}}',
        block_content: '본문 {{number}}',
        block_item: '목록 {{number}} · 항목 {{item}}',
        block_alt: '대체 텍스트 {{number}}',
        block_caption: '설명 {{number}}',
      },
      source: {
        memo: '직접 입력한 메모',
        template_answer: '직접 입력한 답변',
        owner_edit: '직접 입력한 사실',
        memory: '사용하도록 선택한 기억',
        visual_observation: '이 결과에 사용된 시각 관찰',
        ai_proposal: 'AI 제안 근거',
        literal_text: '제공된 원문',
      },
      aiWithoutBasis:
        '제공된 입력이나 시각 관찰 외에 AI가 보탠 의미입니다. 별도의 제안 근거는 기록되지 않았습니다.',
      missing:
        '이 문구에 연결된 출처가 기록되지 않았거나 현재 글과 맞지 않습니다. 현재 메모에서 과거 출처를 추정하지 않습니다.',
      pending:
        '수정한 글과 출처가 저장 결과에 맞춰지는 중입니다. 현재 문구의 출처는 아직 확인되지 않았습니다.',
      stale: '출처가 다른 글 결과에 연결되어 있어 현재 문구의 출처로 표시할 수 없습니다.',
      unavailable:
        '이 문구에 연결된 원본이 삭제되었거나 이용할 수 없습니다. 다른 원본으로 출처를 바꾸지 않습니다.',
    },
  },
  en: {
    originReview: {
      toggle: 'Show origins',
      find: 'Find phrase origins',
      back: 'Back to phrases',
      pickerHelp: 'Choose a phrase to open the sources tied to this result.',
      pickerPhrase: '{{field}} · {{category}} · {{phrase}}',
      legend: 'Origins of meaning',
      category: {
        owner_input: 'Based on your input',
        photo_interpretation: 'Inferred from photos',
        ai_added: 'Added by AI',
      },
      help: 'Select a phrase to inspect its origin of meaning. Origins do not verify facts or identify the author.',
      unconfirmedHelp:
        'Unconfirmed origin: a phrase whose source cannot be checked against the current writing. Editing and exporting remain available.',
      phrase: 'Show {{category}} origin: {{phrase}}',
      unconfirmed: 'Unconfirmed origin',
      detail: 'Phrase origin',
      alt: 'Alt text origins',
      field: {
        title: 'Title',
        summary: 'Summary',
        tag: 'Tag {{number}}',
        block_content: 'Body {{number}}',
        block_item: 'List {{number}} · item {{item}}',
        block_alt: 'Alt text {{number}}',
        block_caption: 'Caption {{number}}',
      },
      source: {
        memo: 'Your supplied memo',
        template_answer: 'Your supplied answer',
        owner_edit: 'Your supplied fact',
        memory: 'Memory selected for use',
        visual_observation: 'Visual observation used for this result',
        ai_proposal: 'AI proposal basis',
        literal_text: 'Supplied original text',
      },
      aiWithoutBasis:
        'AI added this meaning beyond the supplied input and visual observations. No separate proposal basis was recorded.',
      missing:
        'No source was recorded for this phrase, or it does not match the current writing. Past origins are not inferred from the current memo.',
      pending:
        'The edited writing and its origins are waiting for an aligned save result. This phrase’s origin is not yet confirmed.',
      stale:
        'The source belongs to a different writing result and cannot be shown as the origin of this phrase.',
      unavailable:
        'The original linked to this phrase was deleted or is unavailable. Another original is not substituted for it.',
    },
  },
} as const satisfies I18nFragment
