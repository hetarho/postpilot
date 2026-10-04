import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { SpokenDraftSchema, SpokenVoiceSchema } from '@/shared/api'
import { toSpokenDraft, toSpokenVoice } from './mappers'
import { spokenPlaybackUrl } from './hooks'

const profile = {
  id: 'explicit-profile',
  revision: 2n,
  providerId: 'speech',
  designModelId: 'design',
  speechModelId: 'tts',
  descriptionMax: 1000,
  previewMax: 1000,
  speechMax: 1000,
}
describe('private spoken library projections', () => {
  it('restores exact persisted phase, selected candidate and immutable binding', () => {
    const wire = create(SpokenDraftSchema, {
      id: 'draft',
      revision: 7n,
      name: 'Sound',
      profile,
      phase: 'selected',
      selectedCandidateId: 'second',
      candidates: [
        { id: 'second', assetId: 'private-asset', durationMs: 1243n, auditionedAt: 'played' },
      ],
    })
    const d = toSpokenDraft(wire)
    expect(d).toMatchObject({
      revision: 7n,
      phase: 'selected',
      selectedCandidateId: 'second',
      profile: { revision: 2n, speechModelId: 'tts' },
    })
    expect(d.candidates[0]).toEqual({
      id: 'second',
      assetId: 'private-asset',
      durationMs: 1243,
      auditionedAt: 'played',
    })
  })
  it('keeps a removed identity and sample reference while stripping unrelated wire fields', () => {
    const wire = create(SpokenVoiceSchema, {
      id: 'voice',
      revision: 3n,
      name: 'Renamed',
      profile,
      sampleAssetId: 'unchanged',
      sampleDurationMs: 3010n,
      removedAt: 'removed',
    })
    Object.assign(wire, { supplierHandle: 'secret', ownerId: 'foreign', objectKey: 'bucket-key' })
    const result = toSpokenVoice(wire)
    expect(result).toMatchObject({
      revision: 3n,
      sampleAssetId: 'unchanged',
      sampleDurationMs: 3010,
      removedAt: 'removed',
    })
    expect(result).not.toHaveProperty('supplierHandle')
    expect(result).not.toHaveProperty('ownerId')
    expect(result).not.toHaveProperty('objectKey')
  })
  it('refuses missing snapshots and unknown phases instead of inventing a default', () => {
    expect(() => toSpokenDraft(create(SpokenDraftSchema, { profile, phase: 'future' }))).toThrow()
    expect(() => toSpokenVoice(create(SpokenVoiceSchema))).toThrow()
    expect(() => toSpokenDraft(undefined)).toThrow()
  })
  it('accepts only short-lived application paths for credentialed playback', () => {
    expect(spokenPlaybackUrl(`/spoken/audio/${'a'.repeat(32)}`)).toContain(
      `/spoken/audio/${'a'.repeat(32)}`,
    )
    for (const path of [
      'https://supplier.example/secret',
      '//evil.example/audio',
      '/spoken/audio/../secret',
      '/spoken/audio/missing',
    ])
      expect(() => spokenPlaybackUrl(path)).toThrow()
  })
})
