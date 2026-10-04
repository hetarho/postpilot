import { useMutation } from '@connectrpc/connect-query'
import { PostService } from '@/shared/api'
import type { PostImage } from '../model/types'
import { toPostImage } from './image-mappers'

/** Turns a photo by the owner's hand (POST-107); the answer is the photo as it now stands.
 *
 *  Only the call, as `useDeleteImage` is: which post's cache to patch is the post entity's
 *  knowledge, so the feature pairs this with `usePostImagesCache`. `onRotated` hears EVERY turn
 *  that lands: it sits on the mutation itself, where a per-call callback would fire for the
 *  latest call only, so turning a second photo while the first is in flight would drop the
 *  first one's answer. */
export function useRotateImage(onRotated?: (image: PostImage) => void) {
  return useMutation(PostService.method.rotateImage, {
    onSuccess: (response) => {
      if (response.image) onRotated?.(toPostImage(response.image))
    },
  })
}
