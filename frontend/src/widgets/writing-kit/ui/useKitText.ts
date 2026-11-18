import { useTranslation } from 'react-i18next'
import type { KitElement } from '../model/kit'

/** The words one kit piece is read with: its name, its state and its one fact. */
export function useKitText() {
  const { t } = useTranslation('creation')
  const fact = (element: KitElement): string => {
    const value = element.fact
    switch (value.kind) {
      case 'loading':
        return t('kit.facts.loading')
      case 'failed':
        return t('kit.facts.failed')
      case 'none':
        return t('kit.facts.none')
      case 'voice':
        return value.made
          ? t('kit.facts.voice', { name: value.name })
          : value.percent >= 100
            ? t('kit.facts.voiceAnalyse', { name: value.name })
            : t('kit.facts.voiceMaking', { name: value.name, percent: value.percent })
      case 'count':
        return value.lead
          ? value.count > 1
            ? t('kit.facts.countLead', { lead: value.lead, more: value.count - 1 })
            : value.lead
          : t('kit.facts.count', { count: value.count })
      case 'template':
        return value.name
          ? value.count > 1
            ? t('kit.facts.countLead', { lead: value.name, more: value.count - 1 })
            : value.name
          : t('kit.facts.noTemplate')
      case 'defaultsOnly':
        return t('kit.facts.defaultsOnly')
      case 'model':
        return value.label
      case 'unavailable':
        return t('kit.facts.unavailable')
    }
  }
  const name = (element: KitElement) => t(`kit.names.${element.key}`)
  const state = (element: KitElement) => t(`kit.state.${element.state}`)
  /** The state as a line of its own says it, capitalised where the language does. */
  const status = (element: KitElement) => t(`kit.status.${element.state}`)
  const label = (element: KitElement) =>
    t('kit.nodeLabel', { name: name(element), state: state(element), fact: fact(element) })
  const next = (element: KitElement) =>
    element.key === 'voice' && element.state === 'partial'
      ? element.fact.kind === 'voice' && element.fact.percent >= 100
        ? t('kit.next.voiceAnalyse')
        : t('kit.next.voicePartial')
      : t(`kit.next.${element.key}`)
  return { fact, name, state, status, label, next }
}
