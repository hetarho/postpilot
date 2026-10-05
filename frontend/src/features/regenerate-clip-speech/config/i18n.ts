import type { I18nFragment } from '@/shared/lib'
export const i18n = {
  namespace: 'clips',
  ko: {
    dubbing: {
      retainedDraft: '보존된 더빙 대본',
      retainedHelp:
        '중단된 생성의 대본과 완료된 음성이 남아 있습니다. 수정 후 저장하고 ①에서 이어서 생성하면 호환 음성을 재사용합니다. 저장은 음성을 생성하지 않습니다.',
      saveRetained: '수정 대본 저장',
      deliveryPending: '요청한 더빙 음성을 생성하거나 시간을 조정한 뒤 출력할 수 있습니다.',
      enabled: '더빙 사용',
      voice: '확정 목소리',
      choose: '목소리를 선택하세요',
      voiceUnavailable:
        '이 목소리는 선택 목록에 없습니다. 기존 음성은 보존되지만 새 합성에는 사용 가능한 목소리가 필요합니다.',
      createVoice: '목소리 만들기',
      words: '더빙 대본',
      start: '더빙 시작 (초)',
      end: '더빙 끝 (초)',
      split: '대본 커서에서 분할',
      remove: '대본 구간 삭제',
      audioRevision: '대본 {{revision}}판의 음성',
      previousAudioRevision: '현재 대본 {{revision}}판 · 변경 전 음성',
      sourceVolume: '원본 소리 음량',
      voiceVolume: '더빙 음량',
      independent: '대본과 자막은 따로 편집합니다. 입력·음량 조절은 음성을 생성하지 않습니다.',
      segment: '대본 {{number}}',
      select: '타임라인에서 선택',
      add: '대본 구간 추가',
      limits:
        '각 구간은 공백이 아닌 500자 이하, 전체 2000자·32구간 이하이며 끝은 시작 뒤여야 합니다.',
      pending: '재생성 필요한 구간 {{count}}개',
      estimate: '변경 음성 견적 보기',
      conflict: '음성이 지정한 구간에 맞지 않습니다. 대본을 수정하거나 시간 조정안을 검토하세요.',
      reviewRetiming: '시간 조정안 검토',
      reviseScript: '대본 수정으로 해결',
      approval: '변경 음성 생성 승인',
      close: '닫기',
      refreshQuote: '견적 새로 받기',
      approve: '최대 {{credits}} 크레딧으로 생성',
      quoted: '완료된 음성은 재사용하고 {{count}}개 구간만 생성합니다.',
      quoteChanged: '대본 또는 견적 유효 시간이 바뀌었습니다. 새 견적을 받으세요.',
      applyRetiming: '검토한 조정안 적용',
      proposal:
        '전체 {{before}} → {{after}}초. 원본 컷 {{cuts}}개를 연장하고 자막 {{captions}}개의 기준만 출력 시각으로 고정합니다.',
      protected:
        '자막 문구와 출력 시각, 원본 재생 속도를 보존합니다. 변경안은 한 번에 실행 취소할 수 있습니다.',
      play: '더빙 재생',
      playPrevious: '이전 대본 음성 재생',
      stop: '재생 중지',
      audioUnavailable:
        '이 음성을 재생할 수 없습니다. 음성을 다시 생성하지 않고 재생을 다시 시도할 수 있습니다.',
      states: {
        ready: '음성 준비됨',
        stale: '재생성 필요',
        missing: '음성 없음',
        conflict: '시간 조정 필요',
      },
      retimingReasons: {
        speech: '누락되거나 오래된 음성을 먼저 생성해야 조정안을 만들 수 있습니다.',
        duration: '자연 속도 음성이 선택한 길이 또는 60초를 넘습니다. 대본을 수정하세요.',
        footage:
          '관찰된 원본의 사용 가능한 구간이 부족합니다. 대본을 수정하거나 원본을 보완하세요.',
        captions: '기존 자막·영역 시각을 보존하면서 조정할 수 없습니다. 대본을 수정하세요.',
      },
    },
  },
  en: {
    dubbing: {
      retainedDraft: 'Retained spoken draft',
      retainedHelp:
        'The stopped run retains its script and completed speech. Save your edits, then resume generation from step 1 to reuse compatible audio. Saving never generates speech.',
      saveRetained: 'Save edited script',
      deliveryPending: 'Generate the requested narration or resolve its timing before export.',
      enabled: 'Use dubbing',
      voice: 'Confirmed voice',
      choose: 'Choose a voice',
      voiceUnavailable:
        'This voice is unavailable for selection. Existing audio is retained; new synthesis requires an available voice.',
      createVoice: 'Create a voice',
      words: 'Spoken script',
      start: 'Speech start (s)',
      end: 'Speech end (s)',
      split: 'Split script at cursor',
      remove: 'Delete script segment',
      audioRevision: 'Audio for script revision {{revision}}',
      previousAudioRevision: 'Current script revision {{revision}} · previous audio',
      sourceVolume: 'Source sound volume',
      voiceVolume: 'Narration volume',
      independent:
        'Script and captions are independent. Typing and volume changes do not generate speech.',
      segment: 'Script {{number}}',
      select: 'Select in timeline',
      add: 'Add script segment',
      limits:
        'Each segment requires nonempty text up to 500 characters; the script is limited to 2000 characters and 32 segments, with end after start.',
      pending: '{{count}} segments need regeneration',
      estimate: 'Estimate changed speech',
      conflict:
        'Speech does not fit its assigned window. Edit the script or review a retiming proposal.',
      reviewRetiming: 'Review retiming',
      reviseScript: 'Resolve by editing script',
      approval: 'Approve changed speech',
      close: 'Close',
      refreshQuote: 'Refresh estimate',
      approve: 'Generate for up to {{credits}} credits',
      quoted: 'Reuse completed speech and generate only {{count}} segments.',
      quoteChanged: 'The script or quote expiry changed. Refresh the estimate.',
      applyRetiming: 'Apply reviewed retiming',
      proposal:
        'Duration {{before}} → {{after}} s. Extend {{cuts}} footage cuts and anchor {{captions}} captions to their existing output intervals.',
      protected:
        'Caption wording, output timing and source rates stay fixed. Undo reverses the proposal in one step.',
      play: 'Play narration',
      playPrevious: 'Play previous script audio',
      stop: 'Stop playback',
      audioUnavailable: 'This audio is unavailable. Retry playback without regenerating speech.',
      states: {
        ready: 'Speech ready',
        stale: 'Regeneration needed',
        missing: 'No speech',
        conflict: 'Timing conflict',
      },
      retimingReasons: {
        speech: 'Generate missing or stale speech before proposing retiming.',
        duration:
          'Natural-speed speech exceeds the chosen duration or 60 seconds. Edit the script.',
        footage: 'There is insufficient eligible observed footage. Edit the script or add sources.',
        captions:
          'Retiming cannot preserve the existing caption and region timing. Edit the script.',
      },
    },
  },
} as const satisfies I18nFragment
