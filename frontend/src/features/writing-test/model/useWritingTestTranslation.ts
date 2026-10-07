import { useTranslation } from 'react-i18next'
import { writingTestI18n } from '../config/i18n'

type WritingTestTranslation = (
  key: string,
  options?: Readonly<Record<string, string | number | boolean | undefined>>,
) => string

/** The composition task registers this new namespace centrally. Keep its translator
 * local so this independently compilable slice does not infer every existing catalog
 * key from the application's current i18next resource augmentation. The official hook
 * subscribes to language changes; the runtime call still uses the writing-test namespace.
 */
export function useWritingTestTranslation(): { t: WritingTestTranslation } {
  const { i18n } = useTranslation('common')
  const translate = i18n.t as (key: string, options: Record<string, unknown>) => string
  return {
    t: (key, options) => translate.call(i18n, key, { ...options, ns: writingTestI18n.namespace }),
  }
}
