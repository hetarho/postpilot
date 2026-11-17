import { useTranslation } from 'react-i18next'
import type { KitPart, KitStatus } from '../model/readiness'

/** The words under a part's name: what it holds now, from the account's own data. */
export function useKitStatusText() {
  const { t } = useTranslation('creation')
  return (part: KitPart): string => {
    const status: KitStatus = part.status
    switch (status.kind) {
      case 'loading':
        return ''
      case 'failed':
        return t('kit.status.failed')
      case 'none':
        return t('kit.status.none')
      case 'named':
        return part.key === 'voice'
          ? t('kit.status.voiceNamed', { name: status.name })
          : t('kit.status.modelNamed', { name: status.name })
      case 'count':
        return t('kit.status.count', { count: status.count })
      case 'learning':
        // Fully learned but not analysed yet: what is left is the analysis.
        return status.percent >= 100
          ? t('kit.status.analyze')
          : t('kit.status.learning', { percent: status.percent })
      case 'noDefault':
        return t('kit.status.noDefault')
      case 'draft':
        return t(part.key === 'spoken' ? 'kit.status.spokenDraft' : 'kit.status.templateDraft')
      case 'defaultsOnly':
        return t('kit.status.defaultsOnly', { count: status.count })
      case 'ownAndDefaults':
        return t('kit.status.own', { own: status.own })
      case 'unusable':
        return t('kit.status.unusable')
    }
  }
}
