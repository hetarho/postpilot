import type { PostDraft } from './types'

/** One attachment as a tile and the large view draw it — a photo or a clip, by filename. */
export interface ViewableAttachment {
  filename: string
  kind: 'photo' | 'video'
  /** A `blob:` URL right after an upload is fine; absent shows the filename. */
  viewUrl?: string
  width?: number
  height?: number
  /** A photo's clockwise turn (POST-107). */
  rotation?: number
  durationMs?: number
  contentType?: string
}

/** The post's attachments by filename, the one namespace photos and videos share (VIDEO-5),
 *  photos first and then clips — the order ①'s strip shows them in. */
export function postAttachments(
  post: Pick<PostDraft, 'images' | 'videos'>,
): ReadonlyMap<string, ViewableAttachment> {
  const byName = new Map<string, ViewableAttachment>()
  for (const image of post.images)
    byName.set(image.filename, {
      filename: image.filename,
      kind: 'photo',
      viewUrl: image.viewUrl || undefined,
      width: image.width,
      height: image.height,
      rotation: image.rotation,
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
