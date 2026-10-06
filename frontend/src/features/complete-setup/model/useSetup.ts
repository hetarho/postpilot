import { useCallback, useEffect } from 'react'
import { useActorRef, useSelector } from '@xstate/react'
import { useSetupAvailability } from './useSetupAvailability'
import {
  setupMachine,
  setupStateOf,
  setupProgressOf,
  type SetupEvent,
  type SetupForm,
  type SetupTarget,
} from './setup-machine'
import { writeSetupProgress } from './setup-progress'

export function useSetup(ownerId: string, restart: boolean) {
  const availability = useSetupAvailability(ownerId, restart, true)
  const actorRef = useActorRef(setupMachine, { input: { ownerId } })
  const state = useSelector(actorRef, setupStateOf)
  const getSnapshot = () => setupStateOf(actorRef.getSnapshot())
  const send = useCallback(
    (event: SetupEvent) => {
      if (actorRef.getSnapshot().status === 'active') actorRef.send(event)
      return setupStateOf(actorRef.getSnapshot())
    },
    [actorRef],
  )
  useEffect(() => {
    if (availability.status !== 'ready') return
    const missing: SetupForm[] = []
    if (availability.missingVoice) missing.push('voice')
    if (availability.missingPostTemplate) missing.push('post-template')
    if (availability.missingClipTemplate) missing.push('clip-template')
    send({ type: 'hydrate', ownerId, missing, progress: availability.progress })
  }, [
    availability.status,
    availability.missingVoice,
    availability.missingPostTemplate,
    availability.missingClipTemplate,
    availability.progress,
    ownerId,
    send,
  ])
  useEffect(() => {
    if (state.phase !== 'checking') writeSetupProgress(ownerId, setupProgressOf(state))
  }, [ownerId, state])
  const begin = (step: SetupForm) => {
    const before = getSnapshot()
    const next = send({ type: 'begin', ownerId, step })
    return next === before ? null : next.operation
  }
  return {
    state,
    availability,
    begin,
    next: (confirmed = true) =>
      send({ type: 'next', ownerId, step: getSnapshot().step, confirmed }),
    skip: () => {
      const step = getSnapshot().step
      if (step !== 'welcome' && step !== 'ready') send({ type: 'skip', ownerId, step })
    },
    back: () => send({ type: 'back', ownerId }),
    finish: (target: SetupTarget) => send({ type: 'finish', ownerId, target }),
    defer: (target: SetupTarget) => send({ type: 'defer', ownerId, target }),
    success: (step: SetupForm, operation: number, complete: boolean) =>
      send({ type: 'success', ownerId, step, operation, complete }),
    failure: (step: SetupForm, operation: number) =>
      send({ type: 'failure', ownerId, step, operation }),
    running: (step: SetupForm, operation: number) =>
      send({ type: 'running', ownerId, step, operation }),
  }
}
export type SetupController = ReturnType<typeof useSetup>
