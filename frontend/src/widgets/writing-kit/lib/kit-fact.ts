import type { TFunction } from 'i18next'
import type { KitEntry } from '../model/useWritingKit'

/** What an element holds, in one short line. */
export function kitFact(entry: KitEntry, t: TFunction<'creation'>): string {
  const { key, state, pending, failed } = entry.reading
  if (failed) return t('kit.fact.failed')
  if (pending) return t('kit.fact.loading')
  if (state === 'empty') return t('kit.fact.empty')
  const named = (name: string) =>
    entry.count > 1 ? t('kit.fact.more', { name, more: entry.count - 1 }) : name
  switch (key) {
    case 'voices':
      return state === 'ready'
        ? named(entry.sample ?? '')
        : entry.extra >= 100
          ? t('kit.fact.voiceAnalyze')
          : t('kit.fact.voiceMaking', { percent: entry.extra })
    case 'templates':
      return state === 'ready'
        ? named(entry.sample ?? '')
        : t('kit.fact.templateDrafts', { count: entry.extra })
    case 'guidelines':
      return state === 'ready'
        ? t('kit.fact.guidelines', { count: entry.count })
        : t('kit.fact.guidelineDefaults', { count: entry.extra })
    case 'memories':
      return t('kit.fact.memories', { count: entry.count })
  }
}
