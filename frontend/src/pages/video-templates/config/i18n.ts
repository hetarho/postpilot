import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    directory: {
      chooseCreation: '영상 템플릿 만드는 방법을 골라 주세요',
      createAI: 'AI와 영상 템플릿 만들기',
      unsaved: '저장하기 전에는 영상에 사용할 수 없어요.',
      checkingWork: '저장하지 않은 영상 템플릿 작업을 확인하고 있어요.',
      workLoadFailed:
        '영상 템플릿 편집 상태를 확인하지 못했어요. 저장된 템플릿은 계속 사용할 수 있어요.',
      activeAI: '영상 템플릿 “{{name}}”의 AI 작업이 진행 중이에요.',
      publicationPending: '영상 템플릿 “{{name}}”의 저장 결과를 확인해야 해요.',
      conflict: '영상 템플릿 “{{name}}”이 다른 곳에서 바뀌었어요. 편집 내용은 유지됩니다.',
      unsavedWork: '아직 저장하지 않은 새 영상 템플릿',
      resumeNew: '영상 템플릿 “{{name}}” 이어서 만들기',
      savedAvailable: '영상에 사용할 수 있어요',
      lastPublication: '마지막으로 확인한 저장: {{result}}',
      editDesign: '영상 템플릿 “{{name}}”의 시작 디자인 편집',
      designHelp:
        '인트로·아웃트로와 자막 스타일을 바꿔요. 이름과 구성 편집은 저장된 템플릿에서 AI 편집 또는 직접 편집을 골라 주세요.',
      backToSaved: '저장된 영상 템플릿 “{{name}}” 보기',
      title: '영상 템플릿',
      description: '클립에 받을 정보, 순서대로 담을 구성과 시작 디자인을 저장해 두세요.',
      saved: '저장된 영상 템플릿',
      empty: '아직 저장된 영상 템플릿이 없어요',
      emptyHelp:
        '받을 정보와 구성의 순서, 시작 디자인을 한 번 정해 두면 이 템플릿을 고른 클립이 그대로 시작해요.',
      newDockAria: '새 영상 템플릿 만들기',
      loadFailed: '영상 템플릿을 불러오지 못했어요.',
      projectCount: '클립 {{count}}개',
      create: '새 영상 템플릿',
      updated: '수정 {{date}}',
    },
  },
  en: {
    directory: {
      chooseCreation: 'Choose how to create your video template',
      createAI: 'Create a video template with AI',
      unsaved: 'Available for videos after you save it.',
      checkingWork: 'Checking unpublished video template work.',
      workLoadFailed:
        'Could not check video template editing status. Saved templates remain available.',
      activeAI: 'AI is working on video template “{{name}}”.',
      publicationPending: 'Confirm the save result for video template “{{name}}”.',
      conflict: 'Video template “{{name}}” changed elsewhere. Your editing is retained.',
      unsavedWork: 'New video templates not yet saved',
      resumeNew: 'Continue creating video template “{{name}}”',
      savedAvailable: 'Available for videos',
      lastPublication: 'Last confirmed save: {{result}}',
      editDesign: 'Edit starting design of video template “{{name}}”',
      designHelp:
        'Change intro, outro and caption styles. Choose AI editing or direct editing on the saved template to change its name and structure.',
      backToSaved: 'View saved video template “{{name}}”',
      title: 'Video templates',
      description:
        'Save what a clip asks for, the outline it follows in order and the design it starts in.',
      saved: 'Saved video templates',
      empty: 'No video templates yet',
      emptyHelp:
        'Decide once what a clip asks for, the order it follows and the design it starts in, and a clip that picks this template starts that way.',
      newDockAria: 'Create a new video template',
      loadFailed: 'Could not load your video templates.',
      projectCount: '{{count}} clips',
      create: 'New video template',
      updated: 'Updated {{date}}',
    },
  },
} as const satisfies I18nFragment
