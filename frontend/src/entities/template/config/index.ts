/** The template context's own ceilings (ARCH-21), moved out of `shared/config`
 *  with T258. Each is mirrored from the backend's `TEMPLATE_*` env so the
 *  create/edit fields can count down before the round trip; the server stays
 *  authoritative. `shared/config` owns the env READ, this file owns the limit
 *  and its default — a malformed or non-positive override falls back rather than
 *  disabling the counter, because a build-time typo must not silently remove a
 *  client-side bound.
 *
 *  Counted in Unicode scalar values, like the backend, so a Hangul syllable is
 *  one character.
 *
 *  The per-account cap is mirrored too (TMPL-6): the template request box has to
 *  refuse a new template's request before it spends credits on a draft whose save
 *  the server would refuse (TMPL-62). The server still enforces it on every insert. */
import { ENV_LIMIT_OVERRIDES, positiveIntEnv } from '@/shared/config'
export const TEMPLATE_NAME_MAX_CHARS = positiveIntEnv(ENV_LIMIT_OVERRIDES.templateNameMaxChars, 40)
export const TEMPLATE_DESCRIPTION_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateDescriptionMaxChars,
  200,
)
export const TEMPLATE_BODY_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateBodyMaxChars,
  4000,
)
/** The title area's ceiling (TMPL-50): a title is one line of a post, so its source is bounded
 *  like a short field rather than like the body. */
export const TEMPLATE_TITLE_AREA_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateTitleAreaMaxChars,
  200,
)

/** The largest photo count a photo position may suggest as one group (TMPL-38). Unlike the ceilings above this
 *  one is not only a counter: the builder's stepper cannot offer a value the server's parser
 *  would refuse on save, so the two numbers have to be raised together. */
export const TEMPLATE_PHOTO_ROW_MAX = positiveIntEnv(ENV_LIMIT_OVERRIDES.templatePhotoRowMax, 3)

/** The data-field ceilings (TMPL-43): a field's title, one answer's text, and how many
 *  fields one body may declare. The first two are live counters and the third is a refusal the
 *  builder states before the server has to; the backend stays authoritative on all three. */
export const TEMPLATE_ASK_LABEL_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateAskLabelMaxChars,
  40,
)
export const TEMPLATE_ASK_VALUE_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateAskValueMaxChars,
  500,
)
export const TEMPLATE_ASK_MAX_PER_BODY = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateAskMaxPerBody,
  10,
)

/** How many templates one account may hold (TMPL-6). */
export const TEMPLATE_MAX_PER_ACCOUNT = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateMaxPerAccount,
  50,
)

/** What the template request box takes (TMPL-58): room for the longest post the product writes
 *  with a description beside it. A live counter; the server refuses beyond it. */
export const TEMPLATE_REQUEST_MAX_CHARS = positiveIntEnv(
  ENV_LIMIT_OVERRIDES.templateRequestMaxChars,
  12_000,
)
