import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    recommendationSets: {
      title: '추천 조합',
      description:
        '모델 변경 화면에서 모든 계정에 보여 줄 추천 조합입니다. 적용한 계정의 선택은 그 순간의 조합을 복사한 것이라, 여기서 고치거나 지워도 이미 적용한 계정의 선택은 바뀌지 않습니다.',
      loading: '추천 조합을 불러오는 중…',
      loadFailed: '추천 조합을 불러오지 못했어요.',
      empty: '아직 추천 조합이 없어요. 추가하면 모델 변경 화면에 이 순서대로 보입니다.',
      add: '조합 추가',
      full: '추천 조합은 {{limit}}개까지 만들 수 있어요. 하나를 지운 뒤 추가하세요.',
      edit: '수정',
      editLabel: '{{label}} 수정',
      up: '위로',
      upLabel: '{{label}} 위로 옮기기',
      down: '아래로',
      downLabel: '{{label}} 아래로 옮기기',
      delete: '삭제',
      deleteLabel: '{{label}} 삭제',
      deleteTitle: '이 추천 조합을 삭제할까요?',
      deleteBody:
        '"{{label}}" 조합이 모델 변경 화면에서 사라집니다. 이 조합을 이미 적용한 계정의 선택은 그대로 남습니다.',
      deleteConfirm: '삭제',
      stageGroup: { observe: '사진 관찰', analyze: '문체 분석', write: '글 작성' },
      slot: { active: '활성', candidateA: 'A', candidateB: 'B' },
      slotLabel: '{{stage}} {{slot}}',
      flag: {
        unregistered: '등록 해제됨',
        unclassified: '미분류',
      },
      editor: {
        newTitle: '새 추천 조합',
        editTitle: '추천 조합 수정',
        label: '이름',
        choose: '모델 선택',
        retired: '{{model}} · 지금은 고를 수 없는 모델',
        save: '저장',
        cancel: '취소',
      },
      cause: {
        required: '값을 채워 주세요.',
        too_long: '{{max}}자까지 쓸 수 있어요.',
        duplicate: 'A와 다른 모델을 고르세요.',
        labelTaken: '다른 조합이 이미 쓰는 이름이에요.',
        unregistered: '이 단계의 용도에 등록되지 않은 모델이에요.',
        unclassified: '운영자가 아직 등급을 분류하지 않은 모델이에요.',
      },
    },
  },
  en: {
    recommendationSets: {
      title: 'Recommended sets',
      description:
        'The sets every account is offered on the model settings screen. Applying one copies the set as it is at that moment, so editing or deleting it here leaves accounts that already applied it unchanged.',
      loading: 'Loading the recommended sets…',
      loadFailed: 'Could not load the recommended sets.',
      empty:
        'No recommended sets yet. Sets you add appear on the model settings screen in this order.',
      add: 'Add a set',
      full: 'You can keep up to {{limit}} sets. Delete one before adding another.',
      edit: 'Edit',
      editLabel: 'Edit {{label}}',
      up: 'Up',
      upLabel: 'Move {{label}} up',
      down: 'Down',
      downLabel: 'Move {{label}} down',
      delete: 'Delete',
      deleteLabel: 'Delete {{label}}',
      deleteTitle: 'Delete this recommended set?',
      deleteBody:
        '"{{label}}" disappears from the model settings screen. Accounts that already applied it keep their choices.',
      deleteConfirm: 'Delete',
      stageGroup: { observe: 'Photo observation', analyze: 'Voice analysis', write: 'Writing' },
      slot: { active: 'Active', candidateA: 'A', candidateB: 'B' },
      slotLabel: '{{stage}} {{slot}}',
      flag: {
        unregistered: 'Deregistered',
        unclassified: 'Unclassified',
      },
      editor: {
        newTitle: 'New recommended set',
        editTitle: 'Edit recommended set',
        label: 'Name',
        choose: 'Choose a model',
        retired: '{{model}} · cannot be chosen now',
        save: 'Save',
        cancel: 'Cancel',
      },
      cause: {
        required: 'Fill this in.',
        too_long: 'Use at most {{max}} characters.',
        duplicate: 'Choose a model other than A.',
        labelTaken: 'Another set already uses this name.',
        unregistered: 'This model is not registered to this stage’s purpose.',
        unclassified: 'This model has no operator classification yet.',
      },
    },
  },
} as const satisfies I18nFragment
