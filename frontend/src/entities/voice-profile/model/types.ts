export interface VoiceSample {
  id: string
  label: string
  chars: number
  createdAt: string
}

export interface VoiceProfile {
  styleguide: string
  rules: string
  updatedAt: string
  samples: VoiceSample[]
  activeJobId: string
}

/** Rules alone are user-authored guidance, not evidence that a voice was learned. */
export function isEmptyProfile(profile: Pick<VoiceProfile, 'styleguide' | 'samples'>): boolean {
  return profile.styleguide.trim() === '' && profile.samples.length === 0
}
