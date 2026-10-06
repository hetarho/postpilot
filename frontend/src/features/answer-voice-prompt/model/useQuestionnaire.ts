import { useCallback } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import {
  questionnaireMachine,
  questionnaireStateOf,
  type QuestionnaireEvent,
} from './questionnaire-machine'

export function useQuestionnaire(ownerId: string, voiceId: string) {
  const actorRef = useActorRef(questionnaireMachine, { input: { ownerId, voiceId } })
  const state = useSelector(actorRef, questionnaireStateOf)
  const getSnapshot = useCallback(() => questionnaireStateOf(actorRef.getSnapshot()), [actorRef])
  const send = useCallback(
    (event: QuestionnaireEvent) => {
      if (actorRef.getSnapshot().status === 'active') actorRef.send(event)
      return questionnaireStateOf(actorRef.getSnapshot())
    },
    [actorRef],
  )
  return { state, send, getSnapshot, actorRef }
}
