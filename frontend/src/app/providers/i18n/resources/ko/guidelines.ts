export const guidelines = {
  title: '지침',
  page: {
    description:
      '지침은 글에서 피해야 할 내용과 주의할 점을 정해요. 저장하면 이 계정의 모든 글에 적용되고, 특정 용도에만 적용되게 좁힐 수도 있어요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
    saved: '저장된 지침',
    new: '새 지침',
    empty: '아직 저장된 지침이 없어요',
    emptyHelp:
      '글을 받아 보고 매번 지우던 문장을 여기에 한 번만 저장해 두세요. 예를 들어 이런 식이에요.',
    example: '무인 매장 글에서 직원·주인과의 상호작용이나 CCTV를 언급하지 않기',
    order: '지침은 전역 지침 먼저, 그다음 용도 지침 순서로 적용돼요.',
  },
  loadFailed: '지침 목록을 불러오지 못했어요.',
  scope: {
    label: '적용 범위',
    global: '전역',
    purposes: '특정 용도',
    globalHelp: '이 계정의 모든 글에 적용돼요.',
    purposesHelp: '고른 용도가 지정된 글에만 적용돼요.',
    pick: '적용할 용도',
    orphaned: '적용 대상 없음',
    orphanedHelp:
      '지정했던 용도가 삭제돼서 지금은 어떤 글에도 적용되지 않아요. 범위를 다시 고르거나 삭제해 주세요.',
    purposesEmpty: '먼저 용도를 하나 만들어 주세요.',
  },
  create: {
    text: '지침',
    textPlaceholder: '예: 무인 매장 글에서 CCTV를 언급하지 않기',
    help: '한 줄에 규칙 하나씩, 짧게 적어 주세요. 지침이 용도의 요구와 충돌하면 지침을 우선합니다.',
    submit: '지침 만들기',
  },
  edit: {
    text: '지침',
    scope: '적용 범위',
  },
  delete: {
    aria: '지침 삭제',
    title: '이 지침을 삭제할까요?',
    description:
      '이 지침을 지웁니다. 이미 시작된 AI 작업은 시작할 때의 지침으로 끝나고, 글과 본문·용도·말투는 그대로예요.',
  },
  capture: {
    action: '지침으로 저장',
    title: '지침으로 저장',
    description:
      '이 수정 요청을 다음 글에도 계속 적용할 규칙으로 저장해요. 저장 전에 고칠 수 있어요.',
    scopeGlobal: '전역',
    scopePurpose: '이 글의 용도 「{{name}}」에만',
    submit: '저장',
    saved: '지침으로 저장했어요.',
    duplicate: '이미 같은 지침이 있어요.',
  },
} as const
