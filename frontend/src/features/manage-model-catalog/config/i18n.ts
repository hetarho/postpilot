import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `models` namespace (ARCH-16). */
export const i18n = {
  namespace: 'models',
  ko: {
    // Shown beside a model that takes VIDEO input — narrower than vision (VIDEO-11).
    /** The operator's grade for a model at a stage (MODEL-57). Four words that answer "which
     *  of these is the good one" without making the user read prices. */
    level: {
      value: '가성비',
      balanced: '밸런스',
      premium: '고급',
      top: '최고',
    },
    capability: { video: '영상' },
    catalog: {
      title: '모델 관리',
      description:
        'OpenRouter가 제공하는 모델 목록에서, 용도별로 사용할 모델을 직접 골라 둡니다. 각 탭에서 체크한 모델만 해당 용도의 선택 목록에 나타나고, 이미지 생성과 비디오 생성은 아직 설정만 저장됩니다.',
      disableWarning:
        '체크를 해제하면 그 용도에 이 모델을 선택해 둔 사용자의 선택도 함께 초기화되고, 사용자는 모델을 다시 골라야 합니다.',
      purposeAria: '모델 용도',
      purposeTab: {
        'photo-analysis': '사진 해석',
        'style-analysis': '문체 분석',
        writing: '글 작성',
        'image-generation': '이미지 생성',
        'video-generation': '비디오 생성',
      },
      purposeRequirement: {
        'photo-analysis':
          '이미지 입력(멀티모달)이 가능한 모델만 이 용도에 등록할 수 있어서, 그 모델들만 보여 줍니다.',
        'image-generation':
          '이미지를 출력할 수 있는 모델만 이 용도에 등록할 수 있어서, 그 모델들만 보여 줍니다.',
        'video-generation':
          '비디오를 출력할 수 있는 모델만 이 용도에 등록할 수 있어서, 그 모델들만 보여 줍니다.',
      },
      emptyForPurpose: '이 용도의 조건을 만족하는 모델이 아직 없어요.',
      search: '검색',
      searchPlaceholder: '모델 이름이나 아이디',
      provider: '제공사',
      allProviders: '전체 제공사',
      level: '등급',
      levelUnset: '등급 미지정',
      levelMissing: '등급을 아직 정하지 않았어요. 사용자 목록에서는 등급 있는 모델 뒤에 놓입니다.',
      sort: '정렬',
      sortOption: {
        level: '등급순',
        default: '기본',
        'price-asc': '가격 낮은순',
        'price-desc': '가격 높은순',
      },
      filterVision: '이미지 입력 가능',
      filterStructured: '구조화 출력 가능',
      filterEnabled: '이 용도에 등록된 모델만',
      refresh: '목록 새로고침',
      fetchedLive: '{{at, instant}}에 제공사 목록을 읽었어요.',
      fetchedCached: '{{at, instant}}에 읽어 둔 목록이에요.',
      fetchFailed:
        '제공사의 모델 목록을 읽지 못해서, 이미 등록해 둔 모델만 보여 주고 있어요. 잠시 뒤에 다시 새로고침해 주세요.',
      loading: '모델 목록을 불러오는 중…',
      loadFailed: '모델 목록을 불러오지 못했어요.',
      empty: '아직 등록된 모델도, 제공사에서 읽어 온 모델도 없어요.',
      noMatches: '조건에 맞는 모델이 없어요.',
      count: '{{total}}개 중 {{shown}}개를 보여 주고 있어요.',
      use: '이 용도에 사용',
      vision: '이미지 입력',
      structured: '구조화 출력',
      imageOutput: '이미지 생성',
      videoOutput: '비디오 생성',
      registeredPurposes: '등록된 용도: {{purposes}}',
      context: '컨텍스트 {{tokens}} 토큰',
      price: '100만 토큰당 입력 ${{in}} · 출력 ${{out}}',
      priceUnpublished: '토큰 단가 미공개',
      reasoning: '이 용도의 추론 강도',
      reasoningDefault: '단계 기본값',
      reasoningUnsetWithDefault: 'unset (모델 기본값 {{effort}})',
      reasoningDrifted:
        '이 모델이 더 이상 「{{effort}}」을 지원 목록에 두지 않아요. 값은 그대로 보내지만, 지원 목록은 {{supported}} 입니다.',
      reasoningSpend: '최근 추론 사용',
      reasoningSpendValue: '완성 토큰의 {{percent}}%가 추론에 쓰였어요 ({{calls}}회 기준)',
      reasoningTruncations: '추론이 출력 예산을 소진한 호출 {{count}}회',
      reasoningSpendHeavy:
        '설정한 강도를 이 모델이 따르지 않을 수 있어요. 강도를 낮추거나 다른 모델을 고려하세요',
    },
    document: {
      title: '일괄 편집',
      description:
        '추천 목록처럼 이미 정리된 모델 목록이 있다면, 한 줄씩 체크하는 대신 문서 하나를 붙여넣어 한 번에 반영할 수 있어요.',
      syncWarning:
        '문서에 적은 섹션은 그 용도의 최종 목록이에요. 목록에 없는 기존 등록은 해제되고, 문서에 없는 용도는 그대로 둡니다.',
      currentTitle: '지금 등록 상태',
      currentHint: '이걸 복사해 고친 뒤 아래에 붙여넣으면, 바꾸려는 것만 정확히 바꿀 수 있어요.',
      currentLoading: '불러오는 중…',
      currentFailed: '지금 등록 상태를 불러오지 못했어요.',
      pasteLabel: '붙여넣기',
      pastePlaceholder: '# postpilot models v1',
      close: '닫기',
      preview: '미리보기',
      apply: '확정',
      diffTitle: '적용하면 이렇게 바뀌어요',
      diffEmpty: '문서가 아무 용도도 지정하지 않았어요.',
      noChange: '바뀌는 것 없음',
      register: '등록 {{count}}개',
      deregister: '해제 {{count}}개',
      unchanged: '이미 등록됨 {{count}}개',
      relevel: '등급 변경 {{count}}개',
      levelUnset: '미지정',
      untouched: '문서에 없는 용도는 그대로예요: {{purposes}}',
      rejected:
        '{{count}}줄을 읽지 못해서 아무것도 반영하지 않았어요. 아래를 고치고 다시 미리보기 하세요.',
      issueLine: '{{line}}번째 줄',
      issueCause: {
        bad_version: '첫 줄은 `# postpilot models v1` 이어야 해요.',
        unknown_purpose: '없는 용도 이름이에요.',
        duplicate_section: '같은 용도가 두 번 나왔어요. 한 섹션이 그 용도의 전체 목록이에요.',
        id_before_section: '용도 섹션보다 먼저 나온 모델이에요.',
        malformed_line:
          '한 줄에 모델 아이디 하나, 뒤에 등급을 하나만 붙일 수 있어요. 표·따옴표·백틱은 넣지 마세요.',
        duplicate_id: '같은 섹션에 같은 모델이 두 번 있어요.',
        unknown_level: '등급 값이 잘못됐어요. value · balanced · premium · top 중 하나여야 합니다.',
        unknown_model: '제공사 목록에 없는 모델이에요.',
        unlisted_model: '제공사가 더 이상 제공하지 않는 모델이에요.',
        purpose_ineligible: '이 용도에 필요한 기능이 없는 모델이에요.',
        unknown: '이 줄은 반영할 수 없어요.',
      },
      fetchFailed:
        '제공사의 모델 목록을 읽지 못해서 아무것도 반영하지 않았어요. 잠시 뒤에 다시 시도해 주세요.',
      applied: '반영했어요. 등록 {{registered}}개, 해제 {{deregistered}}개.',
    },
  },
  en: {
    // Shown beside a model that takes VIDEO input — narrower than vision (VIDEO-11).
    /** The operator's grade for a model at a stage (MODEL-57). Four words that answer "which
     *  of these is the good one" without making the user read prices. */
    level: {
      value: 'Value',
      balanced: 'Balanced',
      premium: 'Premium',
      top: 'Top',
    },
    capability: { video: 'Video' },
    catalog: {
      title: 'Model catalog',
      description:
        'Choose which of the models OpenRouter offers this installation will use, per purpose. Only the models you check on a tab appear in that purpose’s picker; image and video generation are settings only for now.',
      disableWarning:
        'Unchecking a model also clears it from the selections of everyone who had chosen it for that purpose, and they must pick again.',
      purposeAria: 'Model purpose',
      purposeTab: {
        'photo-analysis': 'Photo analysis',
        'style-analysis': 'Style analysis',
        writing: 'Writing',
        'image-generation': 'Image generation',
        'video-generation': 'Video generation',
      },
      purposeRequirement: {
        'photo-analysis':
          'Only models that accept image input (multimodal) can be registered here, so only they are listed.',
        'image-generation':
          'Only models that can output images can be registered here, so only they are listed.',
        'video-generation':
          'Only models that can output video can be registered here, so only they are listed.',
      },
      emptyForPurpose: 'No model meets this purpose’s requirements yet.',
      search: 'Search',
      searchPlaceholder: 'Model name or id',
      provider: 'Provider',
      allProviders: 'All providers',
      level: 'Level',
      levelUnset: 'No level',
      levelMissing: 'No level set yet. It sorts after every graded model in the user picker.',
      sort: 'Sort',
      sortOption: {
        level: 'By level',
        default: 'Default',
        'price-asc': 'Price: low to high',
        'price-desc': 'Price: high to low',
      },
      filterVision: 'Accepts images',
      filterStructured: 'Structured output',
      filterEnabled: 'Registered for this purpose only',
      refresh: 'Refresh list',
      fetchedLive: 'Read from the provider at {{at, instant}}.',
      fetchedCached: 'Read at {{at, instant}}, served from cache.',
      fetchFailed:
        'The provider’s model list could not be read, so only models already registered are shown. Try refreshing in a moment.',
      loading: 'Loading the model list…',
      loadFailed: 'Could not load the model list.',
      empty: 'No models registered, and none read from the provider yet.',
      noMatches: 'No model matches these filters.',
      count: 'Showing {{shown}} of {{total}}.',
      use: 'Use for this purpose',
      vision: 'Image input',
      structured: 'Structured',
      imageOutput: 'Image output',
      videoOutput: 'Video output',
      registeredPurposes: 'Registered purposes: {{purposes}}',
      context: '{{tokens}} token context',
      price: '${{in}} in · ${{out}} out per 1M tokens',
      priceUnpublished: 'No published token price',
      reasoning: 'Reasoning effort for this purpose',
      reasoningDefault: 'Stage default',
      reasoningUnsetWithDefault: 'unset (model default {{effort}})',
      reasoningDrifted:
        'This model no longer lists “{{effort}}”. The value is still sent; its supported list is {{supported}}.',
      reasoningSpend: 'Recent reasoning spend',
      reasoningSpendValue:
        '{{percent}}% of completion tokens went to reasoning (over {{calls}} calls)',
      reasoningTruncations: 'Reasoning exhausted the output budget in {{count}} calls',
      reasoningSpendHeavy:
        'This model may not be honoring the effort you set. Lower it, or consider another model',
    },
    document: {
      title: 'Bulk edit',
      description:
        'When the list is already written — a recommendation, a set you keep elsewhere — paste it as one document instead of checking rows one at a time.',
      syncWarning:
        'A section is that purpose\u2019s final list: registrations it leaves out are removed, and a purpose the document does not name is left alone.',
      currentTitle: 'Currently registered',
      currentHint:
        'Copy this, edit it, and paste it below to change exactly what you mean to change.',
      currentLoading: 'Loading…',
      currentFailed: 'The current registrations could not be read.',
      pasteLabel: 'Paste',
      pastePlaceholder: '# postpilot models v1',
      close: 'Close',
      preview: 'Preview',
      apply: 'Apply',
      diffTitle: 'Applying this would',
      diffEmpty: 'The document names no purpose.',
      noChange: 'No change',
      register: 'Register {{count}}',
      deregister: 'Deregister {{count}}',
      unchanged: 'Already registered {{count}}',
      relevel: '{{count}} level change(s)',
      levelUnset: 'unset',
      untouched: 'Purposes the document does not name are left alone: {{purposes}}',
      rejected:
        '{{count}} lines could not be read, so nothing was applied. Fix them below and preview again.',
      issueLine: 'Line {{line}}',
      issueCause: {
        bad_version: 'The first line must be `# postpilot models v1`.',
        unknown_purpose: 'Not one of the five purposes.',
        duplicate_section: 'This purpose appears twice. One section is its whole list.',
        id_before_section: 'This model comes before any purpose section.',
        malformed_line:
          'One model id per line, optionally followed by one level. No tables, quotes or backticks.',
        duplicate_id: 'The same model appears twice in this section.',
        unknown_level: 'That is not a level. Use one of value · balanced · premium · top.',
        unknown_model: 'The provider does not offer this model.',
        unlisted_model: 'The provider has stopped offering this model.',
        purpose_ineligible: 'This model lacks the capability this purpose requires.',
        unknown: 'This line cannot be applied.',
      },
      fetchFailed:
        'The provider catalog could not be read, so nothing was applied. Try again in a moment.',
      applied: 'Applied. {{registered}} registered, {{deregistered}} deregistered.',
    },
  },
} as const satisfies I18nFragment
