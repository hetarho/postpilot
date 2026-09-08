export const templates = {
  title: '템플릿',
  page: {
    description:
      '템플릿은 글의 구성과 순서를 정해요. 글마다 하나를 고르면 AI가 그 구성대로 씁니다. 문체와 종결어미는 그대로 말투 프로필을 따라요.',
    saved: '저장된 템플릿',
    new: '새 템플릿',
    empty: '아직 저장된 템플릿이 없어요',
    emptyHelp:
      '매번 같은 틀로 쓰는 글이 있다면, 그 틀을 한 번만 만들어 두고 글마다 골라 쓰세요. 인트로·사진 설명·총평처럼 글의 순서를 정해 두면 AI가 매번 그 순서대로 씁니다.',
    name: '이름',
    newDockAria: '새 템플릿 만들기',
  },
  loadFailed: '템플릿 목록을 불러오지 못했어요.',
  noTemplate: '없음',
  create: {
    name: '템플릿 이름',
    namePlaceholder: '예: 정보성 식당 리뷰',
    description: '어떤 글인가요',
    descriptionPlaceholder: '예: 식사를 제공받고 쓰는 방문 리뷰',
    body: '템플릿 구성',
    help: '템플릿은 글의 구성과 순서를 정해요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
    submit: '템플릿 만들기',
  },
  emptyDescription: '설명 없음',
  screen: {
    mode: {
      aria: '구성 편집 방식',
      builder: '블록',
      source: '원문',
    },
    sourceHelp:
      '템플릿을 저장 형식 그대로 보고 고쳐요. 형식 안내를 AI에게 건네 만든 템플릿을 여기에 붙여 넣을 수 있어요.',
    backToList: '← 템플릿 목록',
    newTitle: '새 템플릿',
    notFound: '이 템플릿을 찾을 수 없어요. 목록에서 다시 골라 주세요.',
    compositionHelp:
      '위에서 블록을 더해 글의 순서를 짜세요. 줄을 누르면 내용을 고칠 수 있고, 끌어서 순서를 바꿀 수 있어요.',
    saved: '저장했어요.',
    saveDockAria: '템플릿 저장',
    leaveTitle: '저장하지 않고 나갈까요?',
    leaveDescription: '고친 내용이 아직 저장되지 않았어요. 지금 나가면 사라집니다.',
    leaveConfirm: '저장하지 않고 나가기',
  },
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
    copy: '원문 복사',
    copyGuide: '형식 안내 복사',
    copied: '복사했어요',
    manualCopy: '복사가 막혀 있어요. 선택된 내용을 길게 눌러 복사해 주세요.',
    showGuide: '형식 안내 보기',
    error: '{{line}}번째 줄: {{reason}}',
    guide: `아래 형식으로 블로그 글 템플릿의 본문을 하나 작성해 주세요.

[템플릿이 하는 일]
템플릿은 글의 뼈대입니다. 글의 순서와 어디에 무엇이 들어갈지를 정하고, 문체나 어휘는 정하지 않습니다.

[쓸 수 있는 표기 다섯 가지]
- 그냥 쓴 문장: 글에 그대로 나옵니다.
- <write>무엇을 쓸지</write>: AI가 그 자리에 지시대로 글을 씁니다. 지시문 자체는 글에 나오지 않습니다.
- <slot kind="photo"/>: 첨부한 사진이 한 장 들어갑니다. <slot kind="photo" count="2"/>처럼 count를 주면 그만큼 한 줄에 나란히 놓입니다.
- <repeat each="photo">…</repeat>: 안쪽 내용이 사진 수만큼 되풀이됩니다.
- <note>AI에게만 하는 말</note>: AI만 읽고 글에는 나오지 않습니다.

[지켜야 할 규칙]
- write, note, repeat는 반드시 닫아야 합니다.
- slot은 <slot …/>처럼 스스로 닫고, kind는 photo만 쓸 수 있습니다.
- count는 1에서 {{photoRowMax}} 사이의 정수입니다. 없으면 1장입니다.
- repeat는 each="photo"만 받고, repeat 안에 repeat를 넣을 수 없습니다.
- write와 note는 비워 둘 수 없습니다.
- 위 다섯 가지 말고 다른 태그를 쓰면 저장되지 않습니다.
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
    },
  },
  slot: {
    unfilled: '채워야 할 자리',
    pending: '채워야 할 자리 {{count}}곳',
    pending_one: '채워야 할 자리 {{count}}곳',
    pending_other: '채워야 할 자리 {{count}}곳',
    exportHint: '대괄호로 남은 자리는 붙여 넣은 뒤 플랫폼에서 직접 채워 주세요.',
  },
  delete: {
    aria: '{{name}} 삭제',
    title: '이 템플릿을 삭제할까요?',
    description:
      '‘{{name}}’을(를) 지웁니다. {{detach}} 이미 만들어진 글의 결과와 진행 중인 작업은 그대로예요.',
  },
  assignment: {
    runningJob:
      '진행 중인 AI 작업은 시작할 때의 템플릿으로 끝나요. 바꾼 템플릿은 다음 생성부터 적용됩니다.',
    notFound: '고른 템플릿을 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요.',
    notFoundDetail:
      '고른 템플릿을 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요. {{error}}',
    failed: '템플릿을 바꾸지 못했어요. 다시 시도해 주세요.',
  },
  postCount: '글 {{count}}개',
  postCount_one: '글 {{count}}개',
  postCount_other: '글 {{count}}개',
  detachWarning: {
    used: '{{count}}개의 글에서 템플릿이 해제됩니다. 글과 본문은 그대로 남아요.',
    used_one: '{{count}}개의 글에서 템플릿이 해제됩니다. 글과 본문은 그대로 남아요.',
    used_other: '{{count}}개의 글에서 템플릿이 해제됩니다. 글과 본문은 그대로 남아요.',
    unused: '이 템플릿을 쓰는 글이 없어요.',
  },
} as const
