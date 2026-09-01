/** The public `/about` surface (plan 15). Every visible label, accessible name and metadata
 *  string lives here so the page composes keys rather than one language.
 *
 *  Claim discipline (spec/policy/public-marketing.md): every sentence here must be true of
 *  SHIPPED behavior. Two boundaries are load-bearing and must not be softened:
 *  - automated Naver publishing is an operator-tier surface AND still in live verification
 *    (plan 12 / job 25), so it is stated as such, never as a generally available feature;
 *  - the plan numbers below mirror the code-owned limits table in `backend/internal/plan`
 *    (spec/policy/plans.md). Changing the ladder means changing this copy in the same change. */
export const marketing = {
  metadata: {
    title: 'Postpilot이란? | 사진과 메모로 블로그 글 초안 만들기',
    description:
      '사진과 거친 메모를 내가 고른 말투로 블로그 초안까지 옮기는 도구입니다. 사진 관찰과 글쓰기를 나누고, 어떤 AI 모델을 쓸지 직접 고릅니다.',
  },
  about: {
    link: 'Postpilot이란?',
  },
  header: {
    nav: '소개',
    login: '로그인',
  },
  hero: {
    title: '사진과 메모를 내 말투의 블로그 초안으로',
    body: '찍어 둔 사진과 몇 줄 메모를 올리면, 내가 만들어 둔 말투 프로필로 블로그에 붙일 수 있는 초안까지 만듭니다. 읽어 보고 한 번 고치는 것까지가 한 흐름입니다.',
    access:
      '가입 절차는 없습니다. 계정은 운영자가 직접 만들어 드리며, 요금제도 운영자가 지정합니다.',
  },
  flow: {
    title: '어떻게 쓰나요',
    step1: {
      title: '말투·용도·출력 언어를 고릅니다',
      body: '어떤 말투로 쓸지, 어떤 종류의 글인지(용도), 한국어와 영어 중 어느 언어로 낼지 글마다 정합니다.',
    },
    step2: {
      title: '사진과 메모를 올립니다',
      body: '사진은 브라우저에서 변환한 뒤 올라갑니다. 메모는 문장이 아니어도 됩니다.',
    },
    step3: {
      title: '사진을 먼저 관찰하고, 그다음 씁니다',
      body: '사진에서 보이는 사실을 먼저 정리하고 그 결과로 본문을 씁니다. 관찰과 작성에 쓸 AI 모델은 각각 직접 고릅니다.',
    },
    step4: {
      title: '고치고 확정한 뒤 내보냅니다',
      body: '수정 요청으로 필요한 부분만 다시 쓰고, 확정한 글을 원하는 플랫폼 형식으로 복사합니다.',
    },
  },
  different: {
    title: '다른 점',
    voices: {
      title: '말투는 서로 섞이지 않습니다',
      body: '말투 프로필은 각각 따로 배웁니다. 한 말투에 모은 글이 다른 말투의 문장에 끼어들지 않습니다.',
    },
    observation: {
      title: '관찰과 작성이 분리돼 있습니다',
      body: '사진에서 무엇이 보이는지 정리하는 단계와 글을 쓰는 단계가 나뉘어 있어서, 사진에 없는 이야기를 덜 만듭니다.',
    },
    blocks: {
      title: '글은 구조화된 블록으로 남습니다',
      body: '본문·제목·이미지·인용·목록이 하나의 정식 구조로 저장되고, 플랫폼별 형식은 거기서 만들어집니다.',
    },
    control: {
      title: '모델과 실행을 직접 고릅니다',
      body: '단계마다 어떤 모델을 쓸지 고르고, 두 모델을 나란히 비교하거나 수정을 실행하는 것도 모두 명시적인 동작입니다.',
    },
  },
  outputs: {
    title: '결과물은 어디로 가나요',
    body: '확정한 글 하나에서 아래 형식들을 만듭니다. 형식마다 글을 다시 쓰지 않습니다.',
    naver: '네이버 블로그용',
    tistory: '티스토리용',
    html: '개인 사이트용 HTML',
    markdown: '마크다운',
    publishing:
      '네이버 자동 발행은 짝지은 Mac이 확정된 글에 대해 사용자가 직접 실행하는 별도 동작이며, 현재 운영자 등급에서만 쓰이고 실제 환경 검증이 진행 중입니다.',
  },
  plans: {
    title: '요금제',
    body: '요금제는 AI 작업을 하루에 몇 번 시작할 수 있는지, 하루와 한 달에 쓸 수 있는 AI 사용 금액, 고를 수 있는 모델 범위를 정합니다.',
    assignment:
      '요금제는 운영자가 계정에 지정합니다. 이 페이지에서 결제하거나 등급을 올릴 수는 없습니다.',
    columns: {
      plan: '요금제',
      dailyStarts: '하루 작업 시작',
      dailyBudget: '하루 AI 사용 금액',
      monthlyBudget: '한 달 AI 사용 금액',
      models: '모델',
    },
    free: {
      name: 'free',
      dailyStarts: '10회',
      dailyBudget: '$0.10',
      monthlyBudget: '$2.00',
      models: '기본 모델',
    },
    basic: {
      name: 'basic',
      dailyStarts: '30회',
      dailyBudget: '$0.50',
      monthlyBudget: '$12.00',
      models: '기본 + 중급 모델',
    },
    max: {
      name: 'max',
      dailyStarts: '100회',
      dailyBudget: '$1.00',
      monthlyBudget: '$25.00',
      models: '등록된 모든 모델',
    },
    master:
      'master는 운영자 등급입니다. 사용량 제한이 없고 네이버 자동 발행과 계정 관리를 담당하며, 사용자가 받을 수 있는 등급이 아닙니다.',
  },
  facts: {
    title: '통제와 데이터',
    images: '사진 원본은 브라우저에서 변환한 뒤 올라갑니다.',
    isolation: '계정과 말투별로 학습 자료가 분리돼 있습니다.',
    noBackground: '화면을 여는 것만으로는 AI 작업이 시작되지 않습니다. 실행은 항상 직접 누릅니다.',
    credentials: '네이버 로그인 정보와 브라우저 상태는 짝지은 Mac에 남습니다.',
  },
  footer: {
    tagline: '사진과 메모에서 블로그 초안까지',
  },
} as const
