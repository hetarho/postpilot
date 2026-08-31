import i18next from 'i18next'
import { writeLocaleOverride, type Locale } from '@/shared/lib/localization'

export async function changeLocale(
  locale: Locale,
  storage?: Pick<Storage, 'setItem'>,
): Promise<void> {
  await i18next.changeLanguage(locale)
  writeLocaleOverride(locale, storage)
}
