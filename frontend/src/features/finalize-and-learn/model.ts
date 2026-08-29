const PREFIX = 'postpilot:voice-learning:'

export interface LearningHandoff { eventId: string; jobId: string }
export function readLearningHandoff(ownerId: string, postSlug: string): LearningHandoff | undefined {
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
export function clearLearningHandoff(ownerId: string, postSlug: string) {
  localStorage.removeItem(`${PREFIX}${ownerId}:${postSlug}`)
}
export function discardLearningHandoffs() {
  for (let index = localStorage.length - 1; index >= 0; index -= 1) {
    const key = localStorage.key(index)
    if (key?.startsWith(PREFIX)) localStorage.removeItem(key)
  }
}
