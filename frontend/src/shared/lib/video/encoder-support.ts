export interface EncoderSupport {
  video: boolean
  audio: boolean
  deviceMemoryGB?: number
}

interface EncoderProbeGlobals {
  video?: Pick<typeof VideoEncoder, 'isConfigSupported'>
  audio?: Pick<typeof AudioEncoder, 'isConfigSupported'>
  deviceMemoryGB?: number
}

/** Static capability calls only: no instances, encoding, source reads or network. */
export async function probeEncoderSupport(
  video: VideoEncoderConfig,
  audio?: AudioEncoderConfig,
  globals: EncoderProbeGlobals = {
    video: typeof VideoEncoder === 'undefined' ? undefined : VideoEncoder,
    audio: typeof AudioEncoder === 'undefined' ? undefined : AudioEncoder,
    deviceMemoryGB:
      typeof navigator === 'undefined'
        ? undefined
        : (navigator as Navigator & { deviceMemory?: number }).deviceMemory,
  },
): Promise<EncoderSupport> {
  const [videoSupport, audioSupport] = await Promise.allSettled([
    Promise.resolve().then(() => globals.video?.isConfigSupported(video)),
    Promise.resolve().then(() =>
      audio ? globals.audio?.isConfigSupported(audio) : { supported: true },
    ),
  ])
  return {
    video: videoSupport.status === 'fulfilled' && videoSupport.value?.supported === true,
    audio: audioSupport.status === 'fulfilled' && audioSupport.value?.supported === true,
    deviceMemoryGB: globals.deviceMemoryGB,
  }
}
