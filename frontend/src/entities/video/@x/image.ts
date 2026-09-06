// What the image entity may import from the video entity (FSD cross-import, `@x`): one upload
// handshake serves both kinds, so the confirm's answer has to be able to be a clip.
export type { PostVideo } from '../model/types'
export { toPostVideo } from '../api/video-mappers'
