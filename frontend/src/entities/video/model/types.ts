/** A video attached to a post, as the app talks about it.
 *
 *  It is its own entity rather than a flag on `PostImage` because almost nothing about the two
 *  is the same on this side: a clip is never converted, it carries a duration and a container
 *  type, and every surface that shows one renders a `<video>` where a photo renders an `<img>`.
 */
export interface PostVideo {
  id: string
  /** The name the model and the exporters refer to this clip by; unique within a post ACROSS
   *  photos and videos — one filename namespace (VIDEO-5). */
  filename: string
  width: number
  height: number
  bytes: number
  /** Client-reported at confirm, from the container's own metadata. */
  durationMs: number
  /** The container's type, e.g. `video/mp4` — what the player needs. */
  contentType: string
  /** The short-lived presigned GET minted per `GetPost`, never persisted. For a clip this
   *  client just uploaded it is a local object URL of the ORIGINAL file until the next
   *  `GetPost` replaces it: the confirm answer carries no URL, and the bytes are already here. */
  viewUrl: string
}
