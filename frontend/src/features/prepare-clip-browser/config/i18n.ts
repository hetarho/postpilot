import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'clips',
  ko: {
    analysisPreparation: {
      measuring: '이 기기에서 원본 길이와 프레임을 확인하는 중이에요',
      starting: '승인한 작업을 시작하는 중이에요',
      encoding: '이 기기에서 분석용 영상을 준비하는 중이에요',
      uploading: '분석용 영상을 비공개 저장소에 올리는 중이에요',
      verifying: '분석용 영상을 검증하는 중이에요',
      cancel: '준비 중단',
      refusal: {
        codec:
          '이 브라우저에서 필요한 영상·오디오 코덱을 지원하지 않아요. 지원되는 브라우저나 기기에서 다시 시도해 주세요.',
        memory:
          '이 영상은 브라우저의 준비 메모리 한도를 넘어요. 더 작은 원본이나 다른 기기에서 다시 시도해 주세요.',
        color: '이 영상의 색상 형식을 지원하지 않아요. 지원되는 SDR 원본을 선택해 주세요.',
      },
      unsupported:
        '이 브라우저나 기기에서 영상 준비를 완료할 수 없어요. 지원되는 브라우저나 기기에서 다시 시도해 주세요.',
    },
  },
  en: {
    analysisPreparation: {
      measuring: 'Checking original duration and frames on this device',
      starting: 'Starting the approved work',
      encoding: 'Preparing analysis footage on this device',
      uploading: 'Uploading analysis footage to private storage',
      verifying: 'Verifying analysis footage',
      cancel: 'Stop preparation',
      refusal: {
        codec:
          'This browser does not support the required video or audio codec. Try a supported browser or device.',
        memory:
          'This video exceeds the browser preparation memory limit. Try a smaller original or another device.',
        color: 'This video’s color format is unsupported. Select a supported SDR original.',
      },
      unsupported:
        'This browser or device could not complete video preparation. Try again with a supported browser or device.',
    },
  },
} satisfies I18nFragment
