import type {
  ProtoSpeechProfileChoice,
  ProtoAdminSpeechProfile,
  ProtoSpeechAdminBrowse,
  ProtoSpeechAccountTariff,
} from '@/shared/api'
import type {
  AdminSpeechProfile,
  SpeechProfileChoice,
  SpeechAdminBrowse,
  SpeechAccountTariff,
} from '../model/speech'
import { isLevelName } from '../model/level'

export function toSpeechChoice(p: ProtoSpeechProfileChoice): SpeechProfileChoice {
  return {
    id: p.id,
    revision: p.revision,
    label: p.label,
    designModel: {
      providerId: p.designModel?.providerId ?? '',
      modelId: p.designModel?.modelId ?? '',
    },
    speechModel: {
      providerId: p.speechModel?.providerId ?? '',
      modelId: p.speechModel?.modelId ?? '',
    },
    designLabel: p.designLabel,
    speechLabel: p.speechLabel,
    grade: isLevelName(p.grade) ? p.grade : '',
    requiredPlan: p.requiredPlan,
    entitled: p.entitled,
    available: p.available,
    unavailableReason: p.unavailableReason,
    descriptionMax: p.descriptionMax,
    previewMin: p.previewMin,
    previewMax: p.previewMax,
    speechMax: p.speechMax,
    voiceReady: p.voiceReady,
    exportReady: p.exportReady,
  }
}

export function toAdminSpeechProfile(p: ProtoAdminSpeechProfile): AdminSpeechProfile {
  const b = p.binding
  const settings = b?.settings
  return {
    id: p.id,
    revision: p.revision,
    label: p.label,
    grade: isLevelName(p.grade) ? p.grade : '',
    enabled: p.enabled,
    voiceReady: p.voiceReady,
    exportReady: p.exportReady,
    binding: {
      designModel: {
        providerId: b?.designModel?.providerId ?? '',
        modelId: b?.designModel?.modelId ?? '',
      },
      speechModel: {
        providerId: b?.speechModel?.providerId ?? '',
        modelId: b?.speechModel?.modelId ?? '',
      },
      settings: {
        stability: settings?.stability ?? 0,
        similarityBoost: settings?.similarityBoost ?? 0,
        style: settings?.style ?? 0,
        speakerBoost: settings?.speakerBoost ?? false,
        speed: settings?.speed ?? 0,
      },
      outputFormat: b?.outputFormat ?? '',
      descriptionMax: b?.descriptionMax ?? 0,
      previewMax: b?.previewMax ?? 0,
      speechMax: b?.speechMax ?? 0,
    },
    prices: p.prices.map((price) => ({
      operation: price.operation,
      source: price.source,
      boundsSource: price.boundsSource,
      checkedAt: price.checkedAt,
      complete: price.complete,
      charges: price.charges.map((c) => ({
        unit: c.unit,
        usdPerUnit: c.usdPerUnit,
        multiplier: c.multiplier,
        maximumUnits: c.maximumUnits,
        unitsPerInputCharacter: c.unitsPerInputCharacter,
      })),
    })),
  }
}

export function toSpeechAdminBrowse(data: ProtoSpeechAdminBrowse): SpeechAdminBrowse {
  return {
    profiles: data.profiles.map(toAdminSpeechProfile),
    choices: data.choices.map(toSpeechChoice),
    fetchError: data.fetchError,
    combinations: data.combinations.map(toAdminSpeechProfile),
    tariff: data.tariff ? toSpeechAccountTariff(data.tariff) : null,
    candidates: data.candidates.map((m) => ({
      ref: { providerId: m.ref?.providerId ?? '', modelId: m.ref?.modelId ?? '' },
      label: m.label,
      design: m.design,
      synthesis: m.synthesis,
      korean: m.korean,
      style: m.style,
      speakerBoost: m.speakerBoost,
      requiresAlpha: m.requiresAlpha,
      maxText: m.maxText,
      tokenCostFactor: m.tokenCostFactor,
      characterCostMultiplier: m.characterCostMultiplier,
      costDiscountMultiplier: m.costDiscountMultiplier,
    })),
  }
}

export function toSpeechAccountTariff(t: ProtoSpeechAccountTariff): SpeechAccountTariff {
  return {
    revision: t.revision,
    designUsdPerUnit: t.designUsdPerUnit,
    speechUsdPerUnit: t.speechUsdPerUnit,
    confirmationUsd: t.confirmationUsd,
    source: t.source,
    complete: t.complete,
    checkedAt: t.checkedAt,
  }
}
