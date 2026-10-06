import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    render: {
      progress: {
        label: '브라우저 렌더 진행',
        sampling: '영상의 밝기를 확인하는 중',
        encoding: '브라우저에서 영상 만드는 중',
        storing: '완성된 영상 저장 중',
        cancelling: '브라우저 렌더 취소 중',
        cancelled: '브라우저 렌더를 취소했어요.',
        done: '브라우저 렌더를 저장했어요.',
        failed: '브라우저 렌더를 완료하지 못했어요.',
        cancel: '브라우저 렌더 취소',
      },
      refusal: {
        speech: '대본과 맞는 음성을 먼저 만들어 주세요. 더빙의 시간 배치도 확인해 주세요.',
        audio:
          '이 브라우저에서 더빙 음성을 읽거나 해독하지 못했어요. 지원하는 브라우저·기기를 이용해 주세요.',
        capability: '이 브라우저는 필요한 영상·음성 인코딩을 지원하지 않아요.',
        memory: '이 기기의 메모리가 브라우저 렌더에 부족해요.',
        sampling:
          '브라우저 렌더에 필요한 영상 밝기 확인을 마치지 못했어요. 원본을 확인하고 다시 시도해 주세요.',
      },
      choose: '어디서 렌더할까요?',
      kind: { browser: '브라우저에서 렌더', server: '서버에서 렌더' },
      kindHelp: {
        browser: '이 기기에서 바로 만들어요. 이 화면을 떠나면 멈춰요.',
        server: '서버에서 만들어요. 화면을 떠나도 계속돼요.',
      },
      serverBalance:
        '이번 달 서버 내보내기: 사용 {{used}}, 예약 {{reserved}}, 남음 {{remaining}}/{{allowance}}',
      serverExisting: '현재 저장된 서버 결과를 그대로 사용해요. 횟수가 차감되지 않아요.',
      serverRenewal: '{{at}}에 서버 내보내기 횟수가 갱신돼요.',
      serverNoAllowance: '현재 사용할 수 있는 서버 내보내기 횟수가 없어요.',
      serverMaxRequired: '새 서버 렌더링은 Max 플랜에서 이용할 수 있어요.',
      serverAccessUnknown: '현재 플랜을 확인한 뒤 서버 렌더링을 이용할 수 있어요.',
      serverUpgrade: '요금제 보기',
      serverBrowserOption: '이 기기에서는 브라우저 내보내기를 선택할 수 있어요.',
    },
  },
  en: {
    render: {
      progress: {
        label: 'Browser render progress',
        sampling: 'Checking the brightness of your footage',
        encoding: 'Rendering in your browser',
        storing: 'Storing the finished video',
        cancelling: 'Cancelling browser render',
        cancelled: 'Browser render cancelled.',
        done: 'Browser render stored.',
        failed: 'Browser render could not finish.',
        cancel: 'Cancel browser render',
      },
      refusal: {
        speech: 'Create the speech that matches the script and check its timing first.',
        audio:
          'This browser could not read or decode the narration. Try a supported browser/device.',
        capability: 'This browser does not support the required video or audio encoding.',
        memory: 'This device reports too little memory for browser rendering.',
        sampling:
          'The footage check a browser render needs did not finish. Check the originals and retry.',
      },
      choose: 'Where should this render run?',
      kind: { browser: 'Render in this browser', server: 'Render on the server' },
      kindHelp: {
        browser: 'Made right here on this device. Leaving this screen stops it.',
        server: 'Made on the server. It keeps going after you leave this screen.',
      },
      serverBalance:
        'Server exports this month: used {{used}}, reserved {{reserved}}, remaining {{remaining}}/{{allowance}}',
      serverExisting: 'Use the current stored server result. No export is spent.',
      serverRenewal: 'Server exports renew at {{at}}.',
      serverNoAllowance: 'No server exports are currently available.',
      serverMaxRequired: 'New server renders are available with Max.',
      serverAccessUnknown: 'Your current plan must be confirmed before server rendering.',
      serverUpgrade: 'See plans',
      serverBrowserOption: 'You can choose a browser export on this device.',
    },
  },
} as const satisfies I18nFragment
