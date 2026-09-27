import i18next from 'i18next'
import {
  TEMPLATE_ASK_LABEL_MAX_CHARS,
  TEMPLATE_ASK_MAX_PER_BODY,
  TEMPLATE_BODY_MAX_CHARS,
  TEMPLATE_PHOTO_ROW_MAX,
} from '../config'
/** The worked example the guide carries, and the one place it is written down.
 *
 *  It is a BODY, not prose: `guide.test.ts` parses it with the real parser, so an example that
 *  drifted from the grammar fails the build rather than teaching an outside AI to write something
 *  this app refuses (TMPL-41). It deliberately uses every construct a person authors today,
 *  including a photo row and a data field, and none of the retired ones; each `<write>` names
 *  what stands at its place, never how to write it (TMPL-57). */
export const GUIDE_EXAMPLE_BODY =
  '<write>방문한 이유와 첫인상</write>\n' +
  '<repeat each="photo">\n' +
  '<slot kind="photo" count="2"/>\n' +
  '<write>이 사진들이 보여주는 장면</write>\n' +
  '</repeat>\n' +
  '<ask label="총평 별점">총평</ask>\n' +
  '오늘도 좋은 하루 보내세요'

/** A self-contained instruction a user can hand to any outside AI so it writes a body in this
 *  app's format (TMPL-41).
 *
 *  The tags, the attributes and the example are IDENTICAL in both languages by construction: only
 *  the surrounding prose lives in the resource, and the grammar arrives through interpolation.
 *  `escapeValue: false` is required — i18next would otherwise turn the example's `<` into `&lt;`
 *  and hand the AI something no parser here accepts.
 *
 *  It is client text and reaches no provider: no template surface makes a model call (TMPL-16). */
export function formatGuide(): string {
  return i18next.t('source.guide', {
    ns: 'templates',
    example: GUIDE_EXAMPLE_BODY,
    bodyMax: TEMPLATE_BODY_MAX_CHARS,
    photoRowMax: TEMPLATE_PHOTO_ROW_MAX,
    askMax: TEMPLATE_ASK_MAX_PER_BODY,
    askLabelMax: TEMPLATE_ASK_LABEL_MAX_CHARS,
    interpolation: { escapeValue: false },
  })
}
