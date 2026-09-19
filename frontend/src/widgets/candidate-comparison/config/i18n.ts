import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    comparison: {
      candidate: '후보 {{label}}',
      status: { pending: '생성 중', running: '생성 중', succeeded: '완료', failed: '오류' },
      failed: '결과를 만들지 못했어요.',
      waiting: '결과를 기다리는 중…',
      photo: '사진 · {{filename}}',
      scene: '장면',
      mood: '분위기',
      visibleText: '글자',
      objects: '사물',
      people: '사람',
      present: '있음',
      absent: '없음',
      modelUnavailable: '등록 해제된 모델',
      usageUnavailable: '사용량 미제공',
      costUnavailable: '비용 미제공',
      usage: '{{prompt}} 입력 · {{completion}} 출력 · {{latency}}ms · {{cost}}',
    },
  },
  en: {
    comparison: {
      candidate: 'Candidate {{label}}',
      status: {
        pending: 'Generating',
        running: 'Generating',
        succeeded: 'Complete',
        failed: 'Error',
      },
      failed: 'Could not create the result.',
      waiting: 'Waiting for results…',
      photo: 'Photo · {{filename}}',
      scene: 'Scene',
      mood: 'Mood',
      visibleText: 'Text',
      objects: 'Objects',
      people: 'People',
      present: 'Present',
      absent: 'None',
      modelUnavailable: 'Unregistered model',
      usageUnavailable: 'Usage unavailable',
      costUnavailable: 'Cost unavailable',
      usage: '{{prompt}} input · {{completion}} output · {{latency}}ms · {{cost}}',
    },
  },
} as const satisfies I18nFragment
