const PREFIX = 'postpilot:voice-learning:'

/** What a started learning run leaves behind for whichever surface renders next: the run outlives
 *  the step change that started it, and a completed one outlives the reload after it — that is how
 *  "이 revision은 이미 학습했어요" survives without a server field to read it from. Only the
 *  revision guard expires an entry; logout discards them all. */
export interface LearningHandoff {
  eventId: string
  jobId: string
  /** Decimal bigint. Keeps a completed/failed handoff from shadowing a later edit. */
  contentRevision?: string
}
export function isLearningHandoffForRevision(
  handoff: LearningHandoff | undefined,
  revision: bigint,
) {
  if (!handoff) return false
  if (handoff.contentRevision === undefined) return true
  try {
    return BigInt(handoff.contentRevision) === revision
  } catch {
    return false
  }
}
export function readLearningHandoff(
  ownerId: string,
  postSlug: string,
): LearningHandoff | undefined {
  try {
    const value = localStorage.getItem(`${PREFIX}${ownerId}:${postSlug}`)
    return value ? (JSON.parse(value) as LearningHandoff) : undefined
  } catch {
    return undefined
  }
}
export function writeLearningHandoff(ownerId: string, postSlug: string, value: LearningHandoff) {
  localStorage.setItem(`${PREFIX}${ownerId}:${postSlug}`, JSON.stringify(value))
}
export function discardLearningHandoffs() {
  for (let index = localStorage.length - 1; index >= 0; index -= 1) {
    const key = localStorage.key(index)
    if (key?.startsWith(PREFIX)) localStorage.removeItem(key)
  }
}
