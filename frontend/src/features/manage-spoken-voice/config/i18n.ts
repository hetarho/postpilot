export const manageSpokenVoiceResources = {
  ko: {
    rename: '이름 바꾸기',
    name: '목소리 이름',
    save: '이름 저장',
    changeSound: '새 목소리로 만들기',
    remove: '목록에서 제거',
    removeTitle: '{{name}} 목소리를 제거할까요?',
    removeEffect:
      '새 더빙에는 이 목소리를 선택할 수 없게 됩니다. 이미 만들어진 음성과 영상은 그대로 남아요.',
    cancel: '취소',
    failed: '변경을 저장하지 못했어요. 다시 시도해 주세요.',
    renamed: '목소리 이름을 저장했어요.',
  },
  en: {
    rename: 'Rename',
    name: 'Voice name',
    save: 'Save name',
    changeSound: 'Create a new sound',
    remove: 'Remove from list',
    removeTitle: 'Remove {{name}}?',
    removeEffect:
      'This voice will no longer be available for new narration. Existing audio and videos remain.',
    cancel: 'Cancel',
    failed: 'Could not save the change. Please try again.',
    renamed: 'Voice name saved.',
  },
} as const
