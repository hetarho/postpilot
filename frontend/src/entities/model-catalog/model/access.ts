import i18next from 'i18next'
import type { CatalogModel, StageName, StageSelection } from './types'

/** A model's current refusal for one stage, kept separate from its credit estimate. */
export function modelChoiceIssue(model: CatalogModel, stage: StageName): string {
  const access = model.access?.[stage]
  if (model.disabled)
    return access ? i18next.t('access.providerUnavailable', { ns: 'models' }) : model.disabledReason
  if (model.access && !access) return i18next.t('unsuitable', { ns: 'models' })
  const unavailable = accessIssue(access?.unavailableReason, access?.requiredPlan)
  if (unavailable) return unavailable
  if (access && !access.entitled)
    return i18next.t('access.planRequired', {
      ns: 'models',
      plan: planLabel(access.requiredPlan),
    })
  if (model.priceUnavailable && access?.grade !== 'free')
    return i18next.t('access.priceUnavailable', { ns: 'models' })
  if (!model.affordable && access?.grade !== 'free')
    return i18next.t('selectField.unaffordable', {
      ns: 'models',
      credits: model.requiredCredits,
    })
  return ''
}

/** The server keeps saved refs even when current access refuses them. */
export function savedChoiceIssue(selection: StageSelection): string {
  return accessIssue(selection.unavailableReason, selection.requiredPlan)
}

function accessIssue(reason: string | undefined, requiredPlan: string | undefined): string {
  switch (reason) {
    case 'MODEL_PLAN_REQUIRED':
      return i18next.t('access.planRequired', {
        ns: 'models',
        plan: planLabel(requiredPlan ?? ''),
      })
    case 'MODEL_UNCLASSIFIED':
      return i18next.t('access.unclassified', { ns: 'models' })
    case 'MODEL_FREE_PATH_UNAVAILABLE':
      return i18next.t('access.freePathUnavailable', { ns: 'models' })
    case 'MODEL_PROVIDER_UNAVAILABLE':
      return i18next.t('access.providerUnavailable', { ns: 'models' })
  }
  return ''
}

export function planLabel(plan: string): string {
  return plan ? plan[0].toUpperCase() + plan.slice(1) : ''
}

export function freeProviderNote(model: CatalogModel, stage: StageName): string {
  return model.access?.[stage]?.grade === 'free'
    ? i18next.t('access.freeProviderLimit', { ns: 'models' })
    : ''
}
