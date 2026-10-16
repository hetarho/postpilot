/** The public `/about` surface (MKT-7). Every visible label, accessible name and metadata
 *  string lives here so the page composes keys rather than one language.
 *
 *  Claim discipline (MKT-4): every sentence here must be true of SHIPPED behavior. The plan
 *  numbers below mirror the code-owned limits table in `backend/internal/plan` (MKT-5, QUOTA-7).
 *  Changing the ladder means changing this copy in the same change. */
export const marketing = {
  metadata: {
    title: 'Postpilot이란? | 내가 겪은 그대로 쓰는 체험단 글',
    description:
      '체험단 방문의 사진·영상·메모를 내 말투의 네이버 블로그 글과 클립으로 옮깁니다. 내가 겪은 재료로 쓰도록 원칙을 적용하고, 문장 출처와 광고주 지침을 검토한 뒤 직접 게시합니다.',
  },
  about: {
    link: 'Postpilot이란?',
  },
  header: {
    nav: '소개',
    getStarted: '시작하기',
    login: '로그인',
  },
  hero: {
    positioning: '내가 겪은 그대로, 체험단 글을 빠르게',
    title: '체험단 사진과 메모를 내 말투의 네이버 블로그 글이나 클립으로',
    body: '체험단 방문에서 남긴 사진·영상·메모로 게시할 글과 클립을 만듭니다. AI는 내가 겪은 재료로만 쓰도록 항상 적용되는 원칙 아래 표현과 구성을 돕습니다. 문장의 출처와 광고주 지침을 살펴보고 내가 고친 뒤 직접 게시합니다.',
    access:
      '이메일 주소와 비밀번호로 계정을 만들고, 첫 로그인 전에 메일로 주소를 인증합니다. Google 로그인도 사용할 수 있습니다.',
    haveAccount: '이미 계정이 있나요?',
  },
  flow: {
    title: '어떻게 쓰나요',
    step1: {
      title: '채널·템플릿·말투를 고릅니다',
      body: '네이버 블로그, 네이버 클립, 인스타 클립 중 어디에 올릴지 고르고, 템플릿과 말투로 내 경험의 모양을 잡습니다.',
    },
    step2: {
      title: '사진·영상·메모와 광고주 지침을 더합니다',
      body: '내가 찍은 사진이나 영상, 겪은 일과 느낌을 적은 메모를 넣습니다. 체험단 글에는 광고주 지침도 함께 넣습니다.',
    },
    step3: {
      title: '재료를 먼저 관찰하고, 원칙 아래 씁니다',
      body: '사진과 영상을 먼저 관찰한 뒤 글을 씁니다. 항상 적용되는 원칙이 재료에 없는 경험을 지어내지 않도록 안내합니다.',
    },
    step4: {
      title: '출처를 살펴보고, 고치고, 지침을 확인합니다',
      body: '문구를 직접 입력 기반·사진에서 추론·광고주 요청·AI가 보탠 내용으로 나눠 살펴봅니다. 필요한 문장을 직접 고치거나 AI 수정을 요청하고, 광고주 지침 체크리스트를 확인합니다.',
    },
    step5: {
      title: '복사하거나 내보내 직접 게시합니다',
      body: '확정한 글은 네이버 블로그 형식으로 복사합니다. 클립은 영상과 업로드 문구를 준비해 각 서비스에 직접 게시합니다.',
    },
  },
  different: {
    title: '다른 점',
    voices: {
      title: '내 말투와 소제목으로 씁니다',
      body: '내가 고른 말투와 소제목으로 내 경험의 모양을 잡습니다. 말투마다 모은 학습 자료도 따로 유지합니다.',
    },
    observation: {
      title: '원칙은 항상 적용됩니다',
      body: '내가 준 재료로 쓰고, 없는 경험을 지어내지 않도록 하는 원칙은 끌 수 없습니다. 관찰과 작성은 별도 단계입니다.',
    },
    blocks: {
      title: '독자의 판단이 가게에도 도움이 되도록',
      body: '내가 적은 아쉬운 점도 정중하게 남깁니다. 독자가 판단할 수 있는 솔직한 글로 가게와 함께하는 상생을 지향하며, 내가 하지 않은 칭찬을 보태지 않도록 합니다.',
    },
    control: {
      title: '문장마다 출처를 살펴봅니다',
      body: '직접 입력 기반, 사진에서 추론, 광고주 요청, AI가 보탠 내용을 구분해 검토합니다. 남길 표현과 고칠 문장은 내가 결정합니다.',
    },
  },
  outputs: {
    title: '결과물은 어디로 가나요',
    body: '네이버 블로그 글이나 네이버 클립·인스타 클립 영상과 업로드 문구를 준비합니다. 출처 표시와 기술 정보는 복사한 글에 섞이지 않습니다. 글을 복사하거나 영상을 내보낸 뒤 내가 각 서비스에 직접 게시합니다.',
    naver: '네이버 블로그 글',
    naverClip: '네이버 클립 영상·업로드 문구',
    instaClip: '인스타 클립 영상·업로드 문구',
  },
  plans: {
    title: '요금제',
    body: '요금제마다 매일 지급 크레딧, 월 보너스, 이용할 수 있는 모델 등급과 서버 내보내기 횟수가 다릅니다. 실제 요금 선택과 결제는 플랜 화면에서 진행합니다.',
    assignment: '구독자는 플랜 화면에서 원하는 등급을 고르고 결제해 구독을 시작할 수 있습니다.',
    master:
      'master는 운영자 등급입니다. 사용량 제한이 없고 계정 관리를 담당하며, 사용자가 받을 수 있는 등급이 아닙니다.',
  },
  facts: {
    title: '통제와 데이터',
    images: '사진 원본은 브라우저에서 변환한 뒤 올라갑니다.',
    isolation: '계정과 말투별로 학습 자료가 분리돼 있습니다.',
    noBackground: '화면을 여는 것만으로는 AI 작업이 시작되지 않습니다. 실행은 항상 직접 누릅니다.',
    credentials: '복사와 내보내기에는 목적지 서비스의 로그인 정보가 필요하지 않습니다.',
  },
  footer: {
    tagline: '내 경험에서 체험단 글과 클립까지',
  },
} as const
