import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `templates` namespace (ARCH-16). */
export const i18n = {
  namespace: 'templates',
  ko: {
    title: '템플릿',
    loadFailed: '템플릿 목록을 불러오지 못했어요.',
    noTemplate: '없음',
    emptyDescription: '설명 없음',
    composition: {
      fixInSource: '원문에서 고치기',
      add: '블록 추가',
      empty: '위에서 블록을 더해 글의 순서를 짜 주세요.',
      insertHere: '여기에 추가돼요',
      repeatEmpty: '이 반복 안에 아직 아무것도 없어요.',
      repeatHelp:
        '첨부한 사진 개수만큼 안쪽 블록이 되풀이돼요. 한 번 되풀이할 때 사진 {{count}}장을 씁니다.',
      unreadable:
        '이 템플릿의 구성을 읽을 수 없어요. 원문에서 직접 고치거나, 구성을 비우고 다시 만들 수 있어요.',
      clearAndRestart: '구성 비우고 다시 만들기',
      // The title area's own composition (TMPL-50): a toolbar the page tests can tell from the
      // body's, and an unreadable state that names the title rather than the post's shape.
      titleArea: {
        add: '제목에 추가',
        empty: '위에서 블록을 더해 제목을 짜 주세요.',
        unreadable:
          '제목 형식을 읽을 수 없어요. 원문에서 직접 고치거나, 비우고 다시 만들 수 있어요.',
        clearAndRestart: '제목 비우고 다시 만들기',
      },
      summary: {
        photo: '사진 {{count}}장',
        photoRow: '사진 {{count}}장 가로로',
      },
      placeholder: {
        write: '무엇을 쓸지 적어 주세요',
        text: '들어갈 문구를 적어 주세요',
        note: 'AI에게 남길 말을 적어 주세요',
        photo: '첨부한 사진이 들어갑니다',
        repeat: '사진마다 되풀이',
      },
    },
    // The one surface where this app's grammar is visible (TEMPLATE-26).
    source: {
      label: '원문',
      titleAreaLabel: '제목 원문',
      copy: '원문 복사',
      copyGuide: '형식 안내 복사',
      copied: '복사했어요',
      manualCopy: '복사가 막혀 있어요. 선택된 내용을 길게 눌러 복사해 주세요.',
      showGuide: '형식 안내 보기',
      error: '{{line}}번째 줄: {{reason}}',
      guide: `아래 형식으로 블로그 글 템플릿의 본문을 하나 작성해 주세요.

[템플릿이 하는 일]
템플릿은 글의 뼈대입니다. 글의 순서와 어디에 무엇이 들어갈지를 정하고, 문체나 어휘는 정하지 않습니다.

[쓸 수 있는 표기 여섯 가지]
- 그냥 쓴 문장: 글에 그대로 나옵니다.
- <write>무엇을 쓸지</write>: AI가 그 자리에 지시대로 글을 씁니다. 지시문 자체는 글에 나오지 않습니다.
- <slot kind="photo"/>: 첨부한 사진이 한 장 들어갑니다. <slot kind="photo" count="2"/>처럼 count를 주면 그만큼 한 줄에 나란히 놓입니다.
- <repeat each="photo">…</repeat>: 안쪽 내용이 사진 수만큼 되풀이됩니다.
- <note>AI에게만 하는 말</note>: AI만 읽고 글에는 나오지 않습니다.
- <ask label="입력란 제목"/>: 글을 쓸 때 사용자가 직접 입력한 내용이 그 자리에 그대로 들어갑니다.
- <ask label="입력란 제목">무엇을 쓸지</ask>: 사용자가 입력한 내용만 근거로 AI가 그 자리에 글을 씁니다.

[지켜야 할 규칙]
- write, note, repeat는 반드시 닫아야 합니다.
- slot은 <slot …/>처럼 스스로 닫고, kind는 photo만 쓸 수 있습니다.
- count는 1에서 {{photoRowMax}} 사이의 정수입니다. 없으면 1장입니다.
- repeat는 each="photo"만 받고, repeat 안에 repeat를 넣을 수 없습니다.
- write와 note는 비워 둘 수 없습니다.
- ask는 label이 반드시 있어야 하고, 한 본문 안에서 label이 겹치면 안 됩니다. repeat 안에는 넣을 수 없고, 한 본문에 최대 {{askMax}}개까지입니다.
- ask는 사용자가 매번 알려줘야 하는 것(별점, 방문일, 가격처럼 AI가 알 수 없는 사실)에만 쓰세요.
- 위 여섯 가지 말고 다른 태그를 쓰면 저장되지 않습니다.
- 문장 안에 <로 시작하는 글자를 그대로 쓰려면 &lt;로 적어 주세요.
- 본문 전체는 {{bodyMax}}자를 넘을 수 없습니다.

[예시]
{{example}}

[답변 방식]
설명이나 코드 블록 없이 본문만 보내 주세요. 글은 제가 쓰는 언어로 써 주세요.`,
    },
    builder: {
      palette: {
        write: 'AI가 쓰는 글',
        writeHelp: 'AI가 이 자리에 글을 씁니다',
        text: '고정 문구',
        textHelp: '적은 그대로 글에 들어갑니다',
        photo: '사진',
        photoHelp: '첨부한 사진이 들어갑니다',
        repeat: '사진마다 반복',
        repeatHelp: '사진 개수만큼 안쪽을 되풀이합니다',
        note: 'AI에게만 하는 말',
        noteHelp: 'AI만 읽고 글에는 안 나옵니다',
      },
      // What an unlabelled legacy 자리 is called once it is read as 고정 문구 (TEMPLATE-37).
      legacy: {
        place: '지도',
        link: '링크',
      },
      block: {
        asksForData: '데이터 받기',
        asksForDataHelp:
          '켜면 글쓰기 화면에서 이 자리에 넣을 내용을 직접 입력받아요. 지어내지 않아요.',
        askTitle: '입력란 제목',
        askInRepeat:
          '사진마다 반복 안에서는 데이터를 받을 수 없어요. 칸 개수가 사진 수에 따라 달라지기 때문이에요.',
        instruction: '무엇을 쓸지',
        text: '들어갈 문구',
        label: '이 자리의 이름',
        note: 'AI에게 남길 말',
        count: '가로로 놓을 사진 수',
        fewer: '줄이기',
        more: '늘리기',
        drag: '끌어서 옮기기',
        moveUp: '위로',
        moveDown: '아래로',
        remove: '삭제',
      },
      reasons: {
        unknown_tag: '모르는 표기예요',
        unclosed_tag: '닫히지 않았어요',
        unexpected_close: '열리지 않은 것이 닫혔어요',
        malformed_tag: '표기 형식이 잘못됐어요',
        missing_attribute: '빠진 항목이 있어요',
        unknown_slot_kind: '모르는 자리 종류예요',
        unknown_repeat_each: '모르는 반복 기준이에요',
        nested_repeat: '반복 안에 반복은 넣을 수 없어요',
        empty_write: '무엇을 쓸지 비어 있어요',
        empty_note: '메모가 비어 있어요',
        invalid_count: '가로 사진 수가 1~{{max}} 사이가 아니에요',
        duplicate_ask_label: '같은 제목의 데이터 받기가 이미 있어요',
        ask_in_repeat: '사진마다 반복 안에서는 데이터를 받을 수 없어요',
        too_many_asks: '데이터 받기는 최대 {{askMax}}개까지예요',
        not_in_title: '사진·사진마다 반복·AI에게만 하는 말은 제목에 넣을 수 없어요',
      },
    },
    slot: {
      unfilled: '채워야 할 자리',
      pending: '채워야 할 자리 {{count}}곳',
      pending_one: '채워야 할 자리 {{count}}곳',
      pending_other: '채워야 할 자리 {{count}}곳',
      exportHint: '대괄호로 남은 자리는 붙여 넣은 뒤 플랫폼에서 직접 채워 주세요.',
    },
    postCount_one: '글 {{count}}개',
    postCount_other: '글 {{count}}개',
    detachWarning: {
      used: '{{count}}개의 글에서 템플릿이 해제됩니다. 글과 본문은 그대로 남아요.',
      used_one: '{{count}}개의 글에서 템플릿이 해제됩니다. 글과 본문은 그대로 남아요.',
      used_other: '{{count}}개의 글에서 템플릿이 해제됩니다. 글과 본문은 그대로 남아요.',
      unused: '이 템플릿을 쓰는 글이 없어요.',
    },
  },
  en: {
    title: 'Templates',
    loadFailed: 'Could not load the template list.',
    noTemplate: 'None',
    emptyDescription: 'No description',
    composition: {
      fixInSource: 'Fix in source',
      add: 'Add block',
      empty: 'Add blocks above to lay out the post.',
      insertHere: 'Adds here',
      repeatEmpty: 'Nothing inside this repeat yet.',
      repeatHelp:
        'The blocks inside repeat once per attached photo. Each repetition uses {{count}} photos.',
      unreadable:
        "This template's composition can't be read. Fix it in the source, or clear it and start over.",
      clearAndRestart: 'Clear and start over',
      titleArea: {
        add: 'Add to the title',
        empty: 'Add blocks above to lay out the title.',
        unreadable:
          "This title format can't be read. Fix it in the source, or clear it and start over.",
        clearAndRestart: 'Clear the title and start over',
      },
      summary: {
        // Only ever formatted with a count of one — photoSummaryKey sends anything above it to
        // photoRow — so no plural form is needed on either side.
        photo: '{{count}} photo',
        photoRow: '{{count}} photos side by side',
      },
      placeholder: {
        write: 'Say what to write here',
        text: 'Type the text that goes in',
        note: 'Say something to AI only',
        photo: 'An attached photo goes here',
        repeat: 'Once per photo',
      },
    },
    // The one surface where this app's grammar is visible (TEMPLATE-26).
    source: {
      label: 'Source',
      titleAreaLabel: 'Title source',
      copy: 'Copy source',
      copyGuide: 'Copy format guide',
      copied: 'Copied',
      manualCopy: 'Copying is blocked. Press and hold the selected text to copy it.',
      showGuide: 'Show format guide',
      error: 'Line {{line}}: {{reason}}',
      guide: `Write the body of one blog post template in the format below.

[What a template does]
A template is the skeleton of a post. It decides the order and what goes where; it never decides tone or word choice.

[The six things you can write]
- Plain text: appears in the post exactly as written.
- <write>what to write</write>: the AI writes here as instructed. The instruction itself never appears in the post.
- <slot kind="photo"/>: one attached photo goes here. With a count, as in <slot kind="photo" count="2"/>, that many stand side by side in one row.
- <repeat each="photo">…</repeat>: what is inside repeats once per group of photos.
- <note>note to the AI</note>: only the AI reads it; it never appears in the post.
- <ask label="field title"/>: what the author types on the write screen goes here exactly as typed.
- <ask label="field title">what to write</ask>: the AI writes here using only what the author typed as its facts.

[Rules that must hold]
- write, note and repeat must be closed.
- slot closes itself, as <slot …/>, and kind may only be photo.
- count is a whole number from 1 to {{photoRowMax}}. Without it, one photo.
- repeat takes only each="photo", and a repeat may not contain a repeat.
- write and note may never be empty.
- ask must carry a label, no two may share one in the same body, none may sit inside a repeat, and one body holds at most {{askMax}}.
- Use ask only for what the author has to supply each time — a rating, a visit date, a price: facts the AI cannot know.
- Any tag other than those six is refused.
- To write a literal < in a sentence, write &lt; instead.
- The whole body may not exceed {{bodyMax}} characters.

[Example]
{{example}}

[How to answer]
Send the body only — no explanation and no code fence. Write it in the language I am writing in.`,
    },
    builder: {
      palette: {
        write: 'AI writes here',
        writeHelp: 'AI writes prose here',
        text: 'Fixed text',
        textHelp: 'Appears in the post exactly as typed',
        photo: 'Photo',
        photoHelp: 'An attached photo goes here',
        repeat: 'Repeat per photos',
        repeatHelp: 'Repeats its contents once per photo',
        note: 'Note to AI',
        noteHelp: 'Only AI reads it; it never appears in the post',
      },
      // What an unlabelled legacy position is called once it is read as fixed text (TEMPLATE-37).
      legacy: {
        place: 'Map',
        link: 'Link',
      },
      block: {
        asksForData: 'Ask for data',
        asksForDataHelp:
          'On, the write screen asks you for what goes here instead of the AI inventing it.',
        askTitle: 'Field title',
        askInRepeat:
          'A row inside Repeat per photos cannot ask for data: how many fields there are must not depend on the photo count.',
        instruction: 'What to write',
        text: 'Text to include',
        label: 'Name for this position',
        note: 'Note for AI',
        count: 'Photos side by side',
        fewer: 'Fewer',
        more: 'More',
        drag: 'Drag to move',
        moveUp: 'Move up',
        moveDown: 'Move down',
        remove: 'Remove',
      },
      reasons: {
        unknown_tag: 'unknown notation',
        unclosed_tag: 'never closed',
        unexpected_close: 'closed without being opened',
        malformed_tag: 'malformed notation',
        missing_attribute: 'a required part is missing',
        unknown_slot_kind: 'unknown position kind',
        unknown_repeat_each: 'unknown repeat basis',
        nested_repeat: 'a repeat cannot contain a repeat',
        empty_write: 'nothing to write',
        empty_note: 'the note is empty',
        invalid_count: 'photos per row must be between 1 and {{max}}',
        duplicate_ask_label: 'another field already asks under that title',
        ask_in_repeat: 'a field inside 사진마다 반복 cannot ask for data',
        too_many_asks: 'at most {{askMax}} fields may ask for data',
        not_in_title: 'a photo, 사진마다 반복 or a note to the AI cannot go in the title',
      },
    },
    slot: {
      unfilled: 'Position to fill',
      pending: '{{count}} positions to fill',
      pending_one: '{{count}} position to fill',
      pending_other: '{{count}} positions to fill',
      exportHint: 'Fill the bracketed positions in the platform editor after pasting.',
    },
    postCount_one: '{{count}} post',
    postCount_other: '{{count}} posts',
    detachWarning: {
      used: '{{count}} posts will lose their template. The posts and their content stay.',
      used_one: '{{count}} post will lose its template. The post and its content stay.',
      used_other: '{{count}} posts will lose their template. The posts and their content stay.',
      unused: 'No post uses this template.',
    },
  },
} as const satisfies I18nFragment
