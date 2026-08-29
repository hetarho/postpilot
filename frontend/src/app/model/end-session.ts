import { discardDraftQueues } from '@/features/save-draft'
import { discardContentQueues } from '@/features/edit-post-content'
import { discardLearningHandoffs } from '@/features/finalize-post'
import { discardUploadBatches } from '@/features/upload-photos'

/** Everything that must not outlive the session, in one place.
 *
 *  A draft queue or an upload batch outlives its editor on purpose, but never its
 *  session: a retry or a confirm landing after someone else has signed in on this device
 *  would file the previous account's text or photo under the new account's cookie. Both
 *  ways out of a session — logout, and a session that died mid-use — call this, so a new
 *  slice with work that outlives its screen has one place to register. */
export function endSession(): void {
  discardDraftQueues()
  discardContentQueues()
  discardLearningHandoffs()
  discardUploadBatches()
}
