import { useCallback, useEffect, useRef, useState } from 'react'
import { useSetupAvailability } from './useSetupAvailability'
import {
  initialSetupState,
  setupTransition,
  setupProgressOf,
  type SetupEvent,
  type SetupForm,
  type SetupTarget,
} from './setup-machine'
import { writeSetupProgress } from './setup-progress'

export function useSetup(ownerId: string, restart: boolean) {
  const availability = useSetupAvailability(ownerId, restart, true)
  const [state, setState] = useState(() => initialSetupState(ownerId))
  const current = useRef(state)
  const send = useCallback((event: SetupEvent) => {
    const next = setupTransition(current.current, event)
    current.current = next
    setState(next)
    return next
  }, [])
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
    const before = current.current
    const next = send({ type: 'begin', ownerId, step })
    return next === before ? null : next.operation
  }
  return {
    state,
    availability,
    begin,
    next: (confirmed = true) =>
      send({ type: 'next', ownerId, step: current.current.step, confirmed }),
    skip: () => {
      const step = current.current.step
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
