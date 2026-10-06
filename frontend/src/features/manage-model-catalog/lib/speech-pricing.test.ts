import { describe, expect, it } from 'vitest'
import type { SpeechOperationPrice } from '@/entities/model-catalog'
import {
  friendlySpeechModel,
  publicSpeechPrice,
  scaleSpeechDecimal,
  speechOperationCost,
  speechTariffDraft,
} from './speech-pricing'

describe('speech pricing references and exact editing', () => {
  it('uses friendly known names and preserves supplier names for other models', () => {
    expect(friendlySpeechModel('eleven_multilingual_ttv_v2')).toBe('Voice Design v2')
    expect(friendlySpeechModel('eleven_v4', 'eleven_v4')).toBe('Eleven v4')
    expect(friendlySpeechModel('new-model', 'New supplier model')).toBe('New supplier model')
    expect(friendlySpeechModel('new-model', 'new-model')).toBeUndefined()
  })
  it('ends promotional estimates at the declared boundary without inventing unknown prices', () => {
    expect(publicSpeechPrice('eleven_v4', new Date('2026-10-06T12:00:00+09:00'))).toMatchObject({
      usd: '0.022',
      standardUsd: '0.08',
      promotionActive: true,
    })
    expect(publicSpeechPrice('eleven_v4_turbo', new Date('2026-10-12T23:59:59+09:00'))?.usd).toBe(
      '0.011',
    )
    expect(publicSpeechPrice('eleven_v4', new Date('2026-10-13T00:00:00+09:00'))).toMatchObject({
      usd: '0.08',
      promotionActive: false,
    })
    expect(publicSpeechPrice('unknown')).toBeNull()
  })
  it.each(['0', '0.000000001', '0.000100001', '12.123456789'])(
    'round-trips a stored %s price through 1,000-unit editing exactly',
    (value) => {
      expect(scaleSpeechDecimal(scaleSpeechDecimal(value, 3), -3)).toBe(value)
    },
  )
  it('keeps invalid draft input invalid rather than manufacturing a free price', () => {
    expect(scaleSpeechDecimal('NaN', -3)).toBe('NaN')
    expect(scaleSpeechDecimal('', -3)).toBe('')
  })
  it('prefills missing values but preserves explicit zero, account prices and verification dates without acknowledging them', () => {
    const draft = speechTariffDraft({
      revision: 8n,
      designUsdPerUnit: '0',
      speechUsdPerUnit: '0.000000001',
      confirmationUsd: '',
      source: 'https://example.com/account',
      complete: true,
      checkedAt: '2026-10-05T00:00:00Z',
    })
    expect(draft).toMatchObject({
      revision: 8n,
      designUsdPerUnit: '0',
      speechUsdPerUnit: '0.000000001',
      confirmationUsd: '0',
      source: 'https://example.com/account',
      complete: false,
      checkedAt: '2026-10-05T00:00:00Z',
    })
    expect(speechTariffDraft(null)).toMatchObject({
      designUsdPerUnit: '0.00008',
      speechUsdPerUnit: '0.00008',
      complete: false,
      checkedAt: '',
    })
  })
  it('uses complete account prices and unit conversions without a float round-trip', () => {
    const price: SpeechOperationPrice = {
      operation: 'speech',
      complete: true,
      source: '',
      boundsSource: '',
      checkedAt: '',
      charges: [
        {
          unit: 'character_cost',
          usdPerUnit: '0.000000001',
          multiplier: '1.25',
          unitsPerInputCharacter: '0.5',
          maximumUnits: '1000',
        },
      ],
    }
    expect(speechOperationCost(price, 1000)).toBe('0.000000625')
    expect(speechOperationCost({ ...price, complete: false }, 1000)).toBeNull()
    expect(
      speechOperationCost(
        {
          ...price,
          charges: [{ ...price.charges[0], usdPerUnit: '0', unitsPerInputCharacter: '' }],
        },
        1000,
      ),
    ).toBe('0')
    expect(
      speechOperationCost(
        { ...price, charges: [{ ...price.charges[0], unitsPerInputCharacter: '' }] },
        1000,
      ),
    ).toBeNull()
  })
})
