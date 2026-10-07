import { describe, expect, it } from 'vitest'
import type { RequestInspectionView } from '@/entities/request-inspection'
import { issuedAtText, requestDocument } from './request-document'

describe('safe technical request copying', () => {
  it('copies only explicit safe fields and keeps reference estimates separate from unknown provider usage', () => {
    const view: RequestInspectionView & {
      canonicalPost: string
      supplierCost: string
      sdkPayload: string
    } = {
      version: 1,
      status: 'prepared',
      stage: 'write',
      mode: 'post-writing',
      promptVersion: 'prompt-v1',
      schemaVersion: 'schema-v1',
      fragments: [
        {
          id: 'source',
          role: 'user',
          authorship: 'account',
          materialRole: 'owner facts',
          text: 'My own material.',
          sourceRefs: ['memo'],
        },
      ],
      selectedRuleIds: ['facts-only'],
      conditions: { maxCompletionTokens: 1000n, reasoningEffort: 'low' },
      measures: { characters: 18n, utf8Bytes: 20n, referenceTokenEstimate: 5n },
      canonicalPost: 'PRIVATE_CANONICAL_POST',
      supplierCost: 'PRIVATE_SUPPLIER_COST',
      sdkPayload: 'PRIVATE_SDK_PAYLOAD',
    }
    const document = requestDocument([view])
    expect(document).toContain('Reference token estimate: 5')
    expect(document).toContain('Actual provider input tokens: Unknown')
    expect(document).toContain('Material owner: account')
    expect(document).toContain('My own material.')
    expect(document).not.toMatch(/PRIVATE_|Issued at:|Call identity:/)
  })

  it('does not copy payload fields or a speculative call identity from an unavailable capture', () => {
    const view: RequestInspectionView = {
      version: 1,
      status: 'unavailable',
      stage: 'write',
      mode: 'PRIVATE_MODE',
      unavailableReason: 'PRIVATE_REASON',
      callId: 'PRIVATE_CALL',
      fragments: [
        {
          id: 'PRIVATE_ID',
          role: 'system',
          authorship: 'account',
          materialRole: 'private',
          text: 'PRIVATE_TEXT',
          sourceRefs: ['PRIVATE_SOURCE'],
        },
      ],
      selectedRuleIds: ['PRIVATE_RULE'],
      conditions: { model: { providerId: 'PRIVATE_PROVIDER', modelId: 'PRIVATE_MODEL' } },
    }
    expect(requestDocument([view])).toBe(
      'Stage: write\nState: unavailable\nRequest information unavailable',
    )
  })

  it('preserves actual captured call order and immutable safe output grammar', () => {
    const base: RequestInspectionView = {
      version: 1,
      status: 'captured',
      stage: 'write',
      mode: 'post-writing',
      fragments: [],
      selectedRuleIds: [],
      output: { name: 'post', version: 'v1', schema: '{"safe":"schema"}' },
    }
    const first = { ...base, callId: 'call-1', issuedAt: { seconds: 1n, nanos: 0 } }
    const second = { ...base, callId: 'call-2', issuedAt: { seconds: 2n, nanos: 0 } }
    const document = requestDocument([first, second])
    expect(document.indexOf('call-1')).toBeLessThan(document.indexOf('call-2'))
    expect(document).toContain('Issued at: 1970-01-01T00:00:01.000Z')
    expect(document).toContain('{"safe":"schema"}')
    expect(first.output?.schema).toBe('{"safe":"schema"}')
    expect(issuedAtText({ seconds: 10n ** 30n, nanos: 0 })).toBeUndefined()
  })
})
