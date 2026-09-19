import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `voices` namespace (ARCH-16). */
export const i18n = {
  namespace: 'voices',
  ko: {
    assignment: {
      jobBlocked: 'AI 작업이 끝나면 말투를 바꿀 수 있어요.',
      experimentBlocked: '대기 중인 A/B 결과를 먼저 확인하면 말투를 바꿀 수 있어요.',
      blocked:
        '지금은 말투를 바꿀 수 없어요. 진행 중인 AI 작업이 끝났는지, 고른 말투가 아직 있는지 확인해 주세요.',
      notFound: '고른 말투를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요.',
      notFoundDetail:
        '고른 말투를 찾을 수 없어요. 목록을 새로 고친 뒤 다시 시도해 주세요. {{error}}',
      failed: '말투를 바꾸지 못했어요. 다시 시도해 주세요.',
      title: '말투를 바꿀까요?',
      confirm: '말투 변경',
      description:
        '‘{{name}}’(으)로 바꿉니다. 제목, 메모, 사진, 본문과 확정 상태는 그대로 남아요. 지금까지 배운 내용은 이전 말투에 남고, 새 말투로 학습하려면 먼저 AI 생성이나 수정으로 새 결과를 만들어야 해요.',
    },
  },
  en: {
    assignment: {
      jobBlocked: 'You can change the voice after the AI job finishes.',
      experimentBlocked: 'Review the pending A/B result before changing the voice.',
      blocked:
        'The voice cannot be changed now. Check that the AI job has finished and the selected voice still exists.',
      notFound: 'The selected voice could not be found. Refresh the list and try again.',
      notFoundDetail:
        'The selected voice could not be found. Refresh the list and try again. {{error}}',
      failed: 'Could not change the voice. Try again.',
      title: 'Change the voice?',
      confirm: 'Change voice',
      description:
        'This changes the post to “{{name}}”. Its title, notes, photos, content, and finalized state stay unchanged. Previous learning remains with the old voice. Generate or revise the post with the new voice before learning from it.',
    },
  },
} as const satisfies I18nFragment
