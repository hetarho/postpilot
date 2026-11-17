import type { WritingTest } from '@/entities/writing-test'

/** A contestant's blind letter, stable for the test's frozen candidate order. */
export const writingTestCandidateLabel = (test: WritingTest, candidateId: string) =>
  String.fromCharCode(65 + test.candidates.findIndex((entry) => entry.id === candidateId))
