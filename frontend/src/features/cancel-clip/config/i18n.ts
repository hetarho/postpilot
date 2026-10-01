import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    cancellation: {
      cancel: '취소',
      cancelling: '취소 중',
      cancelled: '취소됨',
      progressTitle: '클립 작업 진행',
      rule: '취소하면 확인된 AI 사용분만 차감하고 사용하지 않은 예약량은 원래 만료 시각대로 반환돼요. 추가 취소 수수료는 없어요.',
      beforeReservation:
        '아직 예약하지 않았다면 취소 비용은 0이에요. 예약 후에도 확인된 AI 사용분만 차감해요.',
      free: '다시 렌더는 취소해도 크레딧이 차감되지 않아요.',
      legacy: '이 작업은 취소 기능을 지원하지 않아요.',
      confirmTitle: '제작을 취소할까요?',
      continueProduction: '계속 제작',
      confirmCancel: '제작 취소',
      confirmHelp:
        '취소한 작업을 이어서 진행할 수는 없어요. 원본은 보관 기한까지, 이전에 완성한 영상은 그대로 유지돼요.',
      policyUnavailable: '취소 요금 정책을 확인해야 생성을 시작할 수 있어요.',
      reservation:
        '이 작업의 예약량: {{amount}}크레딧. 최종 차감과 반환량은 작업이 멈춘 뒤 확인돼요.',
      uncertain: '취소 요청 결과를 확인하지 못했어요. 서버 상태를 확인한 뒤 다시 시도할 수 있어요.',
      stopped: '작업을 취소했어요. 이전 영상은 그대로이며, 재시도는 직접 시작할 수 있어요.',
    },
  },
  en: {
    cancellation: {
      cancel: 'Cancel',
      cancelling: 'Cancelling',
      cancelled: 'Cancelled',
      progressTitle: 'Clip job progress',
      rule: 'Cancellation charges confirmed AI use only. Unused reservations return under their original expiry. There is no extra cancellation fee.',
      beforeReservation:
        'Cancellation before reservation costs zero. After reservation, only confirmed AI use is charged.',
      free: 'Cancelling a rerender costs no credits.',
      legacy: 'Cancellation is not supported for this job.',
      confirmTitle: 'Cancel production?',
      continueProduction: 'Keep going',
      confirmCancel: 'Cancel production',
      confirmHelp:
        'A cancelled job cannot be resumed. Originals stay until their retention deadline, and any previously completed video is preserved.',
      policyUnavailable: 'The cancellation policy must be available before starting.',
      reservation:
        'This job reserved {{amount}} credits. The final debit and refund are known after processing stops.',
      uncertain: 'The cancellation outcome is unknown. Check the server state before retrying.',
      stopped:
        'The job was cancelled. Your previous video is preserved; you can start a retry yourself.',
    },
  },
} as const satisfies I18nFragment
