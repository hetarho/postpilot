import { MediaRangeError } from './range-source'

export const NATIVE_SOURCE_COLOR_VERSION = 'native-source-color-v1'

/** Native's untagged YUV input uses limited-range BT.601. Explicit SDR tags
 * retain their declared matrix; primaries stay sRGB-compatible because native
 * sampling performs matrix conversion rather than a display-gamut transform. */
export function nativeSourceColorSpace(declared: VideoColorSpaceInit): VideoColorSpaceInit {
  if (
    (declared.primaries &&
      declared.primaries !== 'bt709' &&
      declared.primaries !== 'smpte170m' &&
      declared.primaries !== 'bt470bg') ||
    (declared.transfer &&
      declared.transfer !== 'bt709' &&
      declared.transfer !== 'smpte170m' &&
      declared.transfer !== 'iec61966-2-1') ||
    (declared.matrix &&
      declared.matrix !== 'bt709' &&
      declared.matrix !== 'smpte170m' &&
      declared.matrix !== 'bt470bg')
  )
    throw new MediaRangeError('CLIP_SOURCE_COLOR_UNSUPPORTED')
  return {
    matrix: declared.matrix ?? 'smpte170m',
    primaries: 'bt709',
    transfer: declared.transfer === 'iec61966-2-1' ? 'iec61966-2-1' : 'bt709',
    fullRange: declared.fullRange ?? false,
  }
}

/** The source instance owns its decoder configuration. No global codec hook or
 * library-private field is changed; the sink consumes this public track port. */
export function bindNativeSourceColor<
  T extends {
    getDecoderConfig(): Promise<VideoDecoderConfig | null>
  },
>(track: T, colorSpace: VideoColorSpaceInit): void {
  const get = track.getDecoderConfig.bind(track)
  track.getDecoderConfig = async () => {
    const config = await get()
    if (!config) return null
    return { ...config, colorSpace: { ...colorSpace } }
  }
}
