import { useQuery } from '@connectrpc/connect-query'
import { useTranslation } from 'react-i18next'
import { TemplateService, contentLanguageToProto } from '@/shared/api'
import { isLocale, type Locale } from '@/shared/lib/localization'

/** The 형식 안내 in the reader's UI language, as the backend states it (TMPL-41). The backend
 *  owns the text so the guide an owner copies and the one a template request teaches are one
 *  text; it changes only with a deploy, so a language's guide is read once and kept.
 *
 *  It is read when the source mounts, never inside the copy click: the clipboard write has to be
 *  the first thing the click awaits, or the browser drops the user activation that allows it. */
export function useFormatGuide(): {
  guide: string
  isPending: boolean
  isError: boolean
  refetch: () => void
} {
  const { i18n } = useTranslation()
  const locale: Locale = isLocale(i18n.resolvedLanguage) ? i18n.resolvedLanguage : 'ko'
  const query = useQuery(
    TemplateService.method.getFormatGuide,
    { language: contentLanguageToProto(locale) },
    { staleTime: Infinity },
  )
  return {
    guide: query.data?.text ?? '',
    isPending: query.isPending,
    isError: query.isError,
    refetch: () => void query.refetch(),
  }
}
