import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `templates` namespace (ARCH-16). */
export const i18n = {
  namespace: 'templates',
  ko: {
    create: {
      name: '템플릿 이름',
      namePlaceholder: '예: 정보성 식당 리뷰',
      description: '어떤 글인가요',
      descriptionPlaceholder: '예: 식사를 제공받고 쓰는 방문 리뷰',
      body: '템플릿 구성',
      help: '템플릿은 글의 구성과 순서를 정해요. 문체와 종결어미는 그대로 말투 프로필을 따릅니다.',
      submit: '템플릿 만들기',
    },
    numbers: {
      targetLengthTick: '목표 글자 수 사용',
      targetLength: '목표 글자 수',
      targetLengthHelp:
        '이 템플릿을 고르면 글의 목표 글자 수가 이 값으로 채워져요. 글에서 바꾸면 바꾼 값이 유지됩니다.',
      tagCountTick: '태그 수 사용',
      tagCount: '태그 수',
      tagCountHelp:
        '이 템플릿을 고르면 글의 태그 수가 이 값으로 채워져요. 비워 두면 글의 설정을 그대로 둡니다.',
      range: '{{min}}에서 {{max}} 사이로 적어 주세요.',
    },
    screen: {
      mode: {
        aria: '구성 편집 방식',
        builder: '블록',
        source: '원문',
      },
      sourceHelp:
        '템플릿을 저장 형식 그대로 보고 고쳐요. 형식 안내를 AI에게 건네 만든 템플릿을 여기에 붙여 넣을 수 있어요.',
      backToList: '← 템플릿 목록',
      newTitle: '새 템플릿',
      notFound: '이 템플릿을 찾을 수 없어요. 목록에서 다시 골라 주세요.',
      compositionHelp:
        '위에서 블록을 더해 글의 순서를 짜세요. 줄을 누르면 내용을 고칠 수 있고, 끌어서 순서를 바꿀 수 있어요.',
      titleArea: {
        heading: '제목 형식',
        help: '비워 두면 제목은 AI가 정해요. 채우면 글 제목이 이 형식을 따라요.',
      },
      saved: '저장했어요.',
      saveDockAria: '템플릿 저장',
      leaveTitle: '저장하지 않고 나갈까요?',
      leaveDescription: '고친 내용이 아직 저장되지 않았어요. 지금 나가면 사라집니다.',
      leaveConfirm: '저장하지 않고 나가기',
    },
  },
  en: {
    create: {
      name: 'Template name',
      namePlaceholder: 'For example: Informational restaurant review',
      description: 'What kind of post is this?',
      descriptionPlaceholder: 'For example: A hosted restaurant visit review',
      body: 'Template structure',
      help: 'A template decides the structure and order. Style and endings still follow the voice profile.',
      submit: 'Create template',
    },
    numbers: {
      targetLengthTick: 'Set a target length',
      targetLength: 'Target length',
      targetLengthHelp:
        'Picking this template fills the post’s target length with this value. Changing it on the post keeps what you changed.',
      tagCountTick: 'Set a tag count',
      tagCount: 'Tag count',
      tagCountHelp:
        'Picking this template fills the post’s tag count with this value. Leave it off to keep whatever the post already has.',
      range: 'Enter a number between {{min}} and {{max}}.',
    },
    screen: {
      mode: {
        aria: 'How to edit',
        builder: 'Blocks',
        source: 'Source',
      },
      sourceHelp:
        'See and edit the template in its stored format. Hand the format guide to an AI and paste what it writes here.',
      backToList: '← Templates',
      newTitle: 'New template',
      notFound: 'Could not find this template. Pick one from the list again.',
      compositionHelp:
        'Add blocks above to lay out the post. Tap a line to edit it, and drag to reorder.',
      titleArea: {
        heading: 'Title format',
        help: 'Leave it empty and the AI writes the title. Fill it in and the post title follows this format.',
      },
      saved: 'Saved.',
      saveDockAria: 'Save template',
      leaveTitle: 'Leave without saving?',
      leaveDescription: 'Your changes have not been saved yet. Leaving now discards them.',
      leaveConfirm: 'Leave without saving',
    },
  },
} as const satisfies I18nFragment
