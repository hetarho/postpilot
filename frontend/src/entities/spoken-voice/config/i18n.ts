export const spokenVoiceResources = {
  ko: {
    play: '{{name}} 듣기',
    stop: '재생 멈추기',
    duration: '{{seconds}}초',
    playFailed: '샘플을 재생하지 못했어요. 듣기를 다시 눌러 주세요.',
  },
  en: {
    play: 'Listen to {{name}}',
    stop: 'Stop playback',
    duration: '{{seconds}} seconds',
    playFailed: 'Could not play the sample. Press Listen again.',
  },
} as const
