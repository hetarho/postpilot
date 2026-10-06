import type { SpeechAccountTariff, SpeechOperationPrice } from '@/entities/model-catalog'
import {
  SPEECH_ACCOUNT_TARIFF_DEFAULTS,
  SPEECH_MODEL_PRESENTATION,
  SPEECH_PUBLIC_PRICING,
} from '../config/speech'

const DECIMAL = /^\d+(?:\.\d*)?$/

function decimalParts(value: string): { digits: bigint; scale: number } | null {
  if (!DECIMAL.test(value)) return null
  const [whole, fraction = ''] = value.split('.')
  return { digits: BigInt(whole + fraction), scale: fraction.length }
}
function decimalString(digits: bigint, scale: number): string {
  const raw = digits.toString().padStart(scale + 1, '0')
  if (scale === 0) return raw
  const fraction = raw.slice(-scale).replace(/0+$/, '')
  return `${raw.slice(0, -scale)}${fraction ? `.${fraction}` : ''}`
}
/** Input and stored prices never take a floating-point round-trip. */
export function scaleSpeechDecimal(value: string, places: number): string {
  const parsed = decimalParts(value)
  if (!parsed) return value
  const scale = parsed.scale - places
  return scale >= 0
    ? decimalString(parsed.digits, scale)
    : decimalString(parsed.digits * 10n ** BigInt(-scale), 0)
}
export function speechTariffDraft(tariff: SpeechAccountTariff | null): SpeechAccountTariff {
  return {
    ...SPEECH_ACCOUNT_TARIFF_DEFAULTS,
    ...tariff,
    designUsdPerUnit: tariff?.designUsdPerUnit || SPEECH_ACCOUNT_TARIFF_DEFAULTS.designUsdPerUnit,
    speechUsdPerUnit: tariff?.speechUsdPerUnit || SPEECH_ACCOUNT_TARIFF_DEFAULTS.speechUsdPerUnit,
    confirmationUsd: tariff?.confirmationUsd || SPEECH_ACCOUNT_TARIFF_DEFAULTS.confirmationUsd,
    source: tariff?.source || SPEECH_ACCOUNT_TARIFF_DEFAULTS.source,
    complete: false,
  }
}
export function friendlySpeechModel(modelId: string, supplierLabel?: string): string | undefined {
  return (
    SPEECH_MODEL_PRESENTATION[modelId]?.name ??
    (supplierLabel && supplierLabel !== modelId ? supplierLabel : undefined)
  )
}
export function publicSpeechPrice(modelId: string, now = new Date()) {
  const model = SPEECH_MODEL_PRESENTATION[modelId]
  if (!model?.usdPer1000) return null
  const promotionActive =
    Boolean(model.promotionalUsd) &&
    now.getTime() < Date.parse(SPEECH_PUBLIC_PRICING.promotionExpiresAt)
  return {
    usd: promotionActive ? model.promotionalUsd! : model.usdPer1000,
    standardUsd: model.usdPer1000,
    promotionActive,
  }
}
/** A complete saved tariff can be shown only when every quantity is convertible. */
export function speechOperationCost(
  price: SpeechOperationPrice | undefined,
  characters: number,
): string | null {
  if (!price?.complete || price.charges.length === 0) return null
  const terms: { digits: bigint; scale: number }[] = []
  for (const charge of price.charges) {
    const unitPrice = decimalParts(charge.usdPerUnit)
    const multiplier = decimalParts(charge.multiplier)
    if (!unitPrice || !multiplier || multiplier.digits === 0n) return null
    if (
      unitPrice.digits === 0n &&
      ['characters', 'supplier_credits', 'character_cost', 'requests', 'seconds'].includes(
        charge.unit,
      )
    ) {
      terms.push({ digits: 0n, scale: 0 })
      continue
    }
    let quantity: string
    if (charge.unit === 'requests') quantity = '1'
    else if (
      ['characters', 'supplier_credits', 'character_cost'].includes(charge.unit) &&
      charge.unitsPerInputCharacter
    ) {
      quantity = charge.unitsPerInputCharacter
    } else return null
    const factors = [
      charge.usdPerUnit,
      charge.multiplier,
      quantity,
      charge.unit === 'requests' ? '1' : String(characters),
    ].map(decimalParts)
    if (factors.some((factor) => factor === null)) return null
    terms.push(
      factors.reduce<{ digits: bigint; scale: number }>(
        (sum, factor) => ({
          digits: sum.digits * factor!.digits,
          scale: sum.scale + factor!.scale,
        }),
        { digits: 1n, scale: 0 },
      ),
    )
  }
  const scale = Math.max(...terms.map((term) => term.scale))
  return decimalString(
    terms.reduce((sum, term) => sum + term.digits * 10n ** BigInt(scale - term.scale), 0n),
    scale,
  )
}
