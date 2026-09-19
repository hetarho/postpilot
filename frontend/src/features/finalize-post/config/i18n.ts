import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `posts` namespace (ARCH-16). */
export const i18n = {
  namespace: 'posts',
  ko: {
    finalize: {
      title: '확정',
      already: '이 내용은 이미 확정했어요. 글 완성에서 내보내거나 발행할 수 있습니다.',
      description: '다 다듬었다면 지금 내용을 확정해 주세요. 확정하면 글 완성으로 넘어갑니다.',
      failed: '글을 확정하지 못했어요.',
      goFinish: '글 완성으로 가기',
      analyzeHelp:
        '말투 학습을 하려면 분석 모델을 선택해 주세요. 확정만 하는 데에는 필요하지 않아요.',
      open: '확정하기',
      action: '확정',
      actionLearn: '확정하고 말투 학습',
      optionOnlyHint:
        '지금 편집 내용을 저장하고 그 revision을 확정한 뒤 글 완성으로 넘어갑니다. 모델 호출이나 말투 학습은 하지 않아요.',
      optionLearnHint:
        '확정한 다음, 이 글로 말투 학습까지 시작합니다. 결과는 글 완성에서 알려드려요.',
    },
    learning: {
      title: '말투 학습',
      description:
        '확정과 말투 학습은 별개예요. 확정만 해도 글은 완료되며, 학습은 버튼을 눌렀을 때만 시작합니다.',
      statusFailed: '학습 상태를 확인하지 못했어요.',
      learned: '이 글에서 말투를 배웠어요.',
      startFailed: '글은 확정됐지만 말투 학습은 시작하지 못했어요.',
      startFailedDetail: '글은 확정됐지만 말투 학습은 시작하지 못했어요. {{error}}',
      notFinalized:
        '아직 확정하지 않은 내용이에요. 글 다듬기에서 확정하면 이 글로 말투를 배울 수 있어요.',
      needAnalyze: '말투 학습을 하려면 분석 모델을 선택해 주세요.',
      action: '말투 학습',
      goRefine: '글 다듬기로 가기',
      satisfied: '수정 없이도 마음에 들어요',
      missingJob: '학습 작업 정보가 비어 있어요.',
      blocked: {
        otherVoice:
          '이 글의 AI 결과는 다른 말투에서 만들어졌어요. 새 말투로 다시 생성하거나 수정한 뒤에 학습할 수 있어요.',
        noBaseline: '새 말투로 다시 생성하거나 AI로 수정한 뒤에 학습할 수 있어요.',
      },
    },
  },
  en: {
    finalize: {
      title: 'Finalize',
      already: 'This content is already finalized. You can export or publish it from Finish.',
      description: 'When you are done refining, finalize this content to continue to Finish.',
      failed: 'Could not finalize the post.',
      goFinish: 'Go to Finish',
      analyzeHelp:
        'Select an analysis model to learn the voice. Finalizing alone does not require one.',
      open: 'Finalize…',
      action: 'Finalize',
      actionLearn: 'Finalize and learn voice',
      optionOnlyHint:
        'Saves your current edits, finalizes that revision, and continues to Finish. No model call or voice learning will run.',
      optionLearnHint:
        'Finalizes first, then starts voice learning from this post. The outcome is reported on Finish.',
    },
    learning: {
      title: 'Voice learning',
      description:
        'Finalizing and voice learning are separate. Finalizing completes the post; learning starts only when you choose it.',
      statusFailed: 'Could not check voice-learning status.',
      learned: 'The voice learned from this post.',
      startFailed: 'The post was finalized, but voice learning could not start.',
      startFailedDetail: 'The post was finalized, but voice learning could not start. {{error}}',
      notFinalized:
        'This content is not finalized yet. Finalize it in Refine before learning from it.',
      needAnalyze: 'Select an analysis model to learn the voice.',
      action: 'Learn voice',
      goRefine: 'Go to Refine',
      satisfied: 'I like it without changes',
      missingJob: 'Voice-learning job information is missing.',
      blocked: {
        otherVoice:
          'This AI result was created with another voice. Generate or revise it with the new voice before learning.',
        noBaseline: 'Generate or revise the post with the new voice before learning.',
      },
    },
  },
} as const satisfies I18nFragment
