import i18next from 'i18next'
import { appFailureFromConnect } from '@/shared/api'
import { activeLocale, formatAppFailure } from '@/shared/lib'
import { TEMPLATE_ASK_MAX_PER_BODY, TEMPLATE_PHOTO_ROW_MAX } from '../config'
import { PARSE_REASONS } from '../lib/grammar'

/** Shared by all template mutations so stable reasons are translated consistently. A parse
 *  refusal says its reason in the builder's own words and names its area (TMPL-20, TMPL-42): the
 *  raw key would read as `malformed_tag`. */
export function templateErrorMessage(error: unknown): string {
  if (!error) return ''
  const failure = appFailureFromConnect(error)
  if (failure.reason !== 'TEMPLATE_PARSE_FAILED') return formatAppFailure(failure)
  const t = i18next.getFixedT(activeLocale(), 'templates')
  const area = failure.params.area === 'title_area' ? 'title_area' : 'body'
  const reason = PARSE_REASONS.find((known) => known === failure.params.reason)
  return formatAppFailure({
    ...failure,
    params: {
      ...failure.params,
      area: t(`source.area.${area}`),
      // A reason this build does not know is shown as the server named it.
      reason: reason
        ? t(`builder.reasons.${reason}`, {
            max: TEMPLATE_PHOTO_ROW_MAX,
            askMax: TEMPLATE_ASK_MAX_PER_BODY,
          })
        : failure.params.reason,
    },
  })
}
