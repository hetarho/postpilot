import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'creation',
  ko: {
    setup: {
      checking: '당신의 작업 공간을 준비하고 있어요',
      loadFailed: '설정을 확인하지 못했어요. 잠시 후 다시 시도해 주세요.',
      retry: '다시 시도',
      continue: '먼저 시작하기',
      later: '나중에 설정할게요',
      skip: '지금은 건너뛰기',
      back: '이전',
      next: '계속',
      finish: '시작하기',
      restart: '처음 설정 다시 살펴보기',
      progress: '나만의 작업 공간 준비',
      step: '{{current}} / {{total}}',
      optional: '언제든 설정에서 바꿀 수 있어요.',
      name: '이름',
      templateName: '템플릿 이름',
      save: '저장하고 계속',
      saved: '저장했어요',
      preview: '미리보기',
      modelFailed: 'AI 모델을 불러오지 못했어요.',
      overview: {
        label: '이번에 준비할 내용',
        voice: {
          title: '나에게 맞는 말투',
          description: '직접 쓴 글이나 답변으로 배우고, AI가 만든 말투를 고를 수도 있어요.',
        },
        'post-template': {
          title: '자주 쓰는 글 구성',
          description: '마음에 드는 시작, 사진 배치, 마무리를 골라요.',
        },
        'clip-template': {
          title: '영상이 이어지는 흐름',
          description: '첫 장면부터 마지막 자막까지 어울리는 구성을 골라요.',
        },
      },
      welcome: {
        title: '처음 한 번, 나답게 준비해요.',
        description:
          '말투와 자주 쓰는 구성을 맞춰 두면 다음부터는 바로 만들 수 있어요. 지금 필요한 것만 준비할게요.',
        action: '내 작업 공간 준비하기',
      },
      models: {
        title: '함께 만들 AI를 골라볼까요?',
        description:
          '사진을 읽고, 말투를 배우고, 글을 쓰는 모델이에요. 한 번 고르면 다음 작업에도 그대로 사용해요.',
        waiting: '필요한 모델을 골라 주세요. 나중에 설정해도 괜찮아요.',
      },
      voice: {
        autoName: '나의 말투',
        questionPath: '질문 10개로 나의 말투 찾기',
        questionPathHelp:
          '일상적인 상황에 짧게 답해 주세요. 이름이나 어려운 설정은 나중에 바꿔도 돼요.',
        aiPath: 'AI가 만든 8가지 스타일에서 고르기',
        aiPathHelp:
          '서로 다른 말투의 예문을 보고 마음에 드는 스타일을 골라요. 생성은 직접 누를 때만 시작돼요.',
        otherMethod: '다른 방법으로 준비하기',
        moreWays: '직접 쓴 글이 이미 있나요?',
        title: '글에 나의 말투를 담아볼까요?',
        description: '편한 방법으로 평소 쓰는 표현과 문장의 느낌을 알려 주세요.',
        name: '말투 이름',
        placeholder: '예: 평소의 나',
        create: '이 말투로 준비하기',
        material: '평소에 쓴 글을 알려 주세요.',
        materialHelp:
          '글을 붙여넣거나 짧은 질문에 답해 주세요. 필요한 정보가 모이면 직접 분석을 시작할 수 있어요.',
        confirmed: '말투가 준비됐어요.',
        use: '이 말투를 기본으로 사용',
        running: '당신의 말투를 배우고 있어요.',
        failed: '말투 분석을 마치지 못했어요. 입력한 글은 남아 있어요.',
        createFailed: '말투를 저장하지 못했어요. 다시 시도해 주세요.',
      },
      'post-template': {
        title: '자주 쓰는 글의 구성을 정해요.',
        description:
          '도입, 사진, 본문처럼 글에 반복해서 쓰는 순서를 만들어 보세요. 자세한 조정은 나중에 해도 괜찮아요.',
      },
      'clip-template': {
        title: '영상에도 나만의 흐름을 담아요.',
        description:
          '처음과 끝, 자막에 어떤 내용을 담을지 정해 두세요. 클립마다 자유롭게 바꿀 수 있어요.',
      },
      ready: {
        title: '이제, 당신의 이야기를 만들어요.',
        description:
          '준비한 설정은 다음에도 그대로예요. 나중에 바꾸고 싶다면 메뉴의 설정에서 찾아 주세요.',
        home: '만들러 가기',
        post: '첫 글 작성하기',
        clip: '첫 클립 만들기',
      },
    },
  },
  en: {
    setup: {
      checking: 'Preparing your workspace',
      loadFailed: 'Could not check your settings. Please try again.',
      retry: 'Try again',
      continue: 'Start creating now',
      later: 'Set up later',
      skip: 'Skip for now',
      back: 'Back',
      next: 'Continue',
      finish: 'Get started',
      restart: 'Revisit first-time setup',
      progress: 'Prepare your personal workspace',
      step: '{{current}} / {{total}}',
      optional: 'You can change this in settings anytime.',
      name: 'Name',
      templateName: 'Template name',
      save: 'Save and continue',
      saved: 'Saved',
      preview: 'Preview',
      modelFailed: 'Could not load AI models.',
      overview: {
        label: 'What you will prepare',
        voice: {
          title: 'A voice that fits you',
          description: 'Learn from your own writing or answers, or choose an AI-created style.',
        },
        'post-template': {
          title: 'Your familiar post structure',
          description: 'Choose an opening, photo arrangement, and ending you like.',
        },
        'clip-template': {
          title: 'The flow of your videos',
          description: 'Choose a structure from the first scene to the final caption.',
        },
      },
      welcome: {
        title: 'Make it yours, once.',
        description:
          'Set up your voice and the formats you use. Next time, go straight to creating. We will only ask for what is missing.',
        action: 'Prepare my workspace',
      },
      models: {
        title: 'Choose the AI you will create with.',
        description:
          'These models read photos, learn your writing voice and write posts. Your choices carry over to your next creation.',
        waiting: 'Choose the models you need, or set them up later.',
      },
      voice: {
        autoName: 'My writing voice',
        questionPath: 'Find my voice with 10 questions',
        questionPathHelp:
          'Answer ordinary situations briefly. You can change names and details later.',
        aiPath: 'Choose from 8 AI-created styles',
        aiPathHelp:
          'Compare different writing samples and choose a style. Generation starts only when you explicitly ask.',
        otherMethod: 'Choose another way',
        moreWays: 'Already have your own writing?',
        title: 'Let your writing sound like you.',
        description: 'Choose a comfortable way to share the words and sentence style you like.',
        name: 'Writing voice name',
        placeholder: 'For example: Everyday me',
        create: 'Prepare this writing voice',
        material: 'Show us how you usually write.',
        materialHelp:
          'Paste your writing or answer a few questions. When there is enough information, you can explicitly start the analysis.',
        confirmed: 'Your writing voice is ready.',
        use: 'Use this as my default writing voice',
        running: 'Learning your writing voice.',
        failed: 'Could not finish the analysis. Your writing is still saved.',
        createFailed: 'Could not save your writing voice. Please try again.',
      },
      'post-template': {
        title: 'Give your posts a familiar shape.',
        description:
          'Build the opening, photos and sections you often use. Fine-tune the details later.',
      },
      'clip-template': {
        title: 'Give your videos a personal flow.',
        description:
          'Choose what your opening, captions and ending say. You can change them for each clip.',
      },
      ready: {
        title: 'Your story starts here.',
        description:
          'Your settings will be here next time. To change them, open Settings from the menu.',
        home: 'Start creating',
        post: 'Write my first post',
        clip: 'Create my first clip',
      },
    },
  },
} as const satisfies I18nFragment
