import type { Transport } from '@connectrpc/connect'
import { type ProtoVoiceProfile, type ProtoVoiceSample } from '@/shared/api'
import type { VoiceProfile, VoiceSample } from '../model/types'

export function toVoiceSample(sample: ProtoVoiceSample): VoiceSample {
  return {
    id: sample.id,
    label: sample.label,
    chars: sample.chars,
    createdAt: sample.createdAt,
  }
}

export function toVoiceProfile(profile: ProtoVoiceProfile | undefined): VoiceProfile {
  return {
    styleguide: profile?.styleguide ?? '',
    rules: profile?.rules ?? '',
    updatedAt: profile?.updatedAt ?? '',
    samples: profile?.samples.map(toVoiceSample) ?? [],
    activeJobId: profile?.activeJobId ?? '',
  }
}

export function voiceProfileQueryKey(transport: Transport, ownerId: string) {
  // The RPC request has no user id because the server scopes it from the cookie. The
  // client key still needs the session identity: otherwise a fresh Alice entry can be
  // rendered immediately after Bob signs in on the same QueryClient.
  return ['voice-profile', transport, ownerId] as const
}
