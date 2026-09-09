import { describe, expect, it } from 'vitest'
import { PublishStage } from '@/shared/api'
import { isBeforeCommitFence } from './types'

describe('isBeforeCommitFence', () => {
  // The trap this exists for: FILLING_SETTINGS was appended to the protobuf enum so that
  // adding it could not renumber the stages after it, so its NUMBER is higher than
  // COMMITTING while it belongs before the fence. Comparing numbers hid the cancel button
  // for a job that is still pre-commit and legally cancellable (PUB-13).
  it('does not read the order off the enum numbers', () => {
    expect(PublishStage.FILLING_SETTINGS > PublishStage.COMMITTING).toBe(true)
    expect(isBeforeCommitFence(PublishStage.FILLING_SETTINGS)).toBe(true)
  })

  it('is true for every stage before the fence', () => {
    for (const stage of [
      PublishStage.QUEUED,
      PublishStage.CLAIMED,
      PublishStage.PREPARING,
      PublishStage.OPENING_EDITOR,
      PublishStage.FILLING_CONTENT,
      PublishStage.UPLOADING_PHOTOS,
      PublishStage.FILLING_SETTINGS,
    ]) {
      expect(isBeforeCommitFence(stage)).toBe(true)
    }
  })

  it('is false from the fence onward, because cancelling then could duplicate a post', () => {
    for (const stage of [PublishStage.COMMITTING, PublishStage.VERIFYING, PublishStage.PUBLISHED]) {
      expect(isBeforeCommitFence(stage)).toBe(false)
    }
  })
})
