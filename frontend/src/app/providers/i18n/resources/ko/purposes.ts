export const purposes = {
  title: '용도',
  page: {
    description:
      '용도는 글의 종류와 구성을 정해요. 글마다 하나를 고르면 AI가 그 지침대로 씁니다. 문체와 종결어미는 그대로 말투 프로필을 따라요.',
    saved: '저장된 용도',
    new: '새 용도',
    empty: '아직 저장된 용도가 없어요',
    emptyHelp:
      '메모에 매번 다시 쓰던 설명을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
    name: '이름',
    exampleName: '정보성 식당 리뷰',
    exampleDescription: '식사를 제공받고 쓰는 방문 리뷰',
    exampleInstructions:
      '사진마다 무엇인지 설명하세요.\n일기체로 쓰지 마세요.\n방문 정보를 마지막에 적으세요.',
  },
  loadFailed: '용도 목록을 불러오지 못했어요.',
  noPurpose: '없음',
  create: {
    name: '용도 이름',
    namePlaceholder: '예: 정보성 식당 리뷰',
    description: '어떤 글인가요',
    descriptionPlaceholder: '예: 식사를 제공받고 쓰는 방문 리뷰',
    instructions: '작성 지침',
    instructionsPlaceholder:
      '예: 사진마다 무엇인지 설명하세요.\n일기체로 쓰지 마세요.\n방문 정보를 마지막에 적으세요.',
    help: '용도는 글의 내용과 구성을 정해요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
    submit: '용도 만들기',
  },
  emptyDescription: '설명 없음',
  delete: {
    aria: '{{name}} 삭제',
    title: '이 용도를 삭제할까요?',
    description:
      '‘{{name}}’을(를) 지웁니다. {{detach}} 이미 만들어진 글의 결과와 진행 중인 작업은 그대로예요.',
  },
  assignment: {
    runningJob:
      '진행 중인 AI 작업은 시작할 때의 용도로 끝나요. 바꾼 용도는 다음 생성부터 적용됩니다.',
    notFound: '고른 용도를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요.',
    notFoundDetail: '고른 용도를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요. {{error}}',
    failed: '용도를 바꾸지 못했어요. 다시 시도해 주세요.',
    manage: '용도 관리',
  },
  postCount: '글 {{count}}개',
  postCount_one: '글 {{count}}개',
  postCount_other: '글 {{count}}개',
  detachWarning: {
    used: '{{count}}개의 글에서 용도가 해제됩니다. 글과 본문은 그대로 남아요.',
    used_one: '{{count}}개의 글에서 용도가 해제됩니다. 글과 본문은 그대로 남아요.',
    used_other: '{{count}}개의 글에서 용도가 해제됩니다. 글과 본문은 그대로 남아요.',
    unused: '이 용도를 쓰는 글이 없어요.',
  },
} as const
