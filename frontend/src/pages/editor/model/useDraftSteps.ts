import { useEffect, useReducer } from 'react'
import { stepForPost, type EditorStep, type PostStepFacts } from './steps'

export interface DraftStepState {
  step: EditorStep
  /** The facts this state last followed, so a re-render with the same facts changes nothing. */
  followed: string
}
export type DraftStepEvent =
  { type: 'select'; step: EditorStep } | ({ type: 'facts' } & PostStepFacts)

/** The editor's step machine. Two transitions only:
 *
 *  - the owner picks a step, which stands until the post's lifecycle moves;
 *  - the post's persisted facts change — its status, whether it holds observations, whether it
 *    holds a storyline — which carries the owner to the step those facts belong to. This is also
 *    the "run finished" handoff: an observation run lands on ② 관찰, a storyline run on
 *    ③ 스토리라인 and a writing run on ④ 다듬기, once, rather than leaving the reader where the run
 *    started.
 *
 *  Nothing new is persisted, so a reload, the list badge and this screen cannot disagree. */
export function draftStep(state: DraftStepState, event: DraftStepEvent): DraftStepState {
  if (event.type === 'select')
    return state.step === event.step ? state : { ...state, step: event.step }
  const followed = followedKey(event)
  if (state.followed === followed) return state
  return { step: stepForPost(event), followed }
}

export function draftStepStart(facts: PostStepFacts): DraftStepState {
  return { step: stepForPost(facts), followed: followedKey(facts) }
}

/** What the machine follows: the status, and whether a draft holds observations and a storyline —
 *  an observation or a storyline job finishing is a move without a status change (POST-44). */
function followedKey({ status, hasObservations, hasStoryline }: PostStepFacts): string {
  return `${status}${hasObservations ? '+observations' : ''}${hasStoryline ? '+storyline' : ''}`
}

/** The machine as the editor uses it. The facts transition is applied AFTER the paint that
 *  reported the new facts, not during it: the step that is on screen is what the reader was
 *  looking at when the server answered, and a panel that has something to say about the change
 *  (the publish refusal a landed edit causes) gets to say it before the step moves on. */
export function useDraftSteps({ status, hasObservations, hasStoryline }: PostStepFacts) {
  const [state, dispatch] = useReducer(draftStep, undefined, () =>
    draftStepStart({ status, hasObservations, hasStoryline }),
  )
  useEffect(() => {
    dispatch({ type: 'facts', status, hasObservations, hasStoryline })
  }, [status, hasObservations, hasStoryline])
  return {
    step: state.step,
    select: (step: EditorStep) => dispatch({ type: 'select', step }),
  }
}
