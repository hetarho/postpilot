import type { PcmChannels } from '@/shared/lib'
import { ANALYSIS_PREPARATION_LIMITS as limits } from '../config/limits'

/** Match native float stereo→mono rematrixing. Canonical source decoding uses
 * sqrt(1/2) in both planes for an original mono stream, so this also restores
 * its unchanged level. This applies no owner volume, fade or normalization. */
export function analysisMono(channels: PcmChannels): Float32Array<ArrayBuffer> {
  if (
    channels.length !== 2 ||
    channels[0].length !== channels[1].length ||
    channels[0].length < 1 ||
    channels[0].length > (limits.intervalMs * limits.audioRate) / 1000
  )
    throw new Error('CLIP_SOURCE_AUDIO_MEMORY_LIMIT')
  const mono = new Float32Array(channels[0].length)
  for (let n = 0; n < mono.length; n++) {
    const value = (channels[0][n] + channels[1][n]) * Math.SQRT1_2
    if (!Number.isFinite(value)) throw new Error('CLIP_SOURCE_AUDIO_INVALID')
    mono[n] = value
  }
  return mono
}
