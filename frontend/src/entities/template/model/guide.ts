import i18next from 'i18next'
import {
  TEMPLATE_ASK_MAX_PER_BODY,
  TEMPLATE_BODY_MAX_CHARS,
  TEMPLATE_PHOTO_ROW_MAX,
} from '@/shared/config'

/** The worked example the guide carries, and the one place it is written down.
 *
 *  It is a BODY, not prose: `guide.test.ts` parses it with the real parser, so an example that
 *  drifted from the grammar fails the build rather than teaching an outside AI to write something
 *  this app refuses (TEMPLATE-41). It deliberately uses every construct a person authors today,
 *  including a photo row and a data field, and none of the retired ones. */
export const GUIDE_EXAMPLE_BODY =
  '<write>어디를 왜 갔는지 두세 문장으로 시작</write>\n' +
  '<repeat each="photo">\n' +
  '<slot kind="photo" count="2"/>\n' +
  '<write>위 사진 두 장에 대해 한 문단</write>\n' +
  '</repeat>\n' +
  '<ask label="총평 별점">별점과 한 줄 총평</ask>\n' +
  '오늘도 좋은 하루 보내세요\n' +
  '<note>광고처럼 들리지 않게</note>'

/** A self-contained instruction a user can hand to any outside AI so it writes a body in this
 *  app's format (TEMPLATE-41).
 *
 *  The tags, the attributes and the example are IDENTICAL in both languages by construction: only
 *  the surrounding prose lives in the resource, and the grammar arrives through interpolation.
 *  `escapeValue: false` is required — i18next would otherwise turn the example's `<` into `&lt;`
 *  and hand the AI something no parser here accepts.
 *
 *  It is client text and reaches no provider: no template surface makes a model call (TEMPLATE-16). */
export function formatGuide(): string {
  return i18next.t('source.guide', {
    ns: 'templates',
    example: GUIDE_EXAMPLE_BODY,
    bodyMax: TEMPLATE_BODY_MAX_CHARS,
    photoRowMax: TEMPLATE_PHOTO_ROW_MAX,
    askMax: TEMPLATE_ASK_MAX_PER_BODY,
    interpolation: { escapeValue: false },
  })
}
