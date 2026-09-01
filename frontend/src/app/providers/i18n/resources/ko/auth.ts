export const auth = {
  login: {
    intro: '계속하려면 로그인하세요',
    id: '아이디',
    password: '비밀번호',
    submit: '로그인',
    failed: '아이디 또는 비밀번호가 맞지 않아요',
  },
  logout: {
    failed: '로그아웃하지 못했어요. 세션이 아직 살아 있으니 다시 시도해 주세요.',
  },
  account: {
    label: '내 계정',
    signedInAs: '로그인한 계정',
  },
} as const
