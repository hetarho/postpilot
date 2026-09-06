// What the post entity may import from the video entity (FSD cross-import, `@x`): a post
// carries its clips, so the post's model needs the clip's shape and mappers.
export type { PostVideo } from '../model/types'
export { toPostVideo, toProtoVideo } from '../api/video-mappers'
