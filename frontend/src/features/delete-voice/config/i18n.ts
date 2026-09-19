import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    delete: {
      aria: '{{name}} 삭제',
      title: '이 말투를 삭제할까요?',
      description:
        '‘{{name}}’로 쓴 글과 학습 기록은 그대로 남아요. 그 글에는 삭제된 말투로 표시되고, 복원하거나 다른 말투로 바꾸기 전에는 AI 생성·수정·학습을 할 수 없어요. 새 글의 말투 목록에서는 바로 사라집니다.',
    },
    error: {
      nameLength: '이름은 공백을 빼고 1~{{max}}자로 입력해 주세요.',
      nameExists: '같은 이름의 말투가 이미 있어요.',
      createFailed: '말투를 만들지 못했어요. 다시 시도해 주세요.',
      deleteBlocked:
        '지금은 삭제할 수 없어요. 기본 말투이거나, 진행 중이거나 아직 결정하지 않은 작업이 있어요.',
      deleteBlockedDetail:
        '지금은 삭제할 수 없어요. 기본 말투이거나, 진행 중이거나 아직 결정하지 않은 작업이 있어요. {{error}}',
      notFound: '말투를 찾을 수 없어요. 목록을 새로 고쳐 주세요.',
      deleteFailed: '말투를 삭제하지 못했어요. 다시 시도해 주세요.',
      renameFailed: '이름을 바꾸지 못했어요. 다시 시도해 주세요.',
      restoreNameConflict: '같은 이름의 말투가 이미 있어요. 이름을 바꾼 뒤 복원해 주세요.',
      restoreNameConflictDetail:
        '같은 이름의 말투가 이미 있어요. 이름을 바꾼 뒤 복원해 주세요. {{error}}',
      restoreFailed: '말투를 복원하지 못했어요. 다시 시도해 주세요.',
      defaultDeleted: '삭제된 말투는 기본으로 설정할 수 없어요. 먼저 복원해 주세요.',
      defaultFailed: '기본 말투를 바꾸지 못했어요. 다시 시도해 주세요.',
    },
  },
  en: {
    delete: {
      aria: 'Delete {{name}}',
      title: 'Delete this voice?',
      description:
        'Posts written in “{{name}}” and its learning history remain. Those posts will show a deleted voice and cannot use AI generation, revision, or learning until you restore or change the voice. It disappears from the voice list for new posts immediately.',
    },
    error: {
      nameLength: 'Enter a name between 1 and {{max}} characters after trimming spaces.',
      nameExists: 'A voice with this name already exists.',
      createFailed: 'Could not create the voice. Try again.',
      deleteBlocked:
        'This voice cannot be deleted now. It may be the default or have an active or undecided job.',
      deleteBlockedDetail:
        'This voice cannot be deleted now. It may be the default or have an active or undecided job. {{error}}',
      notFound: 'The voice could not be found. Refresh the list.',
      deleteFailed: 'Could not delete the voice. Try again.',
      renameFailed: 'Could not rename the voice. Try again.',
      restoreNameConflict:
        'A voice with this name already exists. Rename this voice before restoring it.',
      restoreNameConflictDetail:
        'A voice with this name already exists. Rename this voice before restoring it. {{error}}',
      restoreFailed: 'Could not restore the voice. Try again.',
      defaultDeleted: 'A deleted voice cannot be the default. Restore it first.',
      defaultFailed: 'Could not change the default voice. Try again.',
    },
  },
} as const satisfies I18nFragment
