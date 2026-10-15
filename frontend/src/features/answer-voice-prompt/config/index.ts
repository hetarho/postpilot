/** A photo prompt's photo goes through the post photo's browser conversion (VOICE-60, POST-33):
 *  decoded on the device (HEIC included), downscaled to this long edge and re-encoded as JPEG,
 *  which also drops the metadata a phone writes. */
export const VOICE_PHOTO_MAX_LONG_EDGE_PX = 1024
export const VOICE_PHOTO_JPEG_QUALITY = 0.85
