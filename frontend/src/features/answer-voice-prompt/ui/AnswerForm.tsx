import { VoiceAnswerComposer } from './VoiceAnswerComposer'

/** Explicit saved-answer editing keeps its existing form presentation. */
export function AnswerForm(props: Omit<Parameters<typeof VoiceAnswerComposer>[0], 'presentation'>) {
  return <VoiceAnswerComposer {...props} presentation="form" />
}
