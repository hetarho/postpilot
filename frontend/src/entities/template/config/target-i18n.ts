import type { I18nFragment } from '@/shared/lib'

/** The shared target tree and heading-helper copy for post and clip forms. */
export const templateTargetsI18n = {
  namespace: 'templateTargets',
  ko: {
    questions: {
      root: '어떤 체험을 글로 남기나요?',
      restaurant: '어떤 식당인가요?',
      cafe: '어떤 카페인가요?',
      beauty: '어떤 뷰티 체험인가요?',
      stay: '어떤 숙소인가요?',
      activity: '어떤 체험이었나요?',
      product: '어떤 제품인가요?',
      appliance: '어떤 후기를 쓰나요?',
      cosmetics: '어떤 후기를 쓰나요?',
      food: '어떤 후기를 쓰나요?',
      living: '어떤 후기를 쓰나요?',
      babyPet: '어떤 후기를 쓰나요?',
    },
    targets: {
      restaurant: {
        name: '맛집',
        help: '식사, 코스, 술집, 포장·배달',
      },
      cafe: {
        name: '카페',
        help: '디저트, 브런치, 공간이 좋은 카페',
      },
      beauty: {
        name: '뷰티',
        help: '헤어, 네일·속눈썹, 피부 관리',
      },
      stay: {
        name: '숙소',
        help: '호텔, 펜션, 캠핑',
      },
      activity: {
        name: '체험·여가',
        help: '클래스, 액티비티, 전시·공연',
      },
      product: {
        name: '제품',
        help: '받아서 써 본 물건',
      },
      meal: {
        name: '식사',
        help: '한 끼를 먹고 온 식당',
      },
      course: {
        name: '코스·오마카세',
        help: '요리가 차례로 나오는 곳',
      },
      bar: {
        name: '술집·바',
        help: '술과 안주가 중심인 곳',
      },
      takeout: {
        name: '포장·배달',
        help: '집에서 받아 먹은 음식',
      },
      dessertBakery: {
        name: '디저트·베이커리',
        help: '빵과 디저트가 중심인 곳',
      },
      brunch: {
        name: '브런치',
        help: '식사 메뉴가 있는 카페',
      },
      spaceView: {
        name: '공간·뷰',
        help: '자리와 풍경이 좋은 곳',
      },
      hair: {
        name: '헤어',
        help: '커트, 펌, 염색',
      },
      nailLash: {
        name: '네일·속눈썹',
        help: '디자인을 고르는 시술',
      },
      skinEsthetic: {
        name: '피부·에스테틱',
        help: '피부 관리와 마사지',
      },
      hotelResort: {
        name: '호텔·리조트',
        help: '객실과 부대시설이 있는 곳',
      },
      pensionPool: {
        name: '펜션·풀빌라',
        help: '독채나 수영장이 있는 곳',
      },
      camping: {
        name: '캠핑·글램핑',
        help: '사이트나 텐트에서 묵는 곳',
      },
      oneDayClass: {
        name: '원데이 클래스',
        help: '직접 만들어 보는 수업',
      },
      leisure: {
        name: '놀거리·액티비티',
        help: '몸으로 즐기는 체험',
      },
      exhibition: {
        name: '전시·공연',
        help: '보고 듣는 체험',
      },
      appliance: {
        name: '가전·디지털',
        help: '설치하고 쓰는 기기',
      },
      cosmetics: {
        name: '화장품',
        help: '바르고 쓰는 제품',
      },
      food: {
        name: '식품·건강',
        help: '먹고 마시는 제품',
      },
      living: {
        name: '생활·주방',
        help: '집에서 쓰는 물건',
      },
      babyPet: {
        name: '육아·반려',
        help: '아이나 반려동물이 쓰는 제품',
      },
      unboxing: {
        name: '언박싱·첫인상',
        help: '받자마자 열어 본 이야기',
      },
      longTerm: {
        name: '실사용 후기',
        help: '한동안 써 본 이야기',
      },
      comparison: {
        name: '비교 후기',
        help: '다른 제품과 견줘 본 이야기',
      },
    },
    roles: {
      visitReason: {
        name: '방문 계기',
        help: '이곳을 찾게 된 이유와 기대를 씁니다',
      },
      locationParking: {
        name: '위치·주차',
        help: '찾아가는 길과 주차 정보를 씁니다',
      },
      exterior: {
        name: '외관·입구',
        help: '처음 마주한 가게 앞 모습을 씁니다',
      },
      interiorMood: {
        name: '내부 분위기',
        help: '자리와 인테리어, 분위기를 씁니다',
      },
      menuBoard: {
        name: '메뉴판',
        help: '어떤 메뉴가 있는지 메뉴판을 소개합니다',
      },
      orderedMenu: {
        name: '주문한 메뉴',
        help: '주문한 메뉴의 모습과 맛을 씁니다',
      },
      service: {
        name: '서비스·응대',
        help: '직원 응대와 서비스를 씁니다',
      },
      priceValue: {
        name: '가격·가성비',
        help: '가격과 양, 가성비를 씁니다',
      },
      drawbacks: {
        name: '아쉬운 점',
        help: '아쉬웠던 점을 읽는 사람에게 도움이 되게 씁니다',
      },
      verdict: {
        name: '총평',
        help: '전체적인 느낌과 누구에게 맞을지 정리합니다',
      },
      storeInfo: {
        name: '매장 정보',
        help: '주소, 영업시간, 연락처',
      },
      courseFlow: {
        name: '코스 흐름',
        help: '나온 요리를 순서대로 하나씩 씁니다',
      },
      reservation: {
        name: '예약 방법',
        help: '예약하는 방법과 팁을 씁니다',
      },
      drinksSnacks: {
        name: '주류·안주 구성',
        help: '어떤 술과 안주가 있는지 씁니다',
      },
      orderMethod: {
        name: '주문 방법',
        help: '어디서 어떻게 주문했는지 씁니다',
      },
      packaging: {
        name: '포장 상태',
        help: '받았을 때의 포장 상태를 씁니다',
      },
      deliveryTime: {
        name: '배달 시간',
        help: '주문부터 받기까지 걸린 시간을 씁니다',
      },
      spaceSeats: {
        name: '공간·좌석',
        help: '좌석 종류와 공간 구성을 씁니다',
      },
      orderedDrinks: {
        name: '주문한 음료·디저트',
        help: '주문한 음료와 디저트를 씁니다',
      },
      goodToStay: {
        name: '머물기 좋은 점',
        help: '오래 머물기 좋은 이유를 씁니다',
      },
      price: {
        name: '가격',
        help: '가격과 양을 씁니다',
      },
      takeoutGift: {
        name: '포장·선물',
        help: '포장이나 선물용으로 어떤지 씁니다',
      },
      waitingReservation: {
        name: '웨이팅·예약',
        help: '대기 시간과 예약 방법을 씁니다',
      },
      photoSpot: {
        name: '포토 스폿',
        help: '사진 찍기 좋은 자리를 소개합니다',
      },
      locationBooking: {
        name: '위치·예약 방법',
        help: '찾아가는 길과 예약 방법을 씁니다',
      },
      ambienceHygiene: {
        name: '매장 분위기·위생',
        help: '매장 분위기와 청결함을 씁니다',
      },
      consultation: {
        name: '상담',
        help: '받은 상담 내용을 씁니다',
      },
      procedure: {
        name: '시술·관리 과정',
        help: '진행된 과정을 차례로 씁니다',
      },
      productsUsed: {
        name: '사용 제품',
        help: '쓰인 제품을 소개합니다',
      },
      result: {
        name: '결과',
        help: '받고 난 뒤의 모습을 씁니다',
      },
      careTips: {
        name: '관리 팁',
        help: '들은 관리 방법과 팁을 씁니다',
      },
      designChoice: {
        name: '디자인 고르기',
        help: '디자인을 어떻게 골랐는지 씁니다',
      },
      skinDiagnosis: {
        name: '피부 상태 진단',
        help: '진단받은 피부 상태를 씁니다',
      },
      bookingReason: {
        name: '예약 계기',
        help: '이 숙소를 고른 이유를 씁니다',
      },
      locationTransport: {
        name: '위치·교통·주차',
        help: '찾아가는 길과 교통, 주차를 씁니다',
      },
      checkIn: {
        name: '체크인',
        help: '체크인 과정과 첫인상을 씁니다',
      },
      room: {
        name: '객실',
        help: '객실의 구성과 상태를 씁니다',
      },
      bathAmenity: {
        name: '욕실·어메니티',
        help: '욕실과 비치된 물품을 씁니다',
      },
      facilities: {
        name: '부대시설',
        help: '수영장, 라운지 같은 시설을 씁니다',
      },
      meals: {
        name: '식사·조식',
        help: '조식이나 식사를 씁니다',
      },
      nearby: {
        name: '주변 즐길 거리',
        help: '근처에서 즐길 만한 곳을 씁니다',
      },
      priceTips: {
        name: '가격·예약 팁',
        help: '가격과 예약할 때의 팁을 씁니다',
      },
      stayInfo: {
        name: '숙소 정보',
        help: '주소, 체크인·체크아웃 시간, 연락처',
      },
      siteTent: {
        name: '사이트·텐트',
        help: '사이트와 텐트 구성을 씁니다',
      },
      toiletSink: {
        name: '화장실·개수대',
        help: '화장실과 개수대 상태를 씁니다',
      },
      participationReason: {
        name: '참여 계기',
        help: '참여하게 된 이유를 씁니다',
      },
      space: {
        name: '공간',
        help: '진행된 공간을 소개합니다',
      },
      progress: {
        name: '진행 과정',
        help: '어떻게 진행됐는지 차례로 씁니다',
      },
      highlight: {
        name: '결과물·하이라이트',
        help: '가장 좋았던 순간이나 결과물을 씁니다',
      },
      goodWith: {
        name: '함께 가기 좋은 사람',
        help: '누구와 가면 좋을지 씁니다',
      },
      priceDuration: {
        name: '가격·소요 시간',
        help: '가격과 걸린 시간을 씁니다',
      },
      suppliesTips: {
        name: '준비물·팁',
        help: '챙기면 좋을 것과 팁을 씁니다',
      },
      activityInfo: {
        name: '정보',
        help: '장소, 운영 시간, 예약처',
      },
      receivedReason: {
        name: '받게 된 계기',
        help: '이 제품을 쓰게 된 계기를 씁니다',
      },
      components: {
        name: '구성품',
        help: '상자에 든 구성품을 소개합니다',
      },
      designFirst: {
        name: '디자인·첫인상',
        help: '처음 본 모습과 디자인을 씁니다',
      },
      mainFeatures: {
        name: '주요 기능',
        help: '제품의 주요 기능을 씁니다',
      },
      howToUse: {
        name: '사용 방법',
        help: '어떻게 쓰는지 씁니다',
      },
      usageReview: {
        name: '사용 후기',
        help: '직접 써 본 느낌을 씁니다',
      },
      pros: {
        name: '장점',
        help: '좋았던 점을 씁니다',
      },
      recommendedFor: {
        name: '추천 대상',
        help: '누구에게 맞을지 씁니다',
      },
      priceWhere: {
        name: '가격·구매처',
        help: '가격과 살 수 있는 곳',
      },
      usagePeriodEnv: {
        name: '사용 기간·환경',
        help: '얼마나, 어떤 환경에서 썼는지 씁니다',
      },
      comparedWith: {
        name: '비교 대상',
        help: '견줘 본 제품을 소개합니다',
      },
      comparisonCriteria: {
        name: '비교 기준',
        help: '무엇을 기준으로 비교했는지 씁니다',
      },
      comparisonResult: {
        name: '비교 결과',
        help: '비교해 본 결과를 씁니다',
      },
      installNoise: {
        name: '설치·소음',
        help: '설치 과정과 소음을 씁니다',
      },
      textureSpread: {
        name: '제형·발림성',
        help: '제형과 바를 때의 느낌을 씁니다',
      },
      tasteRecipe: {
        name: '맛·조리법',
        help: '맛과 조리 방법을 씁니다',
      },
      safetyReaction: {
        name: '안전·반응',
        help: '안전한지, 아이나 반려동물의 반응을 씁니다',
      },
    },
    headings: {
      title: '부분마다 소제목을 정해요',
      help: '내 블로그에서 쓰는 말로 바꿔 보세요. 같은 구성이어도 소제목이 다르면 다른 글이 돼요.',
      field: '{{name}} 소제목',
      suggest: '제목 제안 받기',
      free: '무료',
      credits: '약 {{count}} 크레딧',
      noModel: '제목을 제안받으려면 글 작성 모델을 골라 주세요.',
      chooseModel: '모델 고르기',
      tooMany: '제목 제안은 한 번에 {{max}}개 부분까지 받을 수 있어요.',
      running: '소제목을 제안하는 중이에요. 다른 단계로 이동해도 괜찮아요.',
      choices: '{{name}} 추천 소제목',
      retryRead: '결과 다시 불러오기',
      sheet: {
        label: '원하는 스타일을 말씀해주세요',
        placeholder: '자유롭게 적거나 아래 예시를 눌러 보세요',
        examplesLabel: '이렇게 말해 보세요',
        examples: [
          '이모지를 포함해서 20대 여성을 타겟팅할 수 있는 소제목으로 만들어줘',
          '짧고 담백하게, 명사로 끝나는 소제목으로 만들어줘',
          '읽고 싶어지게 궁금증을 남기는 질문형으로 만들어줘',
          '친구에게 말하듯 친근한 반말로 만들어줘',
        ],
        send: '제안 받기',
      },
    },
  },
  en: {
    questions: {
      root: 'What experience are you writing about?',
      restaurant: 'What kind of restaurant?',
      cafe: 'What kind of cafe?',
      beauty: 'What kind of beauty visit?',
      stay: 'What kind of stay?',
      activity: 'What kind of experience?',
      product: 'What kind of product?',
      appliance: 'What kind of review?',
      cosmetics: 'What kind of review?',
      food: 'What kind of review?',
      living: 'What kind of review?',
      babyPet: 'What kind of review?',
    },
    targets: {
      restaurant: {
        name: 'Restaurant',
        help: 'Meals, courses, bars, takeout',
      },
      cafe: {
        name: 'Cafe',
        help: 'Desserts, brunch, a great space',
      },
      beauty: {
        name: 'Beauty',
        help: 'Hair, nails and lashes, skin care',
      },
      stay: {
        name: 'Stay',
        help: 'Hotels, pensions, camping',
      },
      activity: {
        name: 'Experience',
        help: 'Classes, activities, shows',
      },
      product: {
        name: 'Product',
        help: 'Something you received and used',
      },
      meal: {
        name: 'Meal',
        help: 'A restaurant you ate at',
      },
      course: {
        name: 'Course · omakase',
        help: 'Dishes served one after another',
      },
      bar: {
        name: 'Bar',
        help: 'Drinks and bar food first',
      },
      takeout: {
        name: 'Takeout · delivery',
        help: 'Food you got at home',
      },
      dessertBakery: {
        name: 'Desserts · bakery',
        help: 'Bread and desserts first',
      },
      brunch: {
        name: 'Brunch',
        help: 'A cafe with meals',
      },
      spaceView: {
        name: 'Space · view',
        help: 'Seats and views first',
      },
      hair: {
        name: 'Hair',
        help: 'Cut, perm, color',
      },
      nailLash: {
        name: 'Nails · lashes',
        help: 'A design you choose',
      },
      skinEsthetic: {
        name: 'Skin care',
        help: 'Facials and massage',
      },
      hotelResort: {
        name: 'Hotel · resort',
        help: 'Rooms and facilities',
      },
      pensionPool: {
        name: 'Pension · pool villa',
        help: 'A private house or pool',
      },
      camping: {
        name: 'Camping · glamping',
        help: 'A site or a tent',
      },
      oneDayClass: {
        name: 'One-day class',
        help: 'Making something yourself',
      },
      leisure: {
        name: 'Activity',
        help: 'Something you do',
      },
      exhibition: {
        name: 'Exhibition · show',
        help: 'Something you watch',
      },
      appliance: {
        name: 'Appliances · digital',
        help: 'Devices you set up and use',
      },
      cosmetics: {
        name: 'Cosmetics',
        help: 'Things you apply',
      },
      food: {
        name: 'Food · health',
        help: 'Things you eat or drink',
      },
      living: {
        name: 'Home · kitchen',
        help: 'Things you use at home',
      },
      babyPet: {
        name: 'Baby · pet',
        help: 'For children or pets',
      },
      unboxing: {
        name: 'Unboxing · first look',
        help: 'Opening it the day it came',
      },
      longTerm: {
        name: 'After using it',
        help: 'Using it for a while',
      },
      comparison: {
        name: 'Comparison',
        help: 'Set against another product',
      },
    },
    roles: {
      visitReason: {
        name: 'Why I went',
        help: 'Why you chose this place and what you expected',
      },
      locationParking: {
        name: 'Location · parking',
        help: 'How to get there and where to park',
      },
      exterior: {
        name: 'Outside · entrance',
        help: 'The place as you first saw it',
      },
      interiorMood: {
        name: 'Inside',
        help: 'Seats, interior and mood',
      },
      menuBoard: {
        name: 'Menu',
        help: 'What the menu offers',
      },
      orderedMenu: {
        name: 'What I ordered',
        help: 'How each dish looked and tasted',
      },
      service: {
        name: 'Service',
        help: 'How the staff served you',
      },
      priceValue: {
        name: 'Price · value',
        help: 'Price, portions and value',
      },
      drawbacks: {
        name: 'Drawbacks',
        help: 'What fell short, written to help readers decide',
      },
      verdict: {
        name: 'Overall',
        help: 'Your overall impression and who it suits',
      },
      storeInfo: {
        name: 'Store details',
        help: 'Address, hours, contact',
      },
      courseFlow: {
        name: 'The course',
        help: 'Each dish in the order it came',
      },
      reservation: {
        name: 'Booking',
        help: 'How to book and tips',
      },
      drinksSnacks: {
        name: 'Drinks · bar food',
        help: 'What drinks and food they have',
      },
      orderMethod: {
        name: 'How I ordered',
        help: 'Where and how you ordered',
      },
      packaging: {
        name: 'Packaging',
        help: 'How it was packed when it came',
      },
      deliveryTime: {
        name: 'Delivery time',
        help: 'How long it took to arrive',
      },
      spaceSeats: {
        name: 'Space · seats',
        help: 'Seating and layout',
      },
      orderedDrinks: {
        name: 'Drinks · desserts',
        help: 'What you ordered',
      },
      goodToStay: {
        name: 'Good for staying',
        help: 'Why it is good to stay a while',
      },
      price: {
        name: 'Price',
        help: 'Price and portions',
      },
      takeoutGift: {
        name: 'Takeout · gifts',
        help: 'How it works to go or as a gift',
      },
      waitingReservation: {
        name: 'Waiting · booking',
        help: 'Waiting time and how to book',
      },
      photoSpot: {
        name: 'Photo spots',
        help: 'The best places for photos',
      },
      locationBooking: {
        name: 'Location · booking',
        help: 'How to get there and book',
      },
      ambienceHygiene: {
        name: 'Mood · hygiene',
        help: 'The atmosphere and cleanliness',
      },
      consultation: {
        name: 'Consultation',
        help: 'What you were told',
      },
      procedure: {
        name: 'The process',
        help: 'What was done, step by step',
      },
      productsUsed: {
        name: 'Products used',
        help: 'The products they used',
      },
      result: {
        name: 'Result',
        help: 'How it looked afterwards',
      },
      careTips: {
        name: 'Care tips',
        help: 'Aftercare and tips you heard',
      },
      designChoice: {
        name: 'Choosing a design',
        help: 'How you picked the design',
      },
      skinDiagnosis: {
        name: 'Skin check',
        help: 'What the skin check found',
      },
      bookingReason: {
        name: 'Why I booked',
        help: 'Why you chose this stay',
      },
      locationTransport: {
        name: 'Location · transport',
        help: 'How to get there and park',
      },
      checkIn: {
        name: 'Check-in',
        help: 'Check-in and first impressions',
      },
      room: {
        name: 'Room',
        help: 'The room and its condition',
      },
      bathAmenity: {
        name: 'Bathroom · amenities',
        help: 'The bathroom and what it provides',
      },
      facilities: {
        name: 'Facilities',
        help: 'Pool, lounge and other facilities',
      },
      meals: {
        name: 'Meals · breakfast',
        help: 'Breakfast or meals',
      },
      nearby: {
        name: 'Nearby',
        help: 'Things to enjoy around it',
      },
      priceTips: {
        name: 'Price · booking tips',
        help: 'Price and booking tips',
      },
      stayInfo: {
        name: 'Stay details',
        help: 'Address, check-in and check-out, contact',
      },
      siteTent: {
        name: 'Site · tent',
        help: 'The site and tent setup',
      },
      toiletSink: {
        name: 'Toilets · sinks',
        help: 'Toilets and washing areas',
      },
      participationReason: {
        name: 'Why I joined',
        help: 'Why you took part',
      },
      space: {
        name: 'The place',
        help: 'Where it took place',
      },
      progress: {
        name: 'How it went',
        help: 'What happened, step by step',
      },
      highlight: {
        name: 'Highlight',
        help: 'The best moment or what you made',
      },
      goodWith: {
        name: 'Good with',
        help: 'Who to go with',
      },
      priceDuration: {
        name: 'Price · time',
        help: 'Price and how long it took',
      },
      suppliesTips: {
        name: 'What to bring',
        help: 'What to bring and tips',
      },
      activityInfo: {
        name: 'Details',
        help: 'Place, hours, where to book',
      },
      receivedReason: {
        name: 'How I got it',
        help: 'How you came to use it',
      },
      components: {
        name: "What's in the box",
        help: 'What came in the box',
      },
      designFirst: {
        name: 'Design · first look',
        help: 'How it looked at first',
      },
      mainFeatures: {
        name: 'Main features',
        help: 'What it does',
      },
      howToUse: {
        name: 'How to use',
        help: 'How you use it',
      },
      usageReview: {
        name: 'Using it',
        help: 'How it felt to use',
      },
      pros: {
        name: 'Pros',
        help: 'What you liked',
      },
      recommendedFor: {
        name: 'Who it suits',
        help: 'Who it is right for',
      },
      priceWhere: {
        name: 'Price · where to buy',
        help: 'Price and where to buy',
      },
      usagePeriodEnv: {
        name: 'How long · where',
        help: 'How long and where you used it',
      },
      comparedWith: {
        name: 'Compared with',
        help: 'The product you compared it with',
      },
      comparisonCriteria: {
        name: 'What I compared',
        help: 'What you compared them on',
      },
      comparisonResult: {
        name: 'Comparison result',
        help: 'How they came out',
      },
      installNoise: {
        name: 'Setup · noise',
        help: 'Installation and noise',
      },
      textureSpread: {
        name: 'Texture · feel',
        help: 'Its texture and how it spreads',
      },
      tasteRecipe: {
        name: 'Taste · cooking',
        help: 'How it tastes and how to cook it',
      },
      safetyReaction: {
        name: 'Safety · reaction',
        help: 'How safe it is and how they reacted',
      },
    },
    headings: {
      title: 'A heading for each part',
      help: 'Use the words your blog uses. The same parts with your own headings make a post of your own.',
      field: 'Heading for {{name}}',
      suggest: 'Suggest headings',
      free: 'Free',
      credits: 'About {{count}} credits',
      noModel: 'Choose a writing model to get heading suggestions.',
      chooseModel: 'Choose a model',
      tooMany: 'Suggest headings for up to {{max}} parts at a time.',
      running: 'Suggesting headings. You can move to another step.',
      choices: 'Heading suggestions for {{name}}',
      retryRead: 'Load the result again',
      sheet: {
        label: 'Tell us the style you want',
        placeholder: 'Write freely or tap an example below',
        examplesLabel: 'Try saying',
        examples: [
          'Headings with emoji that speak to women in their twenties',
          'Short and plain, ending in a noun',
          'Questions that make readers curious',
          'Casual, like talking to a friend',
        ],
        send: 'Suggest',
      },
    },
  },
} as const satisfies I18nFragment
