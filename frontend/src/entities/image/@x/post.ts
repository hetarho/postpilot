// What the post entity may import from the image entity (FSD cross-import, `@x`): a post
// carries its photos, so the post's model needs the photo's shape and mapper.
export type { PostImage } from '../model/types'
export { toPostImage, toProtoImage } from '../api/image-mappers'
