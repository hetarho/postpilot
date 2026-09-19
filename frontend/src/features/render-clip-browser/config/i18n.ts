import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    render: {
      progress: {
        label: '브라우저 렌더 진행',
        encoding: '브라우저에서 영상 만드는 중',
        storing: '완성된 영상 저장 중',
        cancelling: '브라우저 렌더 취소 중',
        cancelled: '브라우저 렌더를 취소했어요.',
        done: '브라우저 렌더를 저장했어요.',
        failed: '브라우저 렌더를 완료하지 못했어요.',
        cancel: '브라우저 렌더 취소',
      },
      refusal: {
        capability:
          '이 브라우저는 필요한 영상·음성 인코딩을 지원하지 않아요. 서버 렌더를 선택해 주세요.',
        memory: '이 기기의 메모리가 브라우저 렌더에 부족해요. 서버 렌더를 선택해 주세요.',
      },
      choose: '어디서 렌더할까요?',
      kind: { browser: '브라우저에서 렌더', server: '서버에서 렌더' },
      kindHelp: {
        browser: '이 기기에서 바로 만들어요. 이 화면을 떠나면 멈춰요.',
        server: '서버에서 만들어요. 화면을 떠나도 계속돼요.',
      },
    },
  },
  en: {
    render: {
      progress: {
        label: 'Browser render progress',
        encoding: 'Rendering in your browser',
        storing: 'Storing the finished video',
        cancelling: 'Cancelling browser render',
        cancelled: 'Browser render cancelled.',
        done: 'Browser render stored.',
        failed: 'Browser render could not finish.',
        cancel: 'Cancel browser render',
      },
      refusal: {
        capability:
          'This browser does not support the required video or audio encoding. Choose server rendering.',
        memory:
          'This device reports too little memory for browser rendering. Choose server rendering.',
      },
      choose: 'Where should this render run?',
      kind: { browser: 'Render in this browser', server: 'Render on the server' },
      kindHelp: {
        browser: 'Made right here on this device. Leaving this screen stops it.',
        server: 'Made on the server. It keeps going after you leave this screen.',
      },
    },
  },
} as const satisfies I18nFragment
