import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    validation: {
      loading: '검증 결과를 불러오는 중…',
      loadFailed: '검증 결과를 불러오지 못했어요.',
      wrongVoice: '이 검증은 다른 말투의 기록이에요.',
      back: '← 프로필 검증',
      title: '프로필 검증 v{{version}}',
      status: {
        queued: '대기 중',
        running: '실행 중',
        partial: '일부 완료',
        failed: '실패',
        done: '완료',
        unknown: '알 수 없음',
      },
      historyEntry: 'v{{version}} · {{status}}',
      historyEntryWithScore: 'v{{version}} · {{status}} · {{rate}}',
      retry: '실패한 항목 다시 실행',
      score: '5개 항목 일치율: {{rate}} ({{yes}}/{{total}})',
      item: '검증 글 {{index}}',
      original: '원문',
      summary: '중립 주제 요약',
      rewritten: '프로필로 다시 쓴 글',
      generating: '생성 중…',
    },
  },
  en: {
    validation: {
      loading: 'Loading validation results…',
      loadFailed: 'Could not load validation results.',
      wrongVoice: 'This validation belongs to another voice.',
      back: '← Profile validation',
      title: 'Profile validation v{{version}}',
      status: {
        queued: 'Queued',
        running: 'Running',
        partial: 'Partially complete',
        failed: 'Failed',
        done: 'Done',
        unknown: 'Unknown',
      },
      historyEntry: 'v{{version}} · {{status}}',
      historyEntryWithScore: 'v{{version}} · {{status}} · {{rate}}',
      retry: 'Retry failed items',
      score: 'Match rate across 5 items: {{rate}} ({{yes}}/{{total}})',
      item: 'Validation post {{index}}',
      original: 'Original',
      summary: 'Neutral topic summary',
      rewritten: 'Rewritten with profile',
      generating: 'Generating…',
    },
  },
} as const satisfies I18nFragment
