import { create } from '@bufbuild/protobuf'
import { describe, expect, it } from 'vitest'
import { GetVoiceProfileResponseSchema, VoiceProfileSchema, VoiceSampleKind } from '@/shared/api'
import { createFakeAuthTransport, createTestQueryClient } from '@/test/session'
import { voiceMaterialFreshness, voiceMaterialVersionKey } from './types'
import { toVoiceProfile, voiceAnalysisQueryKey } from '../api/voice-queries'
import { withdrawCachedVoiceMaterial } from '../api/voice-directory-cache'
const wire = () =>
  create(VoiceProfileSchema, {
    voice: { id: 'voice', made: true },
    made: true,
    samples: [{ id: 'source', kind: VoiceSampleKind.POST, contentRevision: 1n }],
    analysis: {
      sourceVersionsKnown: true,
      acceptedSources: [{ sampleId: 'source', contentRevision: 1n }],
      ai: {
        impression: '이전 분석',
        examples: [{ field: 1, materialId: 'source', sentence: '삭제할 개인 문장' }],
      },
      counted: { marks: { example: { materialId: 'source', sentence: '삭제할 개인 문장' } } },
    },
  })
describe('accepted material freshness', () => {
  it('uses semantic revision identity, preserves label-only freshness, and reports legacy snapshots as unknown', () => {
    const profile = toVoiceProfile(wire())
    const key = voiceMaterialVersionKey(profile)
    expect(voiceMaterialFreshness(profile)).toBe('current')
    profile.samples[0]!.label = '새 제목'
    expect(voiceMaterialFreshness(profile)).toBe('current')
    expect(voiceMaterialVersionKey(profile)).toBe(key)
    profile.samples[0]!.contentRevision = 2n
    expect(voiceMaterialFreshness(profile)).toBe('pending')
    expect(voiceMaterialVersionKey(profile)).not.toBe(key)
    profile.analysis!.sourceVersionsKnown = false
    expect(voiceMaterialFreshness(profile)).toBe('unknown')
    expect(profile.analysis!.ai.impression).toBe('이전 분석')
  })
  it('retains changes made after an admitted snapshot as pending even after analysis completes', () => {
    const profile = toVoiceProfile(wire())
    profile.activeJobId = 'analysis'
    profile.samples[0]!.contentRevision = 2n
    expect(voiceMaterialFreshness(profile)).toBe('pending')
    profile.activeJobId = ''
    expect(voiceMaterialFreshness(profile)).toBe('pending')
    profile.analysis!.acceptedSources = [{ sampleId: 'source', contentRevision: 2n }]
    expect(voiceMaterialFreshness(profile)).toBe('current')
    // Restoring the earlier accepted snapshot does not claim the newer live source is current.
    profile.analysis!.acceptedSources = [{ sampleId: 'source', contentRevision: 1n }]
    expect(voiceMaterialFreshness(profile)).toBe('pending')
  })
  it('withdraws deleted personal examples immediately in the owned cache while preserving accepted analysis metadata', () => {
    const queries = createTestQueryClient(),
      transport = createFakeAuthTransport()
    const key = voiceAnalysisQueryKey(transport, 'alice', 'voice')
    const foreign = voiceAnalysisQueryKey(transport, 'bob', 'voice')
    const response = create(GetVoiceProfileResponseSchema, { profile: wire() })
    queries.setQueryData(key, response)
    queries.setQueryData(foreign, response)
    withdrawCachedVoiceMaterial(queries, transport, 'alice', 'voice', 'source')
    const cached = queries.getQueryData<typeof response>(key)!.profile!
    expect(cached.samples).toEqual([])
    expect(cached.analysis!.ai!.examples).toEqual([])
    expect(cached.analysis!.counted!.marks!.example).toBeUndefined()
    expect(cached.analysis!.acceptedSources).toEqual(response.profile!.analysis!.acceptedSources)
    expect(cached.analysis!.ai!.impression).toBe('이전 분석')
    expect(voiceMaterialFreshness(toVoiceProfile(cached))).toBe('pending')
    expect(queries.getQueryData(foreign)).toBe(response)
  })
})
