import { useMutation } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'

/** Removes a photo — the row and the object (spec/legacy/policy/uploads.md).
 *
 *  Only the call. Which post's cache to update is the post entity's knowledge, so the
 *  caller (the feature) pairs this with `usePostImagesCache` rather than this hook
 *  reaching across into another entity. */
export function useDeleteImage() {
  return useMutation(PostService.method.deleteImage)
}
