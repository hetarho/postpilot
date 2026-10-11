import { useMutation } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'

/** Puts a post's photos in the owner's order (POST-116): the full photo list, each once.
 *
 *  Only the call, as `useRotateImage` is: patching the cached post is the post entity's
 *  knowledge, so the feature pairs this with `usePostImagesCache`. */
export function useReorderImages() {
  return useMutation(PostService.method.reorderImages)
}
