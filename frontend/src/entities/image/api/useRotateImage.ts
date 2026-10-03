import { useMutation } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'

/** Turns a photo by the owner's hand (POST-107); the answer is the photo as it now stands.
 *
 *  Only the call, as `useDeleteImage` is: which post's cache to patch is the post entity's
 *  knowledge, so the feature pairs this with `usePostImagesCache`. */
export function useRotateImage() {
  return useMutation(PostService.method.rotateImage)
}
