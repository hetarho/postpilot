import type { PostDraft } from '@/entities/post'
import type { StorylineAttachment } from '../ui/StorylineParagraphEditor'

/** The post's attachments by filename, the one namespace photos and videos share (VIDEO-5), in
 *  the shape a storyline tile draws. */
export function storylineAttachments(
  post: Pick<PostDraft, 'images' | 'videos'>,
): ReadonlyMap<string, StorylineAttachment> {
  const byName = new Map<string, StorylineAttachment>()
  for (const image of post.images)
    byName.set(image.filename, {
      filename: image.filename,
      kind: 'photo',
      viewUrl: image.viewUrl || undefined,
      width: image.width,
      height: image.height,
    })
  for (const video of post.videos)
    byName.set(video.filename, {
      filename: video.filename,
      kind: 'video',
      viewUrl: video.viewUrl || undefined,
      durationMs: Number(video.durationMs),
      contentType: video.contentType,
    })
  return byName
}
