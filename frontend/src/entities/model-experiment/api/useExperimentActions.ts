import type { CandidateBadges, VerdictBadgeName } from '../model/badges'
import { useRetiredExperimentMutation } from './useRetiredExperimentMutation'

export function useExperimentActions(id: string, onChanged?: () => Promise<unknown>) {
  // Keep the public contract while older editor consumers migrate. No new legacy action,
  // query invalidation, owner refresh or paid retry is admitted from this compatibility hook.
  void id
  void onChanged
  const { refuse, ...state } = useRetiredExperimentMutation()
  const complete: (
    ranks: Array<{
      candidateId: string
      rank: number
      badges: VerdictBadgeName[]
      otherNote: string
    }>,
    skip?: boolean,
  ) => Promise<never> = refuse
  const applyCandidate: (candidateId: string, adoptModel?: boolean) => Promise<never> = refuse
  const adoptCandidate: (candidateId: string) => Promise<never> = refuse
  const choose: (candidateId: string, badges?: CandidateBadges[]) => Promise<never> = refuse
  const decideWrite: (
    candidateId: string,
    adoptWinnerModel: boolean,
    badges?: CandidateBadges[],
  ) => Promise<never> = refuse
  const useSingle: (candidateId: string) => Promise<never> = refuse
  return {
    ...state,
    complete,
    applyCandidate,
    adoptCandidate,
    choose,
    decideWrite,
    useSingle,
    dismiss: refuse,
    retry: refuse,
    apply: refuse,
    adopt: refuse,
  }
}
