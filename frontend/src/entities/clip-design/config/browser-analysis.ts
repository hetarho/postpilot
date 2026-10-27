import readiness from './browser-readiness.json'

/** Analysis qualification is independent of final composition rendering. */
export const CLIP_BROWSER_ANALYSIS_PREPARATION = {
  profile: readiness.analysisPreparation.profile,
  qualified:
    readiness.schemaVersion === 1 &&
    readiness.distributionApproved &&
    readiness.analysisPreparation.qualified &&
    readiness.analysisPreparation.enabled &&
    readiness.analysisPreparation.missingGates.length === 0,
} as const
