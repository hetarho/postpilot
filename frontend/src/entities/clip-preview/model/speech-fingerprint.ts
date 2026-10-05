import type { ClipEditPlan } from '@/entities/clip-plan/@x/clip-preview'
/** Same UTF-8 length framing as clip.SpeechFingerprint; it covers actual audio and its output placement. */
export async function speechRenderFingerprint(plan: ClipEditPlan) {
  const narration = plan.narration
  if (!narration?.enabled || !narration.segments.length) return ''
  const encoder = new TextEncoder()
  let framed = `speech-render-v1|${narration.segments.length}|`
  const field = (value: string | number) => {
    const text = String(value)
    framed += `${encoder.encode(text).byteLength}:${text}`
  }
  for (const segment of narration.segments) {
    const speech = segment.speech
    if (!speech) throw new Error('CLIP_SPEECH_UNAVAILABLE')
    for (const value of [
      segment.id,
      segment.startMs,
      segment.endMs,
      narration.volumePermille,
      speech.assetId,
      speech.voiceId,
      speech.bindingDigest,
      speech.inputHash,
      speech.settingsHash,
      speech.audioHash,
      speech.profileId,
      speech.profileRevision,
      speech.samples,
      speech.sampleRate,
      speech.channels,
      speech.timing.length,
    ])
      field(value)
    for (const timing of speech.timing) {
      field(timing.text)
      field(timing.startMs)
      field(timing.endMs)
    }
  }
  const hash = await crypto.subtle.digest('SHA-256', encoder.encode(framed))
  return Array.from(new Uint8Array(hash), (value) => value.toString(16).padStart(2, '0')).join('')
}
