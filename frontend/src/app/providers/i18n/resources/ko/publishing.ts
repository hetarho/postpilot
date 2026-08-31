export const publishing = {
  title: '발행하기',
  stage: {
    queued: 'Mac 연결을 기다리는 중',
    claimed: 'Mac에서 작업을 받았어요',
    preparing: '발행 준비 중',
    openingEditor: '네이버 편집기 여는 중',
    fillingContent: '글 입력 중',
    uploadingPhotos: '사진 올리는 중',
    committing: '네이버에 최종 발행 중',
    verifying: '발행 결과 확인 중',
    published: '발행 완료',
    progress: '발행 진행 중',
  },
  visibility: { public: '전체 공개', private: '비공개' },
  lastSeen: '마지막 확인',
  configure: {
    label: '연결 이름',
    category: '기본 카테고리',
    visibility: '기본 공개 설정',
    save: '기본값 저장',
    failed: '기본값을 저장하지 못했어요.',
  },
  pair: {
    defaultLabel: '내 Mac',
    title: 'Mac 연결',
    description:
      '네이버 로그인은 Mac의 전용 브라우저에만 남습니다. 아래 코드를 Mac 설정 화면에 입력하세요.',
    label: '연결 이름',
    create: '연결 코드 만들기',
    code: '연결 코드',
    expires: '{{date}}까지 한 번만 사용할 수 있어요.',
    failed: '연결 코드를 만들지 못했어요.',
  },
  retry: {
    pending: '다시 요청하는 중…',
    action: '로그인 복구 후 다시 시도',
    failed: '같은 발행 작업을 다시 시작하지 못했어요. Mac 연결과 카테고리를 확인해 주세요.',
  },
  revoke: {
    action: '연결 해제',
    failed: '연결을 해제하지 못했어요.',
    title: 'Mac 연결을 해제할까요?',
    description:
      '{{label}}의 발행 토큰이 즉시 무효화됩니다. 네이버 로그인과 브라우저 프로필은 Mac에서 직접 지우기 전까지 남아 있습니다.',
  },
  cancelRetained: {
    action: '복구 작업 취소',
    failed: '복구 작업을 취소하지 못했어요.',
    title: '고정해 둔 발행 작업을 취소할까요?',
    confirm: '작업 취소',
    description:
      '{{postSlug}}의 고정된 글과 임시 사진을 삭제합니다. 이 작업은 다시 시도할 수 없습니다.',
  },
  startError: {
    aborted: '다른 화면에서 글이 바뀌었어요. 새로고침한 뒤 다시 확인해 주세요.',
    alreadyExists: '이미 진행 중이거나 발행을 마친 글이에요.',
    failedPrecondition: '글 확정, Mac 연결, 카테고리 상태를 확인해 주세요.',
    permissionDenied: '이 Mac 연결로는 발행할 수 없어요.',
    unknown: '발행 요청을 저장하지 못했어요. 다시 시도해 주세요.',
  },
  form: {
    unavailableAgent:
      '이 작업에 연결된 Mac을 현재 사용할 수 없어 다른 Mac으로 바꾸어 재시도하지 않았어요. Mac 연결을 다시 활성화하거나 이 작업을 취소해 주세요.',
    cancel: '발행 취소',
    cancelFailed: '발행을 취소하지 못했어요.',
    finalizeFirst: '현재 내용을 먼저 확정해야 정확히 이 버전을 발행할 수 있어요.',
    offline:
      'Mac이 지금 응답하지 않아도 요청은 서버에 보관되고, 에이전트가 켜지면 자동으로 시작됩니다.',
    agent: 'Mac 연결',
    category: '카테고리',
    visibility: '공개 설정',
    changedFinalize: '방금 수정한 내용을 다시 확정한 뒤 발행해 주세요.',
    saveFailed: '수정 내용을 저장하지 못해 발행을 시작하지 않았어요.',
    retry: '안전하게 다시 시도',
    publish: '네이버에 발행',
    confirmTitle: '네이버에 최종 발행할까요?',
    confirmDescription:
      '{{account}} 블로그의 {{category}} 카테고리에 올립니다. Mac 에이전트가 사진과 글을 입력한 뒤 네이버의 최종 발행 버튼까지 누르며, 그 순간에는 추가 확인을 요청하지 않습니다.',
  },
  agents: {
    title: '발행 Mac',
    description: '계정마다 별도의 Mac 토큰, Hermes 프로필과 전용 브라우저 프로필을 사용합니다.',
    list: '연결 목록',
    loadFailed: '연결 목록을 불러오지 못했어요.',
    empty: '아직 연결한 Mac이 없어요.',
    naverPending: '네이버 확인 대기',
    revoked: '해제됨',
    ready: '준비됨',
    setup: '설정 필요',
    retryTitle: '로그인 복구 후 다시 시도',
    retryDescription:
      'Mac의 같은 전용 브라우저에서 로그인·CAPTCHA·2단계 인증을 해결한 뒤, 고정해 둔 발행 작업을 그대로 다시 시작합니다. 원본 글을 삭제했어도 이 목록에서 재개할 수 있어요.',
    retryLoading: '복구 대기 작업을 불러오는 중…',
    retryLoadFailed: '복구 대기 작업을 불러오지 못했어요.',
    retryEmpty: '복구 후 다시 시도할 작업이 없어요.',
    loginCheck: 'Mac의 전용 네이버 로그인을 확인해 주세요.',
  },
  panel: {
    description:
      '연결한 Mac이 네이버 편집기를 열어 글과 JPEG 사진을 입력하고 최종 발행까지 마칩니다.',
    loading: '발행 상태를 불러오는 중…',
    loadFailed: '발행 상태를 불러오지 못했어요.',
    reload: '발행 상태 다시 불러오기',
    noAgent:
      '발행할 수 있는 Mac 연결이 아직 없어요. 네이버 로그인 정보는 Mac 밖으로 전송되지 않습니다.',
    connectAgent: 'Mac 연결하기',
    published: '발행을 마쳤어요.',
    viewPost: '네이버 글 보기',
    failed: '최종 발행 전에 안전하게 중단했어요.',
    needsAttention:
      'Mac의 전용 브라우저에서 네이버 로그인을 확인한 뒤 아래에서 같은 작업을 다시 시도하세요.',
    outcomeUnknown:
      '최종 발행 버튼이 눌렸을 수 있어요. 중복을 막기 위해 자동 재시도하지 않습니다. 네이버 블로그에서 직접 확인해 주세요.',
  },
} as const
