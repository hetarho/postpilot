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
      repeatEmpty: '이 사진마다 반복 안에 아직 아무것도 없어요.',
      repeatHelp:
        '스토리라인의 사진 묶음마다 안쪽 블록이 되풀이돼요. 한 번 되풀이할 때 사진 자리 {{count}}장이 있어요.',
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
        photo: '한 줄에 {{count}}장',
      },
      placeholder: {
        write: '이 자리에 무엇이 오는지 적어 주세요 (예: 메뉴 소개)',
        text: '들어갈 문구를 적어 주세요',
        photo: '스토리라인에 맞는 사진이 들어갑니다',
        repeat: '스토리라인의 사진 묶음마다 되풀이합니다',
      },
    },
    // The one surface where this app's grammar is visible (TMPL-26).
    source: {
      label: '원문',
      titleAreaLabel: '제목 원문',
      copy: '원문 복사',
      copyGuide: '형식 안내 복사',
      copied: '복사했어요',
      manualCopy: '복사가 막혀 있어요. 선택된 내용을 길게 눌러 복사해 주세요.',
      showGuide: '형식 안내 보기',
      error: '{{line}}번째 줄: {{reason}}',
      // The area a server refusal names (TMPL-20).
      area: { title_area: '제목', body: '본문' },
      guideFailed: '형식 안내를 불러오지 못했어요.',
    },
    builder: {
      palette: {
        write: 'AI가 쓰는 글',
        writeHelp: '적어 둔 주제로 AI가 글을 씁니다',
        text: '고정 문구',
        textHelp: '적은 그대로 글에 들어갑니다',
        photo: '사진',
        photoHelp: '스토리라인에 맞는 사진이 들어갑니다',
        repeat: '사진마다 반복',
        repeatHelp: '스토리라인의 사진 묶음마다 되풀이합니다',
      },
      // What an unlabelled legacy 자리 is called once it is read as 고정 문구 (TMPL-37).
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
          '사진마다 반복 안에서는 데이터를 받을 수 없어요. 되풀이 횟수가 글의 스토리라인에 따라 달라지기 때문이에요.',
        instruction: '이 자리에 오는 것',
        text: '들어갈 문구',
        count: '한 줄에 놓을 사진 수',
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
        unknown_repeat_each: '사진마다 반복의 기준은 사진만 쓸 수 있어요',
        nested_repeat: '사진마다 반복 안에 사진마다 반복은 넣을 수 없어요',
        empty_write: '이 자리에 오는 것이 비어 있어요',
        invalid_count: '가로 사진 수가 1~{{max}} 사이가 아니에요',
        duplicate_ask_label: '같은 제목의 데이터 받기가 이미 있어요',
        ask_in_repeat: '사진마다 반복 안에서는 데이터를 받을 수 없어요',
        too_many_asks: '데이터 받기는 최대 {{askMax}}개까지예요',
        not_in_title: '사진·사진마다 반복은 제목에 넣을 수 없어요',
      },
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
      repeatEmpty: 'Nothing inside this Repeat per photos yet.',
      repeatHelp:
        'The blocks inside repeat once per photo group of the storyline. Each repetition holds {{count}} photo places.',
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
        photo: '{{count}} per row',
      },
      placeholder: {
        write: 'What goes here (e.g. the menu)',
        text: 'Type the text that goes in',
        photo: 'Photos that fit the storyline go here',
        repeat: 'Repeats once per photo group of the storyline',
      },
    },
    // The one surface where this app's grammar is visible (TMPL-26).
    source: {
      label: 'Source',
      titleAreaLabel: 'Title source',
      copy: 'Copy source',
      copyGuide: 'Copy format guide',
      copied: 'Copied',
      manualCopy: 'Copying is blocked. Press and hold the selected text to copy it.',
      showGuide: 'Show format guide',
      error: 'Line {{line}}: {{reason}}',
      area: { title_area: 'Title', body: 'Body' },
      guideFailed: "Couldn't load the format guide.",
    },
    builder: {
      palette: {
        write: 'AI writes here',
        writeHelp: 'AI writes about the topic you name',
        text: 'Fixed text',
        textHelp: 'Appears in the post exactly as typed',
        photo: 'Photo',
        photoHelp: 'Photos that fit the storyline go here',
        repeat: 'Repeat per photos',
        repeatHelp: 'Repeats once per photo group of the storyline',
      },
      // What an unlabelled legacy position is called once it is read as fixed text (TMPL-37).
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
          'A row inside Repeat per photos cannot ask for data: how many times it repeats follows the post’s storyline.',
        instruction: 'What goes here',
        text: 'Text to include',
        count: 'Photos per row',
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
        unknown_repeat_each: 'Repeat per photos can only go by photo',
        nested_repeat: 'a Repeat per photos cannot hold another one',
        empty_write: 'nothing says what goes here',
        invalid_count: 'photos per row must be between 1 and {{max}}',
        duplicate_ask_label: 'another field already asks under that title',
        ask_in_repeat: 'a field inside Repeat per photos cannot ask for data',
        too_many_asks: 'at most {{askMax}} fields may ask for data',
        not_in_title: 'a photo or Repeat per photos cannot go in the title',
      },
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
