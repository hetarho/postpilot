import { create } from '@bufbuild/protobuf'
import { type Image, ImageSchema } from '@/shared/api'
import type { PostImage } from '../model/types'

export function toPostImage(image: Image): PostImage {
  return {
    id: image.id,
    filename: image.filename,
    width: image.width,
    height: image.height,
    // Safe: the server caps an object at 10 MiB, far inside Number's exact range.
    bytes: Number(image.bytes),
    viewUrl: image.viewUrl,
  }
}

export function toProtoImage(image: PostImage): Image {
  return create(ImageSchema, {
    id: image.id,
    filename: image.filename,
    width: image.width,
    height: image.height,
    bytes: BigInt(image.bytes),
    viewUrl: image.viewUrl,
  })
}
