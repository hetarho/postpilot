import { MediaRangeError } from './range-source'

export const NATIVE_SOURCE_COLOR_VERSION = 'native-source-color-v1'

// The DOM typings predate the BT.2020 and HLG names browsers report, so the
// tags are compared as strings.
const PRIMARIES = new Set(['bt709', 'smpte170m', 'bt470bg', 'bt2020'])
const TRANSFERS = new Set(['bt709', 'smpte170m', 'iec61966-2-1', 'hlg'])
const MATRICES = new Set(['bt709', 'smpte170m', 'bt470bg', 'bt2020-ncl'])

/** Native's untagged YUV input uses limited-range BT.601. Explicit tags retain
 * their declared matrix; primaries stay sRGB-compatible because native
 * sampling performs matrix conversion rather than a display-gamut transform.
 * An HLG source (BT.2020) takes the same path: native decodes it with its
 * BT.2020 matrix and no tone mapping, so its signal is used as is (CLIP-219).
 * PQ has no SDR-compatible signal and stays refused. */
export function nativeSourceColorSpace(declared: VideoColorSpaceInit): VideoColorSpaceInit {
  const tags = declared as {
    primaries?: string | null
    transfer?: string | null
    matrix?: string | null
  }
  if (
    (tags.primaries && !PRIMARIES.has(tags.primaries)) ||
    (tags.transfer && !TRANSFERS.has(tags.transfer)) ||
    (tags.matrix && !MATRICES.has(tags.matrix))
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
