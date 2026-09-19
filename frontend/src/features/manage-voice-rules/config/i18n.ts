import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    rules: {
      title: '대조 규칙',
      empty: '아직 편집 패턴에서 찾은 규칙이 없어요.',
      evidence: '{{status}} · 근거 {{count}}',
      evidence_one: '{{status}} · 근거 {{count}}',
      evidence_other: '{{status}} · 근거 {{count}}',
      activate: '활성화',
      retire: '사용 중지',
      compare: '블라인드 비교',
      conflicts: '확인이 필요한 충돌',
      current: '현재: {{statement}}',
      proposed: '새 근거: {{statement}}',
      keep: '현재 규칙 유지',
      replace: '새 규칙으로 교체',
      compareTitle: '이 규칙만 비교할까요?',
      compareConfirm: '비교 작업 시작',
      compareDescription:
        '같은 입력과 같은 작성 모델로 두 글을 만들고, 선택한 규칙의 포함 여부만 다르게 합니다. 판정 전에는 어느 쪽에 규칙이 들어갔는지 숨깁니다.',
      status: {
        candidate: '후보',
        active: '활성',
        retired: '중지',
        rejected: '거절',
        unknown: '알 수 없음',
      },
      layer: {
        lexical: '어휘',
        endings: '종결어미',
        syntax: '구문',
        structure: '글 구성',
        axes: '성향',
        unknown: '알 수 없음',
      },
    },
  },
  en: {
    rules: {
      title: 'Contrast rules',
      empty: 'No rules have been found from editing patterns yet.',
      evidence: '{{status}} · {{count}} evidence',
      evidence_one: '{{status}} · {{count}} evidence',
      evidence_other: '{{status}} · {{count}} evidence',
      activate: 'Activate',
      retire: 'Retire',
      compare: 'Blind comparison',
      conflicts: 'Conflicts to review',
      current: 'Current: {{statement}}',
      proposed: 'New evidence: {{statement}}',
      keep: 'Keep current rule',
      replace: 'Replace with new rule',
      compareTitle: 'Compare only this rule?',
      compareConfirm: 'Start comparison',
      compareDescription:
        'Two posts will use the same input and writing model, differing only in whether this rule is included. Which candidate used the rule stays hidden until you decide.',
      status: {
        candidate: 'Candidate',
        active: 'Active',
        retired: 'Retired',
        rejected: 'Rejected',
        unknown: 'Unknown',
      },
      layer: {
        lexical: 'Lexical',
        endings: 'Endings',
        syntax: 'Syntax',
        structure: 'Structure',
        axes: 'Axes',
        unknown: 'Unknown',
      },
    },
  },
} as const satisfies I18nFragment
