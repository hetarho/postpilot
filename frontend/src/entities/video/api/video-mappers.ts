import { create } from '@bufbuild/protobuf'
import { type Video, VideoSchema } from '@/shared/api'
import type { PostVideo } from '../model/types'

export function toPostVideo(video: Video): PostVideo {
  return {
    id: video.id,
    filename: video.filename,
    width: video.width,
    height: video.height,
    // Safe: the server caps a clip at 200 MiB, far inside Number's exact range.
    bytes: Number(video.bytes),
    durationMs: Number(video.durationMs),
    contentType: video.contentType,
    viewUrl: video.viewUrl,
  }
}

export function toProtoVideo(video: PostVideo): Video {
  return create(VideoSchema, {
    id: video.id,
    filename: video.filename,
    width: video.width,
    height: video.height,
    bytes: BigInt(video.bytes),
    durationMs: BigInt(video.durationMs),
    contentType: video.contentType,
    viewUrl: video.viewUrl,
  })
}
