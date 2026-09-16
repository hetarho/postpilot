import i18next from 'i18next'
import type { AppFailure } from '@/shared/api'
import { activeLocale, type Locale } from './locale'

/** Formats only the stable, allowlisted failure contract. Provider/backend prose is deliberately
 * absent from this boundary and must never become the primary user-facing explanation. */
export function formatAppFailure(
  failure: AppFailure | undefined,
  locale: Locale = activeLocale(),
): string {
  if (!failure) return ''
  // A group below its effective minimum says how many items it admits and how
  // many were given; a refusal carrying no count keeps the plain wording.
  if (
    failure.reason === 'CLIP_COMPOSITION_INVALID' &&
    failure.params.reason === 'items_required' &&
    failure.params.min
  ) {
    return i18next.getFixedT(locale, 'clips')('composition.errors.items_required_count', {
      element: failure.params.element_id,
      min: failure.params.min,
      actual: failure.params.actual,
    })
  }
  if (
    failure.reason === 'CLIP_COMPOSITION_INVALID' &&
    (failure.params.reason === 'items_required' || failure.params.reason === 'invalid_item_bounds')
  ) {
    return i18next.getFixedT(locale, 'clips')(`composition.errors.${failure.params.reason}`, {
      element: failure.params.element_id,
    })
  }
  // A bounded answer says which answer, its maximum and what was stored — the
  // only refusal in this family that names a field rather than a line.
  if (
    failure.reason === 'CLIP_COMPOSITION_INVALID' &&
    failure.params.reason === 'answer_limit' &&
    failure.params.label
  ) {
    return i18next.getFixedT(locale, 'clips')('composition.errors.answer_limit_field', {
      label: failure.params.label,
      max: failure.params.max,
      actual: failure.params.actual,
    })
  }
  const translate = i18next.getFixedT(locale, 'errors') as unknown as (
    key: string,
    options: Readonly<Record<string, string>>,
  ) => string
  return translate(failure.reason, failure.params)
}
