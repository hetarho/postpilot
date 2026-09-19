import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `clips` namespace (ARCH-16). */
export const i18n = {
  namespace: 'clips',
  ko: {
    generation: {
      responseRetryCount: '{{stage}} ({{current}}/{{total}})',
      generate: '생성',
      retry: '다시 생성',
      running: '클립을 만드는 중이에요',
      stage: {
        flow_retry: '컷 구성 응답 형식 다시 확인 중',
        narrate_retry: '자막 응답 형식 다시 확인 중',
        flow: '컷 구성',
        narrate: '자막 작성',
        analyze_retry: '영상 분석 응답 형식 다시 확인 중',
        plan_retry: '컷·자막 응답 형식 다시 확인 중',

        prepare: '원본 준비 중',
        analyze: '영상 분석',
        plan: '컷·자막 구성',
        render: '영상 렌더링',
        save: '저장 중',
        cleanup: '원본 정리',
      },
      failedAt: '{{stage}} 단계에서 실패했어요',
      finished: '클립이 완성됐어요',
      models: '생성 모델',
      eligibility: {
        loading: '관찰 모델이 클립 분석에 쓸 수 있는지 확인하는 중이에요.',
        failed: '관찰 모델이 클립 분석에 쓸 수 있는지 확인하지 못했어요. 다시 확인해 주세요.',
        retry: '다시 확인',
        unresolved: '선택한 관찰 모델은 클립 분석에 아직 쓸 수 없어요. 다른 모델을 선택해 주세요.',
        reason: {
          video_input_absent: '영상 입력을 받지 않는 모델이에요',
          inline_endpoint_unavailable: '지금 클립 영상을 그대로 받을 수 있는 경로가 없어요',
          required_parameters_unsupported: '클립 분석 요청에 필요한 설정을 지원하지 않는 경로예요',
          price_ceiling_unavailable: '요금 상한을 확인할 수 없는 경로예요',
        },
      },
      selectModels: '관찰 모델과 작성 모델을 선택해 주세요.',
      creditPolicy:
        '표시된 최대 크레딧을 승인해야 생성해요. 모든 원본과 분석용 영상을 검증한 뒤 필요한 크레딧을 한 번 예약하며, 승인액과 예약액을 넘겨 청구하지 않아요. 실패한 작업에 확인된 유료 사용이 없으면 기본 비용도 차감하지 않아요.',
      plans: '크레딧·요금제 확인',
      reselection: '다시 만들려면 원본 영상을 새로 선택해 주세요. 이전 원본은 보관하지 않아요.',
      pollingFailed: '작업 상태를 불러오지 못했어요. 새 작업을 시작하지 않고 다시 확인해 주세요.',
      result: '완성된 클립',
      preview: '클립 미리보기',
      download: '영상 다운로드',
      previewFailed: '영상을 불러오지 못했어요. 다시 불러오거나 다운로드해 주세요.',
      refreshing: '미리보기 링크를 새로 불러오는 중이에요',
      hasResult: '영상 있음',
      noResult: '아직 생성 전',
    },
  },
  en: {
    generation: {
      responseRetryCount: '{{stage}} ({{current}}/{{total}})',
      generate: 'Generate',
      retry: 'Generate again',
      running: 'Creating your clip',
      stage: {
        flow_retry: 'Correcting the flow response format',
        narrate_retry: 'Correcting the narration response format',
        flow: 'Footage flow',
        narrate: 'Narration',
        analyze_retry: 'Correcting the analysis response format',
        plan_retry: 'Correcting the composition response format',

        prepare: 'Preparing originals',
        analyze: 'Analyze footage',
        plan: 'Compose cuts and captions',
        render: 'Render video',
        save: 'Saving',
        cleanup: 'Clean up sources',
      },
      failedAt: 'Failed during: {{stage}}',
      finished: 'Your clip is ready',
      models: 'Generation models',
      eligibility: {
        loading: 'Checking whether the observation model can analyse clips.',
        failed: 'Could not check whether the observation model can analyse clips. Check again.',
        retry: 'Check again',
        unresolved:
          'The selected observation model cannot analyse clips yet. Choose another model.',
        reason: {
          video_input_absent: 'does not take video input',
          inline_endpoint_unavailable: 'has no route right now that accepts the clip video inline',
          required_parameters_unsupported:
            'has no route that supports the settings the clip analysis request needs',
          price_ceiling_unavailable: 'has no route with a confirmable price ceiling',
        },
      },
      selectModels: 'Choose an observation model and a writing model.',
      creditPolicy:
        'Generation requires approval of the displayed maximum. After every original and analysis copy is verified, credits are reserved once. Your charge never exceeds either approval or reservation. Failed work without confirmed billable usage costs nothing, including the base charge.',
      plans: 'Check credits and plans',
      reselection:
        'Select your source videos again to generate another clip. Previous originals are not retained.',
      pollingFailed: 'Unable to check this job. Check again without starting a new job.',
      result: 'Finished clip',
      preview: 'Clip preview',
      download: 'Download video',
      previewFailed: 'Unable to load the video. Reload it or download the result.',
      refreshing: 'Refreshing the preview link',
      hasResult: 'Video ready',
      noResult: 'Not generated yet',
    },
  },
} as const satisfies I18nFragment
