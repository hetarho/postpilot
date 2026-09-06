import { useMutation } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'

/** Removes a video — the row, the object, and the observation its filename owned (VIDEO-12).
 *
 *  Only the call, for the same reason `useDeleteImage` is: which post's cache to update is the
 *  post entity's knowledge, so the caller pairs this with the post cache rather than this hook
 *  reaching across into another entity. */
export function useDeleteVideo() {
  return useMutation(PostService.method.deleteVideo)
}
