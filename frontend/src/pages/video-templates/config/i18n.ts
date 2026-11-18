import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    directory: {
      editAI: 'AI와 고치기',
      editDirect: '직접 고치기',
      startDesign: '시작 디자인',
      workLoadFailed:
        '영상 템플릿 편집 상태를 확인하지 못했어요. 저장된 템플릿은 계속 사용할 수 있어요.',
      activeAI: '영상 템플릿 “{{name}}”의 AI 작업이 진행 중이에요.',
      publicationPending: '영상 템플릿 “{{name}}”의 저장 결과를 확인해야 해요.',
      conflict: '영상 템플릿 “{{name}}”이 다른 곳에서 바뀌었어요. 편집 내용은 유지됩니다.',
      resumeNew: '영상 템플릿 “{{name}}” 이어서 만들기',
      editDesign: '영상 템플릿 “{{name}}”의 시작 디자인 편집',
      backToSaved: '저장된 영상 템플릿 “{{name}}” 보기',
      title: '영상 템플릿',
      statement:
        '영상이 흘러갈 순서와 첫 화면이에요. 어떤 영상인지 AI에게 말하면 처음부터 짜 줘요.',
      new: '새 템플릿',
      saved: '저장된 영상 템플릿',
      resumeHeading: '이어서 만들던 대화',
      emptyTitle: '어떤 영상을 자주 만드세요?',
      emptyBody:
        '맛집 방문 쇼츠, 제품 언박싱, 숙소 룸투어 — 말해 주면 AI가 그 영상이 흘러갈 순서를 짜 드려요.',
      newDockAria: '새 영상 템플릿 만들기',
      loadFailed: '영상 템플릿을 불러오지 못했어요.',
      projectCount: '영상 {{count}}개',
      create: '새 영상 템플릿',
      updated: '수정 {{date}}',
    },
  },
  en: {
    directory: {
      editAI: 'Fix with AI',
      editDirect: 'Fix it yourself',
      startDesign: 'Starting design',
      workLoadFailed:
        'Could not check video template editing status. Saved templates remain available.',
      activeAI: 'AI is working on video template “{{name}}”.',
      publicationPending: 'Confirm the save result for video template “{{name}}”.',
      conflict: 'Video template “{{name}}” changed elsewhere. Your editing is retained.',
      resumeNew: 'Continue creating video template “{{name}}”',
      editDesign: 'Edit starting design of video template “{{name}}”',
      backToSaved: 'View saved video template “{{name}}”',
      title: 'Video templates',
      statement:
        'The order a video runs in and how it opens. Tell the AI what the video is and it drafts one from scratch.',
      new: 'New template',
      saved: 'Saved video templates',
      resumeHeading: 'Conversations in progress',
      emptyTitle: 'What videos do you make most?',
      emptyBody:
        'A restaurant visit short, a product unboxing, a room tour — say it and the AI drafts the order that video runs in.',
      newDockAria: 'Create a new video template',
      loadFailed: 'Could not load your video templates.',
      projectCount: '{{count}} videos',
      create: 'New video template',
      updated: 'Updated {{date}}',
    },
  },
} as const satisfies I18nFragment
