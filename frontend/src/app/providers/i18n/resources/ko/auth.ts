export const auth = {
  login: {
    intro: '계속하려면 로그인하세요',
    id: '이메일 또는 아이디',
    password: '비밀번호',
    submit: '로그인',
    failed: '아이디 또는 비밀번호가 맞지 않아요',
  },
  field: {
    email: '이메일',
    password: '비밀번호',
  },
  links: {
    login: '로그인',
    signup: '회원가입',
    more: '계정 메뉴',
  },
  signup: {
    intro: '이메일로 Postpilot 계정을 만드세요',
    passwordHint: '8자 이상 입력해 주세요.',
    submit: '가입하기',
    mailedHeading: '메일을 확인해 주세요',
    mailedBody: '{{email}}로 인증 링크를 보냈어요. 이미 가입된 주소여도 안내 메일이 도착해요.',
  },
  verification: {
    checking: '이메일을 인증하고 있어요',
    checkingBody: '잠시만 기다려 주세요.',
    successHeading: '이메일 인증 완료',
    successBody: '이제 로그인할 수 있어요.',
    failedHeading: '이메일을 인증하지 못했어요',
    resend: '다시 보내기',
    resent: '{{email}}로 인증 메일을 다시 보냈어요.',
  },
  logout: {
    failed: '로그아웃하지 못했어요. 세션이 아직 살아 있으니 다시 시도해 주세요.',
  },
  account: {
    label: '내 계정',
    signedInAs: '로그인한 계정',
  },
  accountSettings: {
    heading: '계정 설정',
    identity: '계정 정보',
    id: '계정 아이디',
    noEmail: '등록된 이메일 없음',
    verified: '인증됨',
    unverified: '인증 대기',
  },
  registerEmail: {
    heading: '이메일 등록',
    intro: '인증 메일을 받을 주소를 등록해 주세요.',
    submit: '인증 메일 보내기',
    sent: '{{email}}로 인증 메일을 보냈어요.',
  },
} as const
