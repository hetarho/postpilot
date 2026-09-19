import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `guidelines` namespace (ARCH-16). */
export const i18n = {
  namespace: 'guidelines',
  ko: {
    candidate: {
      summary: '지침 후보 {{count}}개',
      approveAll: '전부 수락',
      dismissAll: '전부 거절',
      dismissAllTitle: '후보 {{count}}개를 전부 거절할까요?',
      dismissAllDescription:
        '고른 후보를 모두 무시 처리해요. 되돌릴 수 없고, 같은 요청이 다시 올라오지도 않아요. 지침은 언제든 직접 쓸 수 있어요.',
      approveAllResult: '{{count}}개를 지침으로 저장했어요. {{left}}개는 그대로 남았어요.',
      dismissAllResult: '{{count}}개를 거절했어요.',
      section: '후보 지침',
      sectionHelp:
        'AI 수정을 끝낼 때 보낸 요청을 그대로 모아 둔 목록이에요. 여기 있는 동안에는 어떤 글에도 적용되지 않고, 승인할 때 적용 범위를 고르면 지침으로 저장돼요.',
      occurrences: '{{count}}번 요청함',
      occurrences_one: '{{count}}번 요청함',
      occurrences_other: '{{count}}번 요청함',
      source: '요청한 글 보기',
      sourceGone: '요청한 글이 삭제됐어요',
      queueFull:
        '후보가 가득 차서 새로 들어온 요청은 기록되지 않아요. 하나를 승인하거나 무시하면 다시 기록돼요.',
      approve: '승인',
      dismiss: '무시',
      approveTitle: '지침으로 승인',
      approveSubmit: '지침으로 저장',
      approveDescription:
        '이 요청을 지침으로 저장해요. 문장을 다듬을 수 있고, 적용 범위는 여기서 고릅니다.',
      approveDuplicate: '이미 같은 지침이 있어요. 문장을 바꾸거나 이 후보를 무시해 주세요.',
    },
  },
  en: {
    candidate: {
      summary: 'Guideline candidates ({{count}})',
      approveAll: 'Accept all',
      dismissAll: 'Dismiss all',
      dismissAllTitle: 'Dismiss all {{count}} candidates?',
      dismissAllDescription:
        'Every candidate here is marked dismissed. There is no undo, and the same request will not come back. You can still write the guideline by hand.',
      approveAllResult: 'Saved {{count}} as guidelines. {{left}} were left here.',
      dismissAllResult: 'Dismissed {{count}}.',
      section: 'Candidates',
      sectionHelp:
        'What your finished AI revisions asked for, recorded word for word. Nothing here reaches a post while it sits in this list — approving one is where you choose what it applies to.',
      occurrences: 'asked {{count}} times',
      occurrences_one: 'asked {{count}} time',
      occurrences_other: 'asked {{count}} times',
      source: 'View the post',
      sourceGone: 'That post was deleted',
      queueFull:
        'The candidate list is full, so new requests are no longer recorded. Approve or dismiss one to start recording again.',
      approve: 'Approve',
      dismiss: 'Dismiss',
      approveTitle: 'Approve as a guideline',
      approveSubmit: 'Save as a guideline',
      approveDescription:
        'Save this request as a guideline. You can reword it, and you choose what it applies to here.',
      approveDuplicate: 'That guideline already exists. Reword it, or dismiss this candidate.',
    },
  },
} as const satisfies I18nFragment
