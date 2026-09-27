import { useEffect, useReducer } from 'react'
import { stepForStatus, type EditorStep } from './steps'

export interface DraftStepState {
  step: EditorStep
  /** The status this state last followed, so a re-render with the same status changes nothing. */
  followed: string
}
export type DraftStepEvent =
  { type: 'select'; step: EditorStep } | { type: 'status'; status: string; hasStoryline?: boolean }

/** The editor's step machine. Two transitions only:
 *
 *  - the owner picks a step, which stands until the post's lifecycle moves;
 *  - the post's STATUS changes, which carries the owner to the step that status belongs to —
 *    this is also the "generation finished" handoff, so the finished draft lands in 글 다듬기
 *    once rather than leaving the reader scrolled inside 글 생성.
 *
 *  The status IS the state (nothing new is persisted), so a reload, the list badge and this
 *  screen cannot disagree. */
export function draftStep(state: DraftStepState, event: DraftStepEvent): DraftStepState {
  if (event.type === 'select')
    return state.step === event.step ? state : { ...state, step: event.step }
  const followed = followedKey(event.status, event.hasStoryline)
  if (state.followed === followed) return state
  return { step: stepForStatus(event.status, event.hasStoryline), followed }
}

export function draftStepStart(status: string, hasStoryline = false): DraftStepState {
  return { step: stepForStatus(status, hasStoryline), followed: followedKey(status, hasStoryline) }
}

/** What the machine follows: the status, and whether a draft holds a storyline — a storyline job
 *  finishing is a move to 글 다듬기 without a status change (POST-44). */
function followedKey(status: string, hasStoryline = false): string {
  return hasStoryline ? `${status}+storyline` : status
}

/** The machine as the editor uses it. The status transition is applied AFTER the paint that
 *  reported the new status, not during it: the step that is on screen is what the reader was
 *  looking at when the server answered, and a panel that has something to say about the change
 *  (the publish refusal a landed edit causes) gets to say it before the step moves on. */
export function useDraftSteps(status: string, hasStoryline = false) {
  const [state, dispatch] = useReducer(draftStep, undefined, () =>
    draftStepStart(status, hasStoryline),
  )
  useEffect(() => {
    dispatch({ type: 'status', status, hasStoryline })
  }, [status, hasStoryline])
  return {
    step: state.step,
    select: (step: EditorStep) => dispatch({ type: 'select', step }),
  }
}
