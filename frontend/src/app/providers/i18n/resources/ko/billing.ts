export const billing = {
  title: '결제 관리',
  description: '구독, 결제 수단과 결제 기록을 한곳에서 확인합니다.',
  nav: '결제 관리',
  subscription: {
    heading: '구독',
    empty: '활성 구독이 없습니다.',
    choosePlans: '플랜 보기',
  },
  paymentMethod: {
    heading: '결제 수단',
    empty: '등록된 결제 수단이 없습니다.',
  },
  history: {
    heading: '결제 및 지급 기록',
    empty: '아직 결제 기록이 없습니다.',
  },
  purchases: {
    heading: '크레딧 구매',
    empty: '아직 구매한 크레딧이 없습니다.',
  },
  loadFailed: '결제 정보를 불러오지 못했습니다.',
} as const
